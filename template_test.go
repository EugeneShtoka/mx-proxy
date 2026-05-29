package main

import (
	"encoding/json"
	"testing"
)

func TestCompileTemplate_empty(t *testing.T) {
	et, err := CompileTemplate("")
	if err != nil {
		t.Fatal(err)
	}
	data := TemplateData{RoomID: "!abc:server", Sender: "@user:server", Body: "hello"}
	out, err := et.Render(data)
	if err != nil {
		t.Fatal(err)
	}
	var got TemplateData
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("fallback output is not valid JSON: %v\noutput: %s", err, out)
	}
	if got.Body != "hello" {
		t.Errorf("body: got %q, want %q", got.Body, "hello")
	}
}

func TestCompileTemplate_render(t *testing.T) {
	src := `{"text":"{{.Body}}","room":"{{.RoomID}}"}`
	et, err := CompileTemplate(src)
	if err != nil {
		t.Fatal(err)
	}
	data := TemplateData{Body: "hi there", RoomID: "!room:server"}
	out, err := et.Render(data)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}
	if got["text"] != "hi there" {
		t.Errorf("text: got %q, want %q", got["text"], "hi there")
	}
	if got["room"] != "!room:server" {
		t.Errorf("room: got %q, want %q", got["room"], "!room:server")
	}
}

func TestCompileTemplate_jsonFunc(t *testing.T) {
	src := `{"body":{{.Body | json}},"room":{{.RoomID | json}}}`
	et, err := CompileTemplate(src)
	if err != nil {
		t.Fatal(err)
	}
	data := TemplateData{Body: `say "hello" \world`, RoomID: "!room:server"}
	out, err := et.Render(data)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}
	if got["body"] != `say "hello" \world` {
		t.Errorf("body: got %q, want %q", got["body"], `say "hello" \world`)
	}
}

func TestCompileTemplate_invalid(t *testing.T) {
	_, err := CompileTemplate("{{.Unclosed")
	if err == nil {
		t.Error("expected error for invalid template, got nil")
	}
}

func TestRender_allFields(t *testing.T) {
	src := `{{.EventID}} {{.Sender}} {{.MsgType}} {{.TS}}`
	et, err := CompileTemplate(src)
	if err != nil {
		t.Fatal(err)
	}
	data := TemplateData{
		EventID: "$evt:server",
		Sender:  "@alice:server",
		MsgType: "m.text",
		TS:      1234567890,
	}
	out, err := et.Render(data)
	if err != nil {
		t.Fatal(err)
	}
	want := "$evt:server @alice:server m.text 1234567890"
	if string(out) != want {
		t.Errorf("got %q, want %q", out, want)
	}
}
