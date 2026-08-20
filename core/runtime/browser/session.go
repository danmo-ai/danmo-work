package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"danmo-work/core/port"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

const (
	defaultActTimeout   = 30 * time.Second
	defaultScrollAmount = 600
	refAttr             = "data-danmo-ref"
)

// pageSession is one sticky tab for a Danmo session.
type pageSession struct {
	mu       sync.Mutex
	id       string
	tabCtx   context.Context
	cancel   context.CancelFunc
	lastUsed time.Time
	mgr      *Manager
}

func (p *pageSession) touch() {
	p.lastUsed = time.Now()
}

func (p *pageSession) Navigate(ctx context.Context, opts port.BrowserNavigateOptions) (port.BrowserNavigateResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.touch()
	_ = ctx

	if opts.URL == "" {
		return port.BrowserNavigateResult{}, fmt.Errorf("url is required")
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultActTimeout
	}
	runCtx, cancel := context.WithTimeout(p.tabCtx, timeout)
	defer cancel()

	if err := chromedp.Run(runCtx,
		chromedp.Navigate(opts.URL),
		waitAction(opts.WaitUntil),
	); err != nil {
		return port.BrowserNavigateResult{}, fmt.Errorf("browser navigate failed: %w", err)
	}
	snap, err := p.snapshotLocked(runCtx)
	if err != nil {
		return port.BrowserNavigateResult{}, err
	}
	return port.BrowserNavigateResult{
		URL:      snap.URL,
		Title:    snap.Title,
		Snapshot: snap.Snapshot,
	}, nil
}

func (p *pageSession) Snapshot(ctx context.Context) (port.BrowserSnapshotResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.touch()
	_ = ctx
	runCtx, cancel := context.WithTimeout(p.tabCtx, defaultActTimeout)
	defer cancel()
	return p.snapshotLocked(runCtx)
}

func (p *pageSession) Act(ctx context.Context, req port.BrowserActRequest) (port.BrowserActResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.touch()
	_ = ctx

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = defaultActTimeout
	}
	runCtx, cancel := context.WithTimeout(p.tabCtx, timeout)
	defer cancel()

	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		return port.BrowserActResult{}, fmt.Errorf("action is required")
	}

	var err error
	switch action {
	case "click":
		err = p.clickRef(runCtx, req.Ref)
	case "type":
		err = p.typeRef(runCtx, req.Ref, req.Text)
	case "press":
		err = p.pressKey(runCtx, req.Key)
	case "scroll":
		err = p.scroll(runCtx, req.Direction, req.Amount)
	case "select":
		err = p.selectRef(runCtx, req.Ref, req.Value)
	case "hover":
		err = p.hoverRef(runCtx, req.Ref)
	default:
		return port.BrowserActResult{}, fmt.Errorf("unsupported action %q (click|type|press|scroll|select|hover)", req.Action)
	}
	if err != nil {
		return port.BrowserActResult{}, err
	}

	select {
	case <-runCtx.Done():
		return port.BrowserActResult{}, runCtx.Err()
	case <-time.After(200 * time.Millisecond):
	}

	snap, err := p.snapshotLocked(runCtx)
	if err != nil {
		return port.BrowserActResult{}, err
	}
	return port.BrowserActResult{
		URL:      snap.URL,
		Title:    snap.Title,
		Snapshot: snap.Snapshot,
	}, nil
}

func (p *pageSession) Screenshot(ctx context.Context, fullPage bool) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.touch()
	_ = ctx
	runCtx, cancel := context.WithTimeout(p.tabCtx, defaultActTimeout)
	defer cancel()

	var buf []byte
	var tasks chromedp.Tasks
	if fullPage {
		tasks = chromedp.Tasks{chromedp.FullScreenshot(&buf, 90)}
	} else {
		tasks = chromedp.Tasks{chromedp.CaptureScreenshot(&buf)}
	}
	if err := chromedp.Run(runCtx, tasks); err != nil {
		return nil, fmt.Errorf("browser screenshot failed: %w", err)
	}
	if len(buf) == 0 {
		return nil, fmt.Errorf("browser screenshot returned empty image")
	}
	return buf, nil
}

