# Configuration Reference

mx-proxy is configured via a single TOML file passed with `--config <path>`.

---

## `[upstream]`

```toml
[upstream]
  homeserver = "http://127.0.0.1:8008"
```

| Key          | Required | Description                       |
| :----------- | :------- | :-------------------------------- |
| `homeserver` | yes      | Base URL of the Matrix homeserver |

---

## `[listen]`

```toml
[listen]
  cs = "127.0.0.1:8900"   # bridges connect here instead of the homeserver
  as = "127.0.0.1:8901"   # homeserver pushes appservice transactions here
```

| Key  | Required | Description                                                 |
| :--- | :------- | :---------------------------------------------------------- |
| `cs` | yes      | Address mx-proxy listens on for bridge → homeserver traffic |
| `as` | yes      | Address mx-proxy listens on for homeserver → bridge traffic |

---

## `[[bridges]]`

One entry per appservice bridge. `hs_token` must match the value the homeserver sends
in the `Authorization` header when calling the bridge's transaction endpoint.

```toml
[[bridges]]
  name        = "whatsapp"
  url         = "http://127.0.0.1:29318"
  hs_token    = "secret_token_from_registration_yaml"
  user_prefix = "whatsapp_"

[[bridges]]
  name        = "gmessages"
  url         = "http://127.0.0.1:29336"
  hs_token    = "another_secret_token"
  user_prefix = "gmessages_"
```

| Key           | Required | Description                                                                                     |
| :------------ | :------- | :---------------------------------------------------------------------------------------------- |
| `name`        | yes      | Human-readable label (used in logs and as the `Bridge` field in send_template)                  |
| `url`         | yes      | Real bridge AS endpoint base URL                                                                |
| `hs_token`    | yes      | Token used to identify and route to this bridge                                                 |
| `user_prefix` | no       | Ghost-user MXID localpart prefix (e.g. `"gmessages_"`). When set, incoming CS-path messages whose sender MXID matches this prefix will have `Bridge` populated in the send_template. |

---

## `[processor]`

Configures the external processor that receives intercepted events.

```toml
[processor]
  transport = "unix"
  endpoint  = "/var/run/mx-proxy/processor.sock"

  send_template = '{"workflow":"mx-message","params":{"text":{{.Body | json}},"room":{{.RoomID | json}},"sender":{{.Sender | json}},"event_id":"{{.EventID}}"}}'

  [processor.receive_mapping]
    body    = "text"
    room_id = "room_id"
    sender  = "sender"
```

| Key             | Required | Description                                                       |
| :-------------- | :------- | :---------------------------------------------------------------- |
| `transport`     | yes      | One of `unix`, `websocket`, `http`                                |
| `endpoint`      | yes      | Socket path (unix) or URL (websocket/http)                        |
| `send_template` | no       | Go `text/template` string; defaults to raw Matrix event JSON      |

See [transports.md](transports.md) for transport-specific details.
See [wire-schema.md](wire-schema.md) for template fields and receive mapping keys.

---

## Full example

```toml
[upstream]
  homeserver = "http://127.0.0.1:6167"

[listen]
  cs = "127.0.0.1:6168"
  as = "127.0.0.1:6169"

[[bridges]]
  name        = "whatsapp"
  url         = "http://127.0.0.1:29318"
  hs_token    = "sY3xBzRvPO3YDzcG5nsWAK0yF5sFiJKacF5MK0vLupkzcIxd4N41UbOQwmGvFAGn"
  user_prefix = "whatsapp_"

[[bridges]]
  name        = "gmessages"
  url         = "http://127.0.0.1:29336"
  hs_token    = "7Jsldl7UKaF3ECdg73hehldzhwr6ET6eW68xYMeoK3Oyz7LguWvGl8YDTsSwDZMK"
  user_prefix = "gmessages_"

[processor]
  transport = "unix"
  endpoint  = "/var/run/mx-proxy/processor.sock"
  send_template = '{"workflow":"mx-message","params":{"text":{{.Body | json}},"room":{{.RoomID | json}},"sender":{{.Sender | json}},"event_id":"{{.EventID}}","bridge":{{.Bridge | json}},"msg_type":{{.MsgType | json}}}}'

  [processor.receive_mapping]
    body    = "text"
    room_id = "room_id"
    sender  = "sender"
```
