package netproxy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	defaultDialTimeout = 15 * time.Second
	defaultIdleTimeout = 60 * time.Second
)

// DialFunc dials a network address (host:port) for an upstream connection.
type DialFunc func(network, address string, timeout time.Duration) (net.Conn, error)

// Server is a loopback HTTP proxy that only dials allowlisted hosts.
type Server struct {
	domains []string
	ln      net.Listener
	addr    string
	dial    DialFunc

	mu     sync.Mutex
	closed bool
	conns  map[net.Conn]struct{}
}

// Start listens on 127.0.0.1:0 and serves HTTP proxy / CONNECT.
// domains must be non-empty after NormalizeDomains.
func Start(domains []string) (*Server, error) {
	return StartWithDial(domains, nil)
}

// StartWithDial is like Start but uses dial when non-nil (tests).
func StartWithDial(domains []string, dial DialFunc) (*Server, error) {
	domains = NormalizeDomains(domains)
	if len(domains) == 0 {
		return nil, fmt.Errorf("netproxy: empty allowlist")
	}
	if dial == nil {
		dial = net.DialTimeout
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("netproxy: listen: %w", err)
	}
	s := &Server{
		domains: domains,
		ln:      ln,
		addr:    ln.Addr().String(),
		dial:    dial,
		conns:   make(map[net.Conn]struct{}),
	}
	go s.serve()
	return s, nil
}

// Addr returns the listen address (host:port).
func (s *Server) Addr() string {
	return s.addr
}

// Domains returns the normalized allowlist rules.
func (s *Server) Domains() []string {
	out := make([]string, len(s.domains))
	copy(out, s.domains)
	return out
}

// Close stops accepting and closes active connections.
func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	ln := s.ln
	conns := make([]net.Conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.Unlock()
	if ln != nil {
		_ = ln.Close()
	}
	for _, c := range conns {
		_ = c.Close()
	}
	return nil
}

func (s *Server) serve() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			continue
		}
		s.track(c, true)
		go s.handle(c)
	}
}

func (s *Server) track(c net.Conn, add bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if add {
		if s.closed {
			_ = c.Close()
			return
		}
		s.conns[c] = struct{}{}
		return
	}
	delete(s.conns, c)
}

func (s *Server) handle(c net.Conn) {
	defer func() {
		s.track(c, false)
		_ = c.Close()
	}()
	_ = c.SetDeadline(time.Now().Add(defaultIdleTimeout))
	br := bufio.NewReader(c)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}

	if req.Method == http.MethodConnect {
		s.handleConnect(c, req)
		return
	}
	s.handleHTTP(c, br, req)
}

func (s *Server) handleConnect(client net.Conn, req *http.Request) {
	host, _, err := SplitHostPort(req.Host)
	if err != nil || host == "" {
		writeProxyError(client, http.StatusBadRequest, "bad host")
		return
	}
	if !Match(host, s.domains) {
		writeProxyError(client, http.StatusForbidden, "host not allowlisted")
		return
	}
	target := req.Host
	if !strings.Contains(target, ":") {
		target = net.JoinHostPort(host, "443")
	}
	up, err := s.dial("tcp", target, defaultDialTimeout)
	if err != nil {
		writeProxyError(client, http.StatusBadGateway, "dial failed")
		return
	}
	defer up.Close()
	_, _ = client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	_ = client.SetDeadline(time.Time{})
	_ = up.SetDeadline(time.Time{})
	bidirectionalCopy(client, up)
}

func (s *Server) handleHTTP(client net.Conn, _ *bufio.Reader, req *http.Request) {
	// Absolute-form URI for proxy requests.
	host := req.URL.Hostname()
	if host == "" {
		host, _, _ = SplitHostPort(req.Host)
	}
	if host == "" || !Match(host, s.domains) {
		writeProxyError(client, http.StatusForbidden, "host not allowlisted")
		return
	}
	port := req.URL.Port()
	if port == "" {
		if req.URL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	up, err := s.dial("tcp", net.JoinHostPort(host, port), defaultDialTimeout)
	if err != nil {
		writeProxyError(client, http.StatusBadGateway, "dial failed")
		return
	}
	defer up.Close()

	// Rewrite to origin-form for the upstream.
	outReq := req.Clone(context.Background())
	outReq.RequestURI = ""
	outReq.URL.Scheme = ""
	outReq.URL.Host = ""
	outReq.Header.Del("Proxy-Connection")
	outReq.Header.Del("Proxy-Authenticate")
	outReq.Header.Del("Proxy-Authorization")
	if err := outReq.Write(up); err != nil {
		return
	}
	_ = client.SetDeadline(time.Time{})
	_ = up.SetDeadline(time.Time{})
	_, _ = io.Copy(client, up)
}

func writeProxyError(w net.Conn, code int, msg string) {
	body := msg + "\n"
	resp := fmt.Sprintf("HTTP/1.1 %d %s\r\nContent-Type: text/plain\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
		code, http.StatusText(code), len(body), body)
	_, _ = w.Write([]byte(resp))
}

func bidirectionalCopy(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(a, b)
		_ = closeWrite(a)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(b, a)
		_ = closeWrite(b)
	}()
	wg.Wait()
}

type closeWriter interface {
	CloseWrite() error
}

func closeWrite(c net.Conn) error {
	if cw, ok := c.(closeWriter); ok {
		return cw.CloseWrite()
	}
	return nil
}
