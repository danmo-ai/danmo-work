# Remote Hub (PC connector)

Mobile / remote clients reach a NAT’d Danmo Work PC through **danmo-hub** (separate repo: `../danmo-hub` or `github.com/danmo-ai/danmo-hub`).

Wire protocol: [tunnel-v1.md](./tunnel-v1.md).

## PC config

`~/.danmo-work/config.yaml`:

```yaml
remote:
  enabled: true
  hub_url: "wss://hub.example.com"   # or https://… / ws://… for dev
  local_base: "http://127.0.0.1:7801"
  tls_insecure: false                # true only for dev self-signed
```

Env overrides: `WORK_REMOTE_ENABLED`, `WORK_HUB_URL`, `WORK_REMOTE_TLS_INSECURE`.

Device identity is stored in `~/.danmo-work/remote.json`.

## Settings UI

Settings → **Danmo Hub**：开关、Hub 地址、TLS 跳过校验、本地 API 基址、deviceId、在线状态、生成/复制配对码、撤销 Token。保存后热更新 Connector（无需重启后端）。

## Local API

- `GET /api/v1/remote/status` — connection snapshot  
- `PUT /api/v1/remote` — save config + apply connector (`enabled`, `hubUrl`, `localBase`, `tlsInsecure`)  
- `POST /api/v1/remote/pair/code` — ask Hub for a pairing code (PC must be online)  
- `POST /api/v1/remote/pair/revoke` — revoke all Hub tokens for this device  

## Dev smoke

```bash
# terminal 1 — hub (plaintext)
cd ../danmo-hub && HUB_DEV_HTTP=1 HUB_ADDR=:8443 go run ./cmd/hub

# terminal 2 — PC backend
WORK_REMOTE_ENABLED=1 WORK_HUB_URL=ws://127.0.0.1:8443 WORK_REMOTE_TLS_INSECURE=1 make backend
```
