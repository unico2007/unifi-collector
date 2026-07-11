package query

import "testing"

func TestDecodeLogLineNonJSONFallback(t *testing.T) {
	msg, level := DecodeLogLine("plain text line", "error")
	if msg != "plain text line" || level != "error" {
		t.Fatalf("fallback failed: msg=%q level=%q", msg, level)
	}
}
