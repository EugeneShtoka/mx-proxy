package main

import (
	"encoding/json"
	"fmt"
	"log"
)

type Processor struct {
	cfg       ProcessorConfig
	transport Transport
	tmpl      *EventTemplate
}

func NewProcessor(cfg ProcessorConfig, transport Transport, tmpl *EventTemplate) *Processor {
	return &Processor{cfg: cfg, transport: transport, tmpl: tmpl}
}

// Process sends data to the external processor and blocks until it responds.
// Returns a MappedMessage whose Status field drives routing:
//   - "ok"          → route to homeserver (body/room_id/sender extracted via receive_mapping)
//   - "drop"        → caller should drop/silently discard the message
//   - "passthrough" → no response configured; caller falls through to original behavior
//   - error         → workflow returned "error" status; caller falls through to original
func (p *Processor) Process(data TemplateData) (MappedMessage, error) {
	payload, err := p.tmpl.Render(data)
	if err != nil {
		return MappedMessage{}, fmt.Errorf("render template: %w", err)
	}
	log.Printf("processor: sending event_id=%s room=%s sender=%s", data.EventID, data.RoomID, data.Sender)
	resp, err := p.transport.SendRecv(payload)
	if err != nil {
		return MappedMessage{}, fmt.Errorf("transport: %w", err)
	}
	log.Printf("processor: response raw=%s", resp)

	var respObj map[string]any
	if err := json.Unmarshal(resp, &respObj); err != nil {
		return MappedMessage{}, fmt.Errorf("unmarshal response: %w", err)
	}

	status, _ := respObj["status"].(string)
	log.Printf("processor: status=%q", status)

	switch status {
	case "ok":
		mapped, err := ApplyMapping(resp, p.cfg.ReceiveMapping)
		if err != nil {
			return MappedMessage{}, err
		}
		mapped.Status = "ok"
		log.Printf("processor: mapped room=%s sender=%s body=%q", mapped.RoomID, mapped.Sender, mapped.Body)
		return mapped, nil

	case "drop":
		return MappedMessage{Status: "drop"}, nil

	case "error":
		msg, _ := respObj["message"].(string)
		return MappedMessage{}, fmt.Errorf("workflow error: %s", msg)

	default:
		// No status field — no response template ran; caller falls through.
		return MappedMessage{Status: "passthrough"}, nil
	}
}
