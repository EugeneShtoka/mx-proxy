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

		if event["type"] != "m.room.message" {
			remaining = append(remaining, rawEvent)
			continue
		}

		content, _ := event["content"].(map[string]any)

		if isForwardedByProxy(content) {
			remaining = append(remaining, rawEvent)
			continue
		}

		msgtype, _ := content["msgtype"].(string)
		if !isTextMsgtype(msgtype) {
			remaining = append(remaining, rawEvent)
			continue
		}

		if isEditEvent(content) {
			remaining = append(remaining, rawEvent)
			continue
		}

		msgBody, _ := content["body"].(string)
		if strings.TrimSpace(msgBody) == "" {
			remaining = append(remaining, rawEvent)
			continue
		}

		roomID, _ := event["room_id"].(string)
		sender, _ := event["sender"].(string)
		eventID, _ := event["event_id"].(string)
		var ts int64
		if tsFloat, ok := event["origin_server_ts"].(float64); ok {
			ts = int64(tsFloat)
		}

		data := TemplateData{
			EventID: eventID,
			RoomID:  roomID,
			Sender:  sender,
			Body:    msgBody,
			MsgType: msgtype,
			TS:      ts,
		}

		log.Printf("as: intercepting message from %s in %s (bridge: %s)", sender, roomID, bridge.Name)

		mapped, err := h.processor.Process(data)
		if err != nil {
			log.Printf("as: processor error: %v — passing event to bridge", err)
			remaining = append(remaining, rawEvent)
			continue
		}
		mapped.OriginalContent = content

		if _, err := h.router.Route(mapped); err != nil {
			log.Printf("as: route error: %v", err)
		}
	}

	modifiedTxn, _ := json.Marshal(map[string]any{"events": remaining})
	forward(w, r, bridge.URL, modifiedTxn)
}