func (p *pageSession) Close(ctx context.Context) error {
	_ = ctx
	p.mu.Lock()
	cancel := p.cancel
	p.cancel = nil
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if p.mgr != nil {
		p.mgr.removePage(p.id)
	}
	return nil
}

func (p *pageSession) snapshotLocked(ctx context.Context) (port.BrowserSnapshotResult, error) {
	var raw string
	if err := chromedp.Run(ctx, chromedp.Evaluate(snapshotJS, &raw)); err != nil {
		return port.BrowserSnapshotResult{}, fmt.Errorf("browser snapshot failed: %w", err)
	}
	var parsed snapPayload
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return port.BrowserSnapshotResult{}, fmt.Errorf("browser snapshot parse failed: %w", err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Page: %s\nURL: %s\n", emptyFallback(parsed.Title, "(no title)"), emptyFallback(parsed.URL, ""))
	if len(parsed.Nodes) == 0 {
		b.WriteString("(no interactive elements found)\n")
	} else {
		for _, n := range parsed.Nodes {
			fmt.Fprintf(&b, "- %s %s", n.Ref, n.Role)
			if n.Name != "" {
				fmt.Fprintf(&b, " %q", n.Name)
			}
			if n.Value != "" && n.Value != n.Name {
				fmt.Fprintf(&b, " value=%q", n.Value)
			}
			b.WriteByte('\n')
		}
	}
	return port.BrowserSnapshotResult{
		URL:      parsed.URL,
		Title:    parsed.Title,
		Snapshot: strings.TrimRight(b.String(), "\n"),
	}, nil
}

type snapPayload struct {
	URL   string     `json:"url"`
	Title string     `json:"title"`
	Nodes []snapNode `json:"nodes"`
}

type snapNode struct {
	Ref   string `json:"ref"`
	Role  string `json:"role"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

func emptyFallback(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func (p *pageSession) requireRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("ref is required (from the latest browser_snapshot / navigate result)")
	}
	return ref, nil
}

func refSelector(ref string) string {
	return fmt.Sprintf(`[%s=%q]`, refAttr, ref)
}

func (p *pageSession) clickRef(ctx context.Context, ref string) error {
	ref, err := p.requireRef(ref)
	if err != nil {
		return err
	}
	sel := refSelector(ref)
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(sel, chromedp.ByQuery),
		chromedp.Click(sel, chromedp.ByQuery),
	); err != nil {
		return fmt.Errorf("click %s failed: %w", ref, err)
	}
	return nil
}

func (p *pageSession) typeRef(ctx context.Context, ref, text string) error {
	ref, err := p.requireRef(ref)
	if err != nil {
		return err
	}
	sel := refSelector(ref)
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(sel, chromedp.ByQuery),
		chromedp.Clear(sel, chromedp.ByQuery),
		chromedp.SendKeys(sel, text, chromedp.ByQuery),
	); err != nil {
		return fmt.Errorf("type into %s failed: %w", ref, err)
	}
	return nil
}

func (p *pageSession) hoverRef(ctx context.Context, ref string) error {
	ref, err := p.requireRef(ref)
	if err != nil {
		return err
	}
	sel := refSelector(ref)
	js := fmt.Sprintf(`(() => {
		const el = document.querySelector(%q);
		if (!el) throw new Error('ref not found');
		el.dispatchEvent(new MouseEvent('mouseover', {bubbles:true}));
		el.dispatchEvent(new MouseEvent('mouseenter', {bubbles:true}));
		return true;
	})()`, sel)
	var ok bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &ok)); err != nil {
		return fmt.Errorf("hover %s failed: %w", ref, err)
	}
	return nil
}

func (p *pageSession) selectRef(ctx context.Context, ref, value string) error {
	ref, err := p.requireRef(ref)
	if err != nil {
		return err
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("value is required for select")
	}
	sel := refSelector(ref)
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(sel, chromedp.ByQuery),
		chromedp.SetValue(sel, value, chromedp.ByQuery),
	); err != nil {
		return fmt.Errorf("select %s failed: %w", ref, err)
	}
	return nil
}

func (p *pageSession) pressKey(ctx context.Context, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("key is required for press")
	}
	if err := chromedp.Run(ctx, keyToChromedp(key)); err != nil {
		return fmt.Errorf("press %q failed: %w", key, err)
	}
	return nil
}

func (p *pageSession) scroll(ctx context.Context, direction string, amount int) error {
	if amount <= 0 {
		amount = defaultScrollAmount
	}
	dir := strings.ToLower(strings.TrimSpace(direction))
	if dir == "" {
		dir = "down"
	}
	delta := amount
	switch dir {
	case "down":
	case "up":
		delta = -amount
	default:
		return fmt.Errorf("direction must be up or down")
	}
	js := fmt.Sprintf(`window.scrollBy(0, %d)`, delta)
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, nil)); err != nil {
		return fmt.Errorf("scroll failed: %w", err)
	}
	return nil
}

func keyToChromedp(key string) chromedp.Action {
	switch strings.ToLower(key) {
	case "enter", "return":
		return chromedp.KeyEvent(kb.Enter)
	case "tab":
		return chromedp.KeyEvent(kb.Tab)
	case "escape", "esc":
		return chromedp.KeyEvent(kb.Escape)
	case "backspace":
		return chromedp.KeyEvent(kb.Backspace)
	case "space":
		return chromedp.KeyEvent(" ")
	case "arrowup", "up":
		return chromedp.KeyEvent(kb.ArrowUp)
	case "arrowdown", "down":
		return chromedp.KeyEvent(kb.ArrowDown)
	case "arrowleft", "left":
		return chromedp.KeyEvent(kb.ArrowLeft)
	case "arrowright", "right":
		return chromedp.KeyEvent(kb.ArrowRight)
	default:
		return chromedp.KeyEvent(key)
	}
}

// snapshotJS stamps data-danmo-ref on interactive nodes and returns a JSON summary.
const snapshotJS = `(() => {
  const ATTR = 'data-danmo-ref';
  document.querySelectorAll('[' + ATTR + ']').forEach(el => el.removeAttribute(ATTR));
  const sel = [
    'a[href]', 'button', 'input', 'textarea', 'select',
    '[role="button"]', '[role="link"]', '[role="textbox"]', '[role="checkbox"]',
    '[role="radio"]', '[role="menuitem"]', '[role="tab"]', '[role="switch"]',
    '[contenteditable="true"]', '[onclick]'
  ].join(',');
  const nodes = [];
  let i = 0;
  const seen = new Set();
  for (const el of document.querySelectorAll(sel)) {
    if (!el || seen.has(el)) continue;
    const style = window.getComputedStyle(el);
    if (style && (style.visibility === 'hidden' || style.display === 'none')) continue;
    const rect = el.getBoundingClientRect();
    if (rect.width === 0 && rect.height === 0) continue;
    seen.add(el);
    i += 1;
    const ref = 'e' + i;
    el.setAttribute(ATTR, ref);
    let role = (el.getAttribute('role') || el.tagName || 'element').toLowerCase();
    if (el.tagName === 'A') role = 'link';
    if (el.tagName === 'BUTTON') role = 'button';
    if (el.tagName === 'SELECT') role = 'combobox';
    if (el.tagName === 'TEXTAREA') role = 'textbox';
    if (el.tagName === 'INPUT') {
      const t = (el.type || 'text').toLowerCase();
      if (t === 'submit' || t === 'button') role = 'button';
      else if (t === 'checkbox') role = 'checkbox';
      else if (t === 'radio') role = 'radio';
      else role = 'textbox';
    }
    let name = (el.getAttribute('aria-label')
      || el.getAttribute('placeholder')
      || el.getAttribute('title')
      || el.getAttribute('name')
      || (el.innerText || el.textContent || '')).trim().replace(/\s+/g, ' ');
    if (name.length > 80) name = name.slice(0, 77) + '...';
    let value = '';
    if ('value' in el && el.value != null && String(el.value).length > 0) {
      value = String(el.value).slice(0, 80);
    }
    nodes.push({ ref, role, name, value });
    if (nodes.length >= 80) break;
  }
  return JSON.stringify({
    url: location.href,
    title: document.title || '',
    nodes
  });
})()`
