# Wire Schema — Send Template & Receive Mapping

## Overview

mx-proxy communicates with the processor in a request/response pattern over the configured transport:

- **Outbound** (mx-proxy → processor): intercepted Matrix events, formatted by `send_template`.
- **Inbound** (processor → mx-proxy): JSON response; `status` field controls routing; payload extracted via `receive_mapping`.

---

## Send Template

`send_template` is a Go [`text/template`](https://pkg.go.dev/text/template) string
rendered against the intercepted event. The result is sent to the processor.

### Available fields

| Field      | Type   | Description                                                              |
| :--------- | :----- | :----------------------------------------------------------------------- |
| `EventID`  | string | Matrix event ID (`$abc:server`); empty on CS direction                   |
| `RoomID`   | string | Matrix room ID (`!abc:server`)                                           |
| `Sender`   | string | Matrix user ID of the sender                                             |
| `Body`     | string | Plaintext message body                                                   |
| `MsgType`  | string | `m.text`, `m.notice`, or `m.emote`                                       |
| `TS`       | int64  | Event timestamp (milliseconds since epoch); 0 on CS direction            |
| `Bridge`   | string | Bridge name (e.g. `"gmessages"`); populated from `user_prefix` on CS path, from the bridge entry on AS path; empty for direct user sends |

### Available functions

| Function | Example | Description |
| :------- | :------ | :---------- |
| `json`   | `{{.Body \| json}}` | JSON-serializes the value with quotes and escaping |

### Example

```toml
[processor]
  send_template = '{"workflow":"mx-message","params":{"text":{{.Body | json}},"room":{{.RoomID | json}},"sender":{{.Sender | json}},"event_id":"{{.EventID}}","bridge":{{.Bridge | json}},"msg_type":{{.MsgType | json}}}}'
```

If `send_template` is not set, mx-proxy sends the raw Matrix event JSON as-is.

---

## Response Format

The processor response must be a JSON object. mx-proxy reads the `status` field to decide
what to do:

| `status`    | CS direction                                 | AS direction                            |
| :---------- | :------------------------------------------- | :-------------------------------------- |
| `"ok"`      | Extract body via `receive_mapping`, route to homeserver | Route to homeserver |
| `"drop"`    | Return fake `event_id` to bridge, do not forward | Consume silently, not delivered to bridge |
| `"error"`   | Log message field, fall through to original forwarding | Log, fall through |
| *(absent)*  | Fall through to original forwarding          | Fall through to bridge                  |

For `"error"` responses, mx-proxy logs the `message` field if present.

---

## Receive Mapping

`receive_mapping` defines how mx-proxy extracts the message payload from a `status: "ok"` response.
Each key is a well-known field mx-proxy needs; the value is a dot-separated path into the
processor's JSON payload.

### Supported keys

| Key             | Type   | Required | Description                                                    |
| :-------------- | :----- | :------- | :------------------------------------------------------------- |
| `body`          | string | yes      | Path to the message body to forward                            |
| `room_id`       | string | no       | Path to the target room ID                                     |
| `sender`        | string | no       | Path to the sender Matrix user ID                              |
| `reply_fallback`| string | no       | Path to the `"> quote"` prefix to prepend for reply messages   |

### Dot-path traversal

Path values support dot-separated access into nested objects: `meta.body` extracts
`response["meta"]["body"]`. If any segment along the path is a JSON-encoded string rather
than an object, mx-proxy will transparently parse it and continue traversal.

### Example

Processor response:
```json
{
  "id":      "mx-proxy-a3f2…",
  "status":  "ok",
  "text":    "Hello, world!",
  "room_id": "!abc:matrix.org",
  "sender":  "@user:server"
}
```

Config:
```toml
[processor.receive_mapping]
  body    = "text"
  room_id = "room_id"
  sender  = "sender"
```

mx-proxy will route `"Hello, world!"` to the homeserver in room `!abc:matrix.org` on behalf
of `@user:server`.
