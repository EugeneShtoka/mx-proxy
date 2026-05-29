package main

import (
	"bytes"
	"encoding/json"
	"text/template"
)

type EventTemplate struct {
	t *template.Template
}

type TemplateData struct {
	EventID       string
	RoomID        string
	Sender        string
	Body          string
	MsgType       string
	TS            int64
	ReplyFallback string // "> quote" prefix lines from a Matrix reply, empty for non-replies
}

var templateFuncs = template.FuncMap{
	// json marshals any value to its JSON representation, including surrounding
	// quotes and escaping for strings. Use {{.Body | json}} instead of "{{.Body}}".
	"json": func(v any) (string, error) {
		b, err := json.Marshal(v)
		return string(b), err
	},
}

func CompileTemplate(src string) (*EventTemplate, error) {
	if src == "" {
		return &EventTemplate{}, nil
	}
	t, err := template.New("send").Funcs(templateFuncs).Parse(src)
	if err != nil {
		return nil, err
	}
	return &EventTemplate{t: t}, nil
}

func (et *EventTemplate) Render(data TemplateData) ([]byte, error) {
	if et.t == nil {
		return json.Marshal(data)
	}
	var buf bytes.Buffer
	if err := et.t.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
