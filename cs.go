package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
)

var csSendRE = regexp.MustCompile(`^/_matrix/client/[^/]+/rooms/([^/]+)/send/m\.room\.message/[^/]+$`)

type csHandler struct {
	upstream  string
	processor *Processor
	router    *Router
}

func newCSHandler(upstream string, processor *Processor, router *Router) *csHandler {
	return &csHandler{upstream: upstream, processor: processor, router: router}
}

func (h *csHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	m := csSendRE.FindStringSubmatch(r.URL.Path)
	if r.Method != http.MethodPut || m == nil {
		forward(w, r, h.upstream, body)
		return
	}
	roomID := m[1]

	authHdr := r.Header.Get("Authorization")
	if len(authHdr) > 20 {
		authHdr = authHdr[:20] + "..."
	}
	log.Printf("cs: raw send request url=%s user_id_param=%q auth=%q body=%s",
		r.URL.RequestURI(), r.URL.Query().Get("user_id"), authHdr, body)

	var content map[string]any
	if err := json.Unmarshal(body, &content); err != nil {
		forward(w, r, h.upstream, body)
		return
	}

	sender := r.URL.Query().Get("user_id")
	data, ok := extractCSMessageData(content, roomID, sender)
	if !ok {
		forward(w, r, h.upstream, body)
		return
	}

	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	h.router.CacheToken(sender, token)
	log.Printf("cs: intercepting message from %s in %s", sender, roomID)

	mapped, err := h.processor.Process(data)
	if err != nil {
		log.Printf("cs: processor error: %v — forwarding original", err)
		forward(w, r, h.upstream, body)
		return
	}

	switch mapped.Status {
	case "ok":
		if mapped.ReplyFallback != "" {
			mapped.Body = mapped.ReplyFallback + "\n\n" + mapped.Body
		}
		mapped.OriginalContent = content
		eventID, err := h.router.Route(mapped)
		if err != nil {
			log.Printf("cs: route error: %v", err)
			http.Error(w, "routing error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"event_id": eventID})

	case "drop":
		log.Printf("cs: dropping message from %s in %s", sender, roomID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"event_id": fmt.Sprintf("$mx-proxy-dropped-%s", nextTxnID())})

	default: // "passthrough" or unknown
		forward(w, r, h.upstream, body)
	}
}

// extractCSMessageData checks whether a client-server send event is interceptable
// and extracts template fields. Returns (data, false) for events that should be
// forwarded to the homeserver unchanged.
func extractCSMessageData(content map[string]any, roomID, sender string) (TemplateData, bool) {
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
	replyFallback, actualBody := splitReplyFallback(msgBody)
	return TemplateData{RoomID: roomID, Sender: sender, Body: actualBody, MsgType: msgtype, ReplyFallback: replyFallback}, true
}

func isTextMsgtype(msgtype string) bool {
	switch msgtype {
	case "m.text", "m.notice", "m.emote":
		return true
	}
	return false
}

func isEditEvent(content map[string]any) bool {
	relates, ok := content["m.relates_to"].(map[string]any)
	if !ok {
		return false
	}
	return relates["rel_type"] == "m.replace"
}

func isForwardedByProxy(content map[string]any) bool {
	v, ok := content["io.mx-proxy.forwarded"].(bool)
	return ok && v
}

// splitReplyFallback splits a Matrix reply body into the "> quote" prefix and
// the actual message text. Returns ("", body) for non-reply messages.
func splitReplyFallback(body string) (fallback, actual string) {
	idx := strings.Index(body, "\n\n")
	if idx == -1 {
		return "", body
	}
	prefix := body[:idx]
	for _, line := range strings.Split(prefix, "\n") {
		if !strings.HasPrefix(line, "> ") {
			return "", body
		}
	}
	return prefix, body[idx+2:]
}
