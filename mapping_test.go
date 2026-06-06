package main

import (
	"testing"
)

func TestExtractString_flat(t *testing.T) {
	data := map[string]any{"key": "value"}
	got, ok := ExtractString(data, "key")
	if !ok || got != "value" {
		t.Errorf("got (%q, %v), want (\"value\", true)", got, ok)
	}
}

func TestExtractString_nested(t *testing.T) {
	data := map[string]any{
		"routing": map[string]any{"target": "homeserver"},
	}
	got, ok := ExtractString(data, "routing.target")
	if !ok || got != "homeserver" {
		t.Errorf("got (%q, %v), want (\"homeserver\", true)", got, ok)
	}
}

func TestExtractString_missing(t *testing.T) {
	data := map[string]any{"other": "x"}
	_, ok := ExtractString(data, "key")
	if ok {
		t.Error("expected ok=false for missing key")
	}
}

func TestExtractString_wrongType(t *testing.T) {
	data := map[string]any{"key": 42}
	_, ok := ExtractString(data, "key")
	if ok {
		t.Error("expected ok=false for non-string value")
	}
}

func TestExtractString_deeplyNested(t *testing.T) {
	data := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": "deep",
			},
		},
	}
	got, ok := ExtractString(data, "a.b.c")
	if !ok || got != "deep" {
		t.Errorf("got (%q, %v), want (\"deep\", true)", got, ok)
	}
}

func TestExtractBool(t *testing.T) {
	data := map[string]any{"flag": true}
	got, ok := ExtractBool(data, "flag")
	if !ok || !got {
		t.Errorf("got (%v, %v), want (true, true)", got, ok)
	}
}

func TestApplyMapping_success(t *testing.T) {
	raw := []byte(`{"text":"Hello","room_id":"!abc:matrix.org","sender":"@user:server"}`)
	mapping := map[string]string{
		"body":    "text",
		"room_id": "room_id",
		"sender":  "sender",
	}
	msg, err := ApplyMapping(raw, mapping)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Body != "Hello" {
		t.Errorf("body: got %q, want %q", msg.Body, "Hello")
	}
	if msg.RoomID != "!abc:matrix.org" {
		t.Errorf("room_id: got %q, want %q", msg.RoomID, "!abc:matrix.org")
	}
	if msg.Sender != "@user:server" {
		t.Errorf("sender: got %q, want %q", msg.Sender, "@user:server")
	}
}

func TestApplyMapping_missingBody(t *testing.T) {
	raw := []byte(`{"room_id":"!abc:matrix.org"}`)
	mapping := map[string]string{
		"body":    "text",
		"room_id": "room_id",
	}
	_, err := ApplyMapping(raw, mapping)
	if err == nil {
		t.Error("expected error for missing body")
	}
}

func TestApplyMapping_invalidJSON(t *testing.T) {
	_, err := ApplyMapping([]byte(`not json`), map[string]string{"body": "text"})
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestExtractString_jsonEncodedString(t *testing.T) {
	data := map[string]any{
		"output": `{"body":"hello","dest":"homeserver"}`,
	}
	got, ok := ExtractString(data, "output.body")
	if !ok || got != "hello" {
		t.Errorf("got (%q, %v), want (\"hello\", true)", got, ok)
	}
}

func TestExtractString_jsonEncodedStringMissingKey(t *testing.T) {
	data := map[string]any{
		"output": `{"body":"hello"}`,
	}
	_, ok := ExtractString(data, "output.missing")
	if ok {
		t.Error("expected ok=false for missing key inside JSON-encoded string")
	}
}

func TestExtractString_nonJsonString(t *testing.T) {
	data := map[string]any{
		"output": "not json",
	}
	_, ok := ExtractString(data, "output.body")
	if ok {
		t.Error("expected ok=false when string value is not JSON")
	}
}

func TestApplyMapping_nestedPaths(t *testing.T) {
	raw := []byte(`{"meta":{"sender":"@user:s"},"payload":{"text":"hi"},"room":"!x:y"}`)
	mapping := map[string]string{
		"body":    "payload.text",
		"room_id": "room",
		"sender":  "meta.sender",
	}
	msg, err := ApplyMapping(raw, mapping)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Body != "hi" || msg.RoomID != "!x:y" || msg.Sender != "@user:s" {
		t.Errorf("unexpected result: %+v", msg)
	}
}
