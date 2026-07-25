package service

import (
	"testing"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

func TestApplyQQGroupPolicyDenyTools(t *testing.T) {
	msg := &port.InboundMessage{
		Meta: map[string]string{"scene": "group", "group_openid": "g1", "receive_type": "group"},
	}
	cfg := domain.ConfigQQChannel{
		Group: domain.ConfigQQGroupPolicy{
			DenyTools: []string{"exec_shell", "write"},
		},
	}
	if !applyQQGroupPolicy(cfg, msg) {
		t.Fatal("expected accept")
	}
	if msg.Meta["deny_tools"] != "exec_shell,write" {
		t.Fatalf("deny_tools=%q", msg.Meta["deny_tools"])
	}
	if msg.Meta["policy_note"] == "" {
		t.Fatal("expected policy note")
	}
}

func TestApplyQQGroupPolicyC2CUnaffected(t *testing.T) {
	msg := &port.InboundMessage{
		Meta: map[string]string{"scene": "c2c", "receive_type": "c2c"},
	}
	cfg := domain.ConfigQQChannel{
		Group: domain.ConfigQQGroupPolicy{DenyTools: []string{"exec_shell"}},
	}
	applyQQGroupPolicy(cfg, msg)
	if msg.Meta["deny_tools"] != "" {
		t.Fatalf("c2c should not get deny_tools: %q", msg.Meta["deny_tools"])
	}
}

func TestChannelToolDenied(t *testing.T) {
	msg := &port.InboundMessage{Meta: map[string]string{"deny_tools": "exec_shell,write"}}
	ok, _ := channelToolDenied(msg, "exec_shell")
	if !ok {
		t.Fatal("expected deny")
	}
	ok, _ = channelToolDenied(msg, "read_file")
	if ok {
		t.Fatal("read_file should pass")
	}
}
