package browser

import (
	"context"
	"encoding/base64"
	"sync"
	"time"

	"danmo-work/core/port"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

const (
	defaultWatchWidth  = 1280
	defaultWatchHeight = 800
	defaultWatchJPEG   = 72
	watchBufferSize    = 8
)

// pageStream owns the CDP listeners for one watched sticky page. The callback
// only copies and enqueues data; all CDP commands run in the subscription loop.
type pageStream struct {
	page   *pageSession
	opts   port.BrowserWatchOptions
	newRun func(context.Context)

	mu       sync.Mutex
	watchers map[uint64]chan port.BrowserWatchEvent
	nextID   uint64
	closed   bool
	last     *port.BrowserWatchEvent
	cancel   context.CancelFunc
	done     chan struct{}
}

func normalizeWatchOptions(opts port.BrowserWatchOptions) port.BrowserWatchOptions {
	if opts.MaxWidth <= 0 || opts.MaxWidth > defaultWatchWidth {
		opts.MaxWidth = defaultWatchWidth
	}
	if opts.MaxHeight <= 0 || opts.MaxHeight > defaultWatchHeight {
		opts.MaxHeight = defaultWatchHeight
	}
	if opts.Quality <= 0 || opts.Quality > 100 {
		opts.Quality = defaultWatchJPEG
	}
	return opts
}

func (p *pageSession) newStream(opts port.BrowserWatchOptions) *pageStream {
	return &pageStream{
		page:     p,
		opts:     normalizeWatchOptions(opts),
		watchers: map[uint64]chan port.BrowserWatchEvent{},
		done:     make(chan struct{}),
	}
}

func (s *pageStream) subscribe(ctx context.Context) (<-chan port.BrowserWatchEvent, error) {
	streamCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	if s.closed || s.page.isClosed() {
		s.mu.Unlock()
		cancel()
		return nil, port.ErrBrowserPageClosed
	}
	s.nextID++
	id := s.nextID
	out := make(chan port.BrowserWatchEvent, watchBufferSize)
	s.watchers[id] = out
	last := s.last
	status := s.page.status("live")
	start := len(s.watchers) == 1
	if start {
		s.cancel = cancel
	}
	s.mu.Unlock()

	sendWatchEvent(ctx, out, port.BrowserWatchEvent{Status: status})
	if last != nil {
		sendWatchEvent(ctx, out, *last)
	}
	if start {
		run := s.run
		if s.newRun != nil {
			run = s.newRun
		}
		go run(streamCtx)
	} else {
		cancel()
	}
	go func() {
		select {
		case <-ctx.Done():
			s.unsubscribe(id)
		case <-s.done:
		}
	}()
	return out, nil
}

func (s *pageStream) unsubscribe(id uint64) {
	s.mu.Lock()
	if _, ok := s.watchers[id]; !ok {
		s.mu.Unlock()
		return
	}
	ch := s.watchers[id]
	delete(s.watchers, id)
	close(ch)
	cancel := s.cancel
	empty := len(s.watchers) == 0
	if empty {
		s.cancel = nil
	}
	s.mu.Unlock()
	if cancel != nil && empty {
		cancel()
	}
}

func (s *pageStream) lastFrame() port.BrowserLastFrame {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last == nil || s.last.Frame == nil {
		return port.BrowserLastFrame{}
	}
	return port.BrowserLastFrame{Frame: *s.last.Frame}
}

func (s *pageStream) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	cancel := s.cancel
	s.cancel = nil
	last := s.last
	s.mu.Unlock()
	if last != nil && last.Frame != nil && s.page.mgr != nil {
		s.page.mgr.mu.Lock()
		s.page.mgr.lastFrames[s.page.id] = port.BrowserLastFrame{
			Status: s.page.status("closed"),
			Frame:  *last.Frame,
		}
		s.page.mgr.mu.Unlock()
	}
	s.mu.Lock()
	for id, ch := range s.watchers {
		delete(s.watchers, id)
		close(ch)
	}
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *pageStream) publish(ev port.BrowserWatchEvent) {
	if ev.Frame != nil {
		copyEv := ev
		s.mu.Lock()
		s.last = &copyEv
		for _, out := range s.watchers {
			sendWatchEvent(context.Background(), out, ev)
		}
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	for _, out := range s.watchers {
		sendWatchEvent(context.Background(), out, ev)
	}
	s.mu.Unlock()
}

func sendWatchEvent(ctx context.Context, out chan port.BrowserWatchEvent, ev port.BrowserWatchEvent) {
	select {
	case out <- ev:
		return
	default:
	}
	if ev.Frame != nil {
		// A slow observer gets the newest visual state, never a backlog of JPEGs.
		select {
		case <-out:
		default:
		}
	}
	select {
	case out <- ev:
	case <-ctx.Done():
	default:
	}
}

func (s *pageStream) run(ctx context.Context) {
	defer close(s.done)
	defer func() {
		s.mu.Lock()
		s.cancel = nil
		s.mu.Unlock()
	}()
	tabCtx := s.page.tabContext()
	if tabCtx == nil || chromedp.FromContext(tabCtx) == nil {
		s.close()
		return
	}
	events := make(chan *page.EventScreencastFrame, watchBufferSize)
	chromedp.ListenTarget(tabCtx, func(ev any) {
		frame, ok := ev.(*page.EventScreencastFrame)
		if !ok {
			return
		}
		data, err := base64.StdEncoding.DecodeString(frame.Data)
		if err != nil {
			return
		}
		watchEv := port.BrowserWatchEvent{Frame: &port.BrowserFrame{Data: data, MimeType: "image/jpeg"}}
		if frame.Metadata != nil {
			watchEv.Frame.Width = int(frame.Metadata.DeviceWidth)
			watchEv.Frame.Height = int(frame.Metadata.DeviceHeight)
		}
		s.publish(watchEv)
		select {
		case events <- frame:
		default:
		}
	})
	if err := chromedp.Run(tabCtx, page.StartScreencast().
		WithFormat(page.ScreencastFormatJpeg).
		WithQuality(int64(s.opts.Quality)).
		WithMaxWidth(int64(s.opts.MaxWidth)).
		WithMaxHeight(int64(s.opts.MaxHeight)).
		WithEveryNthFrame(1)); err != nil {
		s.close()
		return
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(tabCtx, 2*time.Second)
		defer cancel()
		_ = chromedp.Run(stopCtx, chromedp.ActionFunc(func(ctx context.Context) error {
			return page.StopScreencast().Do(ctx)
		}))
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tabCtx.Done():
			s.close()
			return
		case frame := <-events:
			ackCtx, cancel := context.WithTimeout(tabCtx, 2*time.Second)
			_ = chromedp.Run(ackCtx, chromedp.ActionFunc(func(ctx context.Context) error {
				return page.ScreencastFrameAck(frame.SessionID).Do(ctx)
			}))
			cancel()
		}
	}
}

func (p *pageSession) stream(opts port.BrowserWatchOptions) *pageStream {
	p.streamMu.Lock()
	defer p.streamMu.Unlock()
	if p.live == nil {
		p.live = p.newStream(opts)
	}
	return p.live
}
