package connector

import (
	"sync"

	"github.com/gorilla/websocket"

	"danmo-work/core/remote/tunnel"
)

// connWriter serializes WebSocket binary writes.
type connWriter struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

func (w *connWriter) writeFrame(typ uint8, streamID uint32, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.conn == nil {
		return errDisconnected
	}
	buf := frameBufferpool.Get().(*[]byte)
	*buf = (*buf)[:0]
	// Encode into a growable slice via bytes.Buffer pattern.
	tmp := make([]byte, 0, 9+len(payload))
	enc := &sliceWriter{b: tmp}
	if err := tunnel.EncodeFrame(enc, typ, streamID, payload); err != nil {
		return err
	}
	err := w.conn.WriteMessage(websocket.BinaryMessage, enc.b)
	frameBufferpool.Put(buf)
	return err
}

var frameBufferpool = sync.Pool{New: func() any {
	b := make([]byte, 0, 256)
	return &b
}}

type sliceWriter struct{ b []byte }

func (w *sliceWriter) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)
	return len(p), nil
}
