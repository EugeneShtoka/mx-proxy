# mx-proxy

A bidirectional proxy between a Matrix homeserver and one or more application service bridges. It intercepts `m.room.message` events in both directions and redirects them to a configured external processor.

## How it works

mx-proxy sits in two places in the Matrix event flow:

- **CS API** — bridges point their `homeserver_url` at mx-proxy instead of the real homeserver. When a bridge sends a text message, mx-proxy intercepts it and forwards it to the processor.
- **AS API** — the homeserver's appservice transaction URLs point at mx-proxy instead of bridges directly. mx-proxy intercepts text messages from the homeserver before they reach the bridge.

The processor has full ownership of intercepted messages. If it sends a message back, mx-proxy routes it to the specified destination (homeserver or a named bridge). If it sends nothing, the original message is dropped.

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
  name     = "whatsapp"
  url      = "http://127.0.0.1:29318"
  hs_token = "secret_hs_token"

[processor]
  transport = "unix"
  endpoint  = "/var/run/mx-proxy/processor.sock"
  send_template = '{"text": "{{.Body}}", "room": "{{.RoomID}}", "from": "{{.Sender}}"}'

  [processor.receive_mapping]
    body        = "output"
    destination = "target"
    room_id     = "target_room"
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
