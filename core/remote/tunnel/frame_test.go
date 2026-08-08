package tunnel

import (
	"bytes"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	payload, _ := MarshalPayload(RegisterPayload{
		DeviceID: "dev1", DeviceSecret: "sec", AppVersion: "0.1",
	})
	var buf bytes.Buffer
	if err := EncodeFrame(&buf, TypeRegister, 0, payload); err != nil {
		t.Fatal(err)
	}
	f, err := DecodeFrame(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if f.Type != TypeRegister || f.StreamID != 0 {
		t.Fatalf("header mismatch: %+v", f)
	}
	var reg RegisterPayload
	if err := UnmarshalPayload(f.Payload, &reg); err != nil {
		t.Fatal(err)
	}
	if reg.DeviceID != "dev1" || reg.DeviceSecret != "sec" {
		t.Fatalf("payload: %+v", reg)
	}
}

func TestHTTPBodyFlags(t *testing.T) {
	p := EncodeHTTPBody([]byte("hi"), true)
	data, fin, err := DecodeHTTPBody(p)
	if err != nil || !fin || string(data) != "hi" {
		t.Fatalf("got data=%q fin=%v err=%v", data, fin, err)
	}
}
