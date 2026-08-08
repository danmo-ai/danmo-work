# tunnel/v1 — Danmo Hub reverse HTTP tunnel

Shared wire protocol between **PC Connector** (`danmo-work`) and **danmo-hub**.
Both sides implement this document independently (no shared Go module required).

## Transport

- PC dials Hub: `wSS://<hub>/v1/connector` (TLS required in production).
- One WebSocket connection multiplexes many HTTP streams.
- WebSocket message type: **binary**.

## Frame layout

```
offset  size   field
0       4      payload_len   uint32 big-endian  (length of payload only)
4       1      type          uint8
5       4      stream_id     uint32 big-endian
9       N      payload       N == payload_len
```

Total frame size = `9 + payload_len`.

`stream_id == 0` for connection-level messages (Register, Heartbeat, Error without stream).

## Message types

| type | name          | direction   | payload                          |
|------|---------------|-------------|----------------------------------|
| 1    | Register      | PC → Hub    | JSON `RegisterPayload`           |
| 2    | RegisterOK    | Hub → PC    | JSON `RegisterOKPayload`         |
| 3    | Heartbeat     | bidirectional | JSON `HeartbeatPayload`        |
| 10   | HTTPOpen      | Hub → PC    | JSON `HTTPOpenPayload`           |
| 11   | HTTPBody      | bidirectional | `flags(u8) + bytes`            |
| 12   | HTTPRespOpen  | PC → Hub    | JSON `HTTPRespOpenPayload`       |
| 13   | StreamClose   | bidirectional | JSON `StreamClosePayload`      |
| 15   | Error         | bidirectional | JSON `ErrorPayload`            |

### HTTPBody flags

- bit0 (`0x01`): end of body (FIN). Payload bytes may be empty with FIN set.

### JSON payloads

```json
// Register
{"device_id":"…","device_secret":"…","app_version":"…"}

// RegisterOK
{"server_time_unix":1710000000}

// Heartbeat
{"ts_unix_ms":1710000000123}

// HTTPOpen
{"method":"GET","path":"/api/v1/sessions","headers":[["Accept","*/*"]]}

// HTTPRespOpen
{"status":200,"headers":[["Content-Type","text/event-stream"]]}

// StreamClose
{"code":0,"reason":"ok"}

// Error
{"code":"unauthorized","message":"bad device secret","stream_id":0}
```

`path` in HTTPOpen is the request URI path + raw query (must start with `/`).

## Lifecycle

1. PC connects WSS, sends `Register`.
2. Hub validates device credentials, replies `RegisterOK` (or `Error` then close).
3. Both sides send `Heartbeat` every **15s**. No frame for **90s** ⇒ close.
4. For each client HTTPS request, Hub allocates a new non-zero `stream_id` and sends `HTTPOpen`, then zero or more `HTTPBody`, ending with FIN.
5. PC proxies to `http://127.0.0.1:7801` (configurable), sends `HTTPRespOpen`, then body chunks with FIN, then optional `StreamClose`.
6. Either side may send `StreamClose` to cancel.

## Semantics

- Hub does **not** persist session event logs; it byte-forwards only.
- SSE (`Content-Type: text/event-stream`) is a normal long-lived HTTPBody stream.
- PC reconnects with exponential backoff **1s … 30s** after disconnect.

## Hub HTTPS surface (outside the tunnel)

| method | path | purpose |
|--------|------|---------|
| POST | `/v1/pair/code` | device_id+secret → short-lived pairing code |
| POST | `/v1/pair/exchange` | code → device-scoped bearer token |
| POST | `/v1/pair/revoke` | revoke token(s) for a device |
| GET/POST/… | `/api/v1/*` | authenticated reverse proxy through tunnel |

Bearer token on `/api/v1/*`: `Authorization: Bearer <token>`.  
Offline device ⇒ `503` with body `{"error":"device_offline"}`.
