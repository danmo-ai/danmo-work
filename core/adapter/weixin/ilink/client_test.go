package ilink_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"danmo-work/core/adapter/weixin/ilink"
)

func TestBuildClientVersion(t *testing.T) {
	if got := ilink.BuildClientVersion("2.4.3"); got != ((2 << 16) | (4 << 8) | 3) {
		t.Fatalf("version encode: got %d", got)
	}
}

func TestNormalizeAccountID(t *testing.T) {
	got := ilink.NormalizeAccountID("abc@im.bot")
	if got != "abc_im.bot" {
		t.Fatalf("got %q", got)
	}
}

func TestTextFromMessage(t *testing.T) {
	msg := ilink.Message{ItemList: []ilink.MessageItem{{
		Type:     ilink.MessageItemText,
		TextItem: &ilink.TextItem{Text: "  hello  "},
	}}}
	if got := ilink.TextFromMessage(msg); got != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestClientQRUpdatesSend(t *testing.T) {
	var sawQR, sawSend, sawUpdates bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.Contains(path, "get_bot_qrcode"):
			sawQR = true
			if r.Header.Get("iLink-App-Id") != "bot" {
				t.Errorf("missing app id header")
			}
			_ = json.NewEncoder(w).Encode(map[string]string{
				"qrcode":             "qr-token",
				"qrcode_img_content": "data:image/png;base64,xxx",
			})
		case strings.Contains(path, "getupdates"):
			sawUpdates = true
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ret":             0,
				"get_updates_buf": "buf-1",
				"msgs": []map[string]any{{
					"message_type": 1,
					"from_user_id": "peer1",
					"item_list": []map[string]any{{
						"type":      1,
						"text_item": map[string]string{"text": "ping"},
					}},
				}},
			})
		case strings.Contains(path, "sendmessage"):
			sawSend = true
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "hi there") {
				t.Errorf("body missing text: %s", body)
			}
			if r.Header.Get("AuthorizationType") != "ilink_bot_token" {
				t.Errorf("missing AuthorizationType")
			}
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"ret":0}`))
		default:
			t.Errorf("unexpected path %s", path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	c := ilink.NewClient()
	c.HTTP = srv.Client()
	c.BaseURL = srv.URL

	qr, err := c.GetBotQRCode(context.Background(), ilink.DefaultBotType, nil)
	if err != nil {
		t.Fatal(err)
	}
	if qr.QRCode != "qr-token" || qr.QRCodeImgContent == "" {
		t.Fatalf("qr=%+v", qr)
	}
	if !sawQR {
		t.Fatal("qrcode endpoint not hit")
	}

	acc := ilink.Account{AccountID: "a1", Token: "tok", BaseURL: srv.URL}
	resp, err := c.GetUpdates(context.Background(), acc, "", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !sawUpdates || resp.GetUpdatesBuf != "buf-1" || len(resp.Msgs) != 1 {
		t.Fatalf("unexpected updates: %+v", resp)
	}
	if ilink.TextFromMessage(resp.Msgs[0]) != "ping" {
		t.Fatalf("text=%q", ilink.TextFromMessage(resp.Msgs[0]))
	}
	if err := c.SendText(context.Background(), acc, "peer1", "hi there", "ctx", "cid-1"); err != nil {
		t.Fatal(err)
	}
	if !sawSend {
		t.Fatal("sendmessage not hit")
	}
}
