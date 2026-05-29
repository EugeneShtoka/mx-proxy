package main

import (
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
// Returns the mapped routing instruction from the processor's response.
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
	mapped, err := ApplyMapping(resp, p.cfg.ReceiveMapping)
	if err != nil {
		return MappedMessage{}, err
	}
	log.Printf("processor: mapped dest=%s room=%s sender=%s body=%q", mapped.Destination, mapped.RoomID, mapped.Sender, mapped.Body)
	return mapped, nil
}
