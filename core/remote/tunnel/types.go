// Package tunnel implements the danmo-hub tunnel/v1 wire protocol.
package tunnel

import "encoding/json"

// Message type codes (tunnel/v1).
const (
	TypeRegister     uint8 = 1
	TypeRegisterOK   uint8 = 2
	TypeHeartbeat    uint8 = 3
	TypeHTTPOpen     uint8 = 10
	TypeHTTPBody     uint8 = 11
	TypeHTTPRespOpen uint8 = 12
	TypeStreamClose  uint8 = 13
	TypeError        uint8 = 15
)

// HTTPBodyFlags bit0 = end of body.
const FlagBodyFIN uint8 = 0x01

const (
	HeartbeatIntervalSec = 15
	HeartbeatTimeoutSec  = 90
	MinBackoffSec        = 1
	MaxBackoffSec        = 30
)

// Frame is one decoded tunnel frame.
type Frame struct {
	Type     uint8
	StreamID uint32
	Payload  []byte
}

type RegisterPayload struct {
	DeviceID     string `json:"device_id"`
	DeviceSecret string `json:"device_secret"`
	AppVersion   string `json:"app_version"`
}

type RegisterOKPayload struct {
	ServerTimeUnix int64 `json:"server_time_unix"`
}

type HeartbeatPayload struct {
	TsUnixMs int64 `json:"ts_unix_ms"`
}

type HTTPOpenPayload struct {
	Method  string     `json:"method"`
	Path    string     `json:"path"`
	Headers [][2]string `json:"headers"`
}

type HTTPRespOpenPayload struct {
	Status  int        `json:"status"`
	Headers [][2]string `json:"headers"`
}

type StreamClosePayload struct {
	Code   int    `json:"code"`
	Reason string `json:"reason"`
}

type ErrorPayload struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	StreamID uint32 `json:"stream_id,omitempty"`
}

func MarshalPayload(v any) ([]byte, error) {
	return json.Marshal(v)
}

func UnmarshalPayload(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
