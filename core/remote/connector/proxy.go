package connector

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	"danmo-work/core/remote/tunnel"
)

type inboundStream struct {
	open    tunnel.HTTPOpenPayload
	body    bytes.Buffer
	bodyMu  sync.Mutex
	done    chan struct{}
	finOnce sync.Once
}

func (c *Connector) startHTTPOpen(ctx context.Context, streamID uint32, open tunnel.HTTPOpenPayload) {
	s := &inboundStream{open: open, done: make(chan struct{})}
	c.streamsMu.Lock()
	// Preserve body bytes if FIN already arrived (unlikely) — replace placeholder.
	if prev := c.inbound[streamID]; prev != nil {
		prev.bodyMu.Lock()
		s.body.Write(prev.body.Bytes())
		prev.bodyMu.Unlock()
		// If prev already finished, mark done.
		select {
		case <-prev.done:
			s.finOnce.Do(func() { close(s.done) })
		default:
		}
	}
	c.inbound[streamID] = s
	cancelCtx, cancel := context.WithCancel(ctx)
	c.streamCancels[streamID] = cancel
	c.streamsMu.Unlock()

	go c.runHTTPOpen(cancelCtx, cancel, streamID, s)
}

func (c *Connector) runHTTPOpen(cancelCtx context.Context, cancel context.CancelFunc, streamID uint32, s *inboundStream) {
	defer func() {
		cancel()
		c.streamsMu.Lock()
		delete(c.inbound, streamID)
		delete(c.streamCancels, streamID)
		c.streamsMu.Unlock()
	}()

	select {
	case <-cancelCtx.Done():
		return
	case <-s.done:
	}

	s.bodyMu.Lock()
	bodyBytes := append([]byte(nil), s.body.Bytes()...)
	s.bodyMu.Unlock()

	open := s.open
	method := open.Method
	if method == "" {
		method = http.MethodGet
	}
	path := open.Path
	if path == "" || !strings.HasPrefix(path, "/") {
		_ = c.sendError(streamID, "bad_path", "path must start with /")
		_ = c.sendStreamClose(streamID, 400, "bad_path")
		return
	}

	url := strings.TrimRight(c.cfg.LocalBase, "/") + path
	var bodyReader io.Reader
	if len(bodyBytes) > 0 {
		bodyReader = bytes.NewReader(bodyBytes)
	}
	req, err := http.NewRequestWithContext(cancelCtx, method, url, bodyReader)
	if err != nil {
		_ = c.sendError(streamID, "bad_request", err.Error())
		_ = c.sendStreamClose(streamID, 400, "bad_request")
		return
	}
	if len(bodyBytes) > 0 {
		req.ContentLength = int64(len(bodyBytes))
	}
	for _, hv := range open.Headers {
		if hv[0] == "" {
			continue
		}
		canon := http.CanonicalHeaderKey(hv[0])
		if canon == "Host" || canon == "Connection" || canon == "Transfer-Encoding" {
			continue
		}
		req.Header.Add(hv[0], hv[1])
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if cancelCtx.Err() != nil {
			return
		}
		log.Printf("[remote] proxy %s %s: %v", method, path, err)
		_ = c.sendError(streamID, "proxy_error", err.Error())
		_ = c.sendStreamClose(streamID, 502, "proxy_error")
		return
	}
	defer resp.Body.Close()

	headers := make([][2]string, 0)
	for k, vals := range resp.Header {
		for _, v := range vals {
			headers = append(headers, [2]string{k, v})
		}
	}
	payload, _ := tunnel.MarshalPayload(tunnel.HTTPRespOpenPayload{
		Status:  resp.StatusCode,
		Headers: headers,
	})
	if err := c.writer.writeFrame(tunnel.TypeHTTPRespOpen, streamID, payload); err != nil {
		return
	}

	buf := make([]byte, 32*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			chunk := tunnel.EncodeHTTPBody(buf[:n], false)
			if werr := c.writer.writeFrame(tunnel.TypeHTTPBody, streamID, chunk); werr != nil {
				return
			}
		}
		if rerr != nil {
			fin := tunnel.EncodeHTTPBody(nil, true)
			_ = c.writer.writeFrame(tunnel.TypeHTTPBody, streamID, fin)
			_ = c.sendStreamClose(streamID, 0, "ok")
			return
		}
	}
}

func (c *Connector) handleHTTPBody(streamID uint32, payload []byte) {
	data, fin, err := tunnel.DecodeHTTPBody(payload)
	if err != nil {
		return
	}
	c.streamsMu.Lock()
	s := c.inbound[streamID]
	if s == nil {
		// Body before Open handler registered — create placeholder.
		s = &inboundStream{done: make(chan struct{})}
		c.inbound[streamID] = s
	}
	c.streamsMu.Unlock()
	if len(data) > 0 {
		s.bodyMu.Lock()
		_, _ = s.body.Write(data)
		s.bodyMu.Unlock()
	}
	if fin {
		s.finOnce.Do(func() { close(s.done) })
	}
}

func (c *Connector) handleStreamClose(streamID uint32) {
	c.streamsMu.Lock()
	if cancel := c.streamCancels[streamID]; cancel != nil {
		cancel()
	}
	if s := c.inbound[streamID]; s != nil {
		s.finOnce.Do(func() { close(s.done) })
	}
	c.streamsMu.Unlock()
}

func (c *Connector) sendStreamClose(streamID uint32, code int, reason string) error {
	payload, _ := tunnel.MarshalPayload(tunnel.StreamClosePayload{Code: code, Reason: reason})
	return c.writer.writeFrame(tunnel.TypeStreamClose, streamID, payload)
}

func (c *Connector) sendError(streamID uint32, code, msg string) error {
	payload, _ := tunnel.MarshalPayload(tunnel.ErrorPayload{
		Code: code, Message: msg, StreamID: streamID,
	})
	return c.writer.writeFrame(tunnel.TypeError, streamID, payload)
}
