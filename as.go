package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
)

var asTxnRE = regexp.MustCompile(`^/_matrix/app/[^/]+/transactions/[^/]+$`)

type asHandler struct {
	cfg       *Config
	processor *Processor
	router    *Router
}

func newASHandler(cfg *Config, processor *Processor, router *Router) *asHandler {
	return &asHandler{cfg: cfg, processor: processor, router: router}
}

func (h *asHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	bridge := h.cfg.bridgeByHSToken(token)
	if bridge == nil {
		log.Printf("as: unknown hs_token — rejecting request")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"errcode":"M_FORBIDDEN","error":"unrecognized hs_token"}`))
		return
	}

	if r.Method != http.MethodPut || !asTxnRE.MatchString(r.URL.Path) {
		forward(w, r, bridge.URL, body)
		return
	}

	var txn struct {
		Events []json.RawMessage `json:"events"`
	}
	if err := json.Unmarshal(body, &txn); err != nil {
		forward(w, r, bridge.URL, body)
		return
	}

	var remaining []json.RawMessage
	for _, rawEvent := range txn.Events {
		var event map[string]any
		if err := json.Unmarshal(rawEvent, &event); err != nil {
			remaining = append(remaining, rawEvent)
			continue
		}
		data, ok := extractASMessageData(event)
		if !ok {
			remaining = append(remaining, rawEvent)
			continue
		}
		log.Printf("as: intercepting message from %s in %s (bridge: %s)", data.Sender, data.RoomID, bridge.Name)
		mapped, err := h.processor.Process(data)
		if err != nil {
			log.Printf("as: processor error: %v — passing event to bridge", err)
			remaining = append(remaining, rawEvent)
			continue
		}
		mapped.OriginalContent, _ = event["content"].(map[string]any)
		if _, err := h.router.Route(mapped); err != nil {
			log.Printf("as: route error: %v", err)
		}
	}

	modifiedTxn, _ := json.Marshal(map[string]any{"events": remaining})
	forward(w, r, bridge.URL, modifiedTxn)
}

// extractASMessageData checks whether a raw Matrix event is an interceptable
// message and extracts the fields needed for template rendering. Returns
// (data, false) for any event that should be left for the bridge unchanged.
func extractASMessageData(event map[string]any) (TemplateData, bool) {
	if event["type"] != "m.room.message" {
		return TemplateData{}, false
	}
	content, _ := event["content"].(map[string]any)
	if isForwardedByProxy(content) {
		return TemplateData{}, false
	}
	msgtype, _ := content["msgtype"].(string)
	if !isTextMsgtype(msgtype) {
		return TemplateData{}, false
	}
	if isEditEvent(content) {
		return TemplateData{}, false
	}
	msgBody, _ := content["body"].(string)
	if strings.TrimSpace(msgBody) == "" {
		return TemplateData{}, false
	}
	var ts int64
	if tsFloat, ok := event["origin_server_ts"].(float64); ok {
		ts = int64(tsFloat)
	}
	roomID, _ := event["room_id"].(string)
	sender, _ := event["sender"].(string)
	eventID, _ := event["event_id"].(string)
	return TemplateData{EventID: eventID, RoomID: roomID, Sender: sender, Body: msgBody, MsgType: msgtype, TS: ts}, true
}
