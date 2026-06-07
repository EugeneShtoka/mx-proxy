# mx-proxy

A bidirectional proxy between a Matrix homeserver and one or more application service bridges. It intercepts `m.room.message` events in both directions and redirects them to a configured external processor.

## How it works

mx-proxy sits in two places in the Matrix event flow:

- **CS API** — bridges point their `homeserver_url` at mx-proxy instead of the real homeserver. When a bridge sends a text message, mx-proxy intercepts it and forwards it to the processor.
- **AS API** — the homeserver's appservice transaction URLs point at mx-proxy instead of bridges directly. mx-proxy intercepts text messages from the homeserver before they reach the bridge.

The processor controls routing via a `status` field in its response:

| `status` | CS direction | AS direction |
|---|---|---|
| `"ok"` | Route message to homeserver | Route message to homeserver |
| `"drop"` | Return fake `event_id` to bridge; don't forward | Consume silently; don't deliver to bridge |
| `"error"` | Log error, fall through to original forwarding | Log error, fall through |
| *(absent)* | Fall through to original forwarding | Fall through to bridge |

Only `m.text`, `m.notice`, and `m.emote` messages are intercepted. Edit events, non-text msgtypes, and all other event types pass through unchanged.

## Build

```sh
go build -o mx-proxy .
```

## Usage

```sh
mx-proxy --config /path/to/config.toml
```

## Configuration

Configuration is a single TOML file. See [`.doc/configuration.md`](.doc/configuration.md) for the full reference.

```toml
[upstream]
  homeserver = "http://127.0.0.1:8008"

[listen]
  cs = "127.0.0.1:8900"
  as = "127.0.0.1:8901"

[[bridges]]
  name        = "whatsapp"
  url         = "http://127.0.0.1:29318"
  hs_token    = "secret_hs_token"
  user_prefix = "whatsapp_"   # optional: ghost-user MXID prefix for Bridge field in send_template

[processor]
  transport = "unix"
  endpoint  = "/var/run/mx-proxy/processor.sock"
  # send_template fields: .Body .RoomID .Sender .EventID .MsgType .TS .Bridge
  send_template = '{"workflow":"mx-message","params":{"text":{{.Body | json}},"room":{{.RoomID | json}},"sender":{{.Sender | json}},"event_id":"{{.EventID}}","bridge":{{.Bridge | json}}}}'

  # receive_mapping maps JSON keys in the processor response to MappedMessage fields.
  # Dot-notation descends into nested objects.
  [processor.receive_mapping]
    body    = "text"
    room_id = "room_id"
    sender  = "sender"
```

### Processor transports

| Transport   | Best for              | Config key                          |
| :---------- | :-------------------- | :---------------------------------- |
| `unix`      | Same-host deployments | `endpoint = "/path/to/socket"`      |
| `websocket` | Remote processor      | `endpoint = "ws://host:port/path"`  |
| `http`      | Stateless/serverless  | `endpoint = "http://host:port/path"`|

See [`.doc/transports.md`](.doc/transports.md) and [`.doc/wire-schema.md`](.doc/wire-schema.md) for details on the send template and receive mapping.

## Bridge setup

For each bridge:

1. Set the bridge's `homeserver_url` to `http://<mx-proxy-cs-addr>`.
2. In the homeserver's appservice registration, set the `url` to `http://<mx-proxy-as-addr>`.
3. Add a `[[bridges]]` entry in the mx-proxy config with the bridge's `hs_token` (from its registration YAML).

## License

MIT
