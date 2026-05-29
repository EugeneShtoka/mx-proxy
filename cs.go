package main

import (
	"encoding/json"
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

	if isForwardedByProxy(content) {
		forward(w, r, h.upstream, body)
		return
	}

	msgtype, _ := content["msgtype"].(string)
	if !isTextMsgtype(msgtype) {
		forward(w, r, h.upstream, body)
		return
	}

	if isEditEvent(content) {
		forward(w, r, h.upstream, body)
		return
	}

	msgBody, _ := content["body"].(string)
	if strings.TrimSpace(msgBody) == "" {
		forward(w, r, h.upstream, body)
		return
	}

	sender := r.URL.Query().Get("user_id")
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	h.router.CacheToken(sender, token)

	replyFallback, actualBody := splitReplyFallback(msgBody)

	data := TemplateData{
		RoomID:        roomID,
		Sender:        sender,
		Body:          actualBody,
		MsgType:       msgtype,
		ReplyFallback: replyFallback,
	}

	log.Printf("cs: intercepting message from %s in %s", sender, roomID)

	mapped, err := h.processor.Process(data)
	if err != nil {
		log.Printf("cs: processor error: %v — forwarding original", err)
		forward(w, r, h.upstream, body)
		return
	}
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
