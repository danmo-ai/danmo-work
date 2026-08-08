package tunnel

import (
	"encoding/binary"
	"fmt"
	"io"
)

const headerSize = 9 // len(4) + type(1) + stream_id(4)

// EncodeFrame writes one tunnel/v1 frame to w.
func EncodeFrame(w io.Writer, typ uint8, streamID uint32, payload []byte) error {
	if len(payload) > 16<<20 {
		return fmt.Errorf("tunnel: payload too large: %d", len(payload))
	}
	var hdr [headerSize]byte
	binary.BigEndian.PutUint32(hdr[0:4], uint32(len(payload)))
	hdr[4] = typ
	binary.BigEndian.PutUint32(hdr[5:9], streamID)
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil
	}
	_, err := w.Write(payload)
	return err
}

// DecodeFrame reads one tunnel/v1 frame from r.
func DecodeFrame(r io.Reader) (Frame, error) {
	var hdr [headerSize]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return Frame{}, err
	}
	n := binary.BigEndian.Uint32(hdr[0:4])
	if n > 16<<20 {
		return Frame{}, fmt.Errorf("tunnel: payload too large: %d", n)
	}
	f := Frame{
		Type:     hdr[4],
		StreamID: binary.BigEndian.Uint32(hdr[5:9]),
	}
	if n == 0 {
		return f, nil
	}
	f.Payload = make([]byte, n)
	if _, err := io.ReadFull(r, f.Payload); err != nil {
		return Frame{}, err
	}
	return f, nil
}

// EncodeHTTPBody builds an HTTPBody payload: flags + data.
func EncodeHTTPBody(data []byte, fin bool) []byte {
	out := make([]byte, 1+len(data))
	if fin {
		out[0] = FlagBodyFIN
	}
	copy(out[1:], data)
	return out
}

// DecodeHTTPBody splits flags and data from an HTTPBody payload.
func DecodeHTTPBody(payload []byte) (data []byte, fin bool, err error) {
	if len(payload) < 1 {
		return nil, false, fmt.Errorf("tunnel: empty HTTPBody")
	}
	fin = payload[0]&FlagBodyFIN != 0
	return payload[1:], fin, nil
}
