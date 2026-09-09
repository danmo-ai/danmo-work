package service

import (
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func TestDecodeProjectTextBytesUTF8(t *testing.T) {
	got, err := decodeProjectTextBytes([]byte("你好\n"))
	if err != nil || got != "你好\n" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestDecodeProjectTextBytesGB18030(t *testing.T) {
	raw, _, err := transform.Bytes(simplifiedchinese.GB18030.NewEncoder(), []byte("第75章\n"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeProjectTextBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got != "第75章\n" {
		t.Fatalf("got %q", got)
	}
}
