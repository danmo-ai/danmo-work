package service

import (
	"strings"
	"testing"
	"unicode/utf8"

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

func TestDecodeProjectTextBytesTruncatedUTF8(t *testing.T) {
	full := []byte("title: \"女警她脑子里有个刑侦泰斗\"\nstage: writing\n")
	// Cut mid-rune inside the Chinese title (first byte of 警 is kept, rest dropped).
	cut := full[:len([]byte("title: \"女"))+1]
	if utf8.Valid(cut) {
		t.Fatal("fixture must be incomplete UTF-8")
	}
	got, err := decodeProjectTextBytes(cut)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "title: \"女") {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(got, "濂") {
		t.Fatalf("GB18030 mojibake leaked: %q", got)
	}
}
