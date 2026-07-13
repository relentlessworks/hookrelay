# HookRelay

> Agentic-first webhook relay and inspection service. The agent IS the interface.

HookRelay is a webhook relay and inspection service designed for AI agents to drive over plain HTTP. No UI, no SDK. The API is the product.

Create webhook endpoints, receive incoming webhooks, forward them to your services, and inspect every delivery — all through a simple plain-text API.

## Quick Start

```bash
make build
./hookrelay
```

The server starts on `:8080` with zero config. Data is persisted to a JSON file automatically.

## How It Works

1. **Get a token**: `POST /auth/request` → `POST /auth/verify` → bearer token
2. **Create an endpoint**: `POST /api/endpoints` with `target_url=https://your-service.com/webhook`
3. **Receive webhooks**: Any service sends to `POST /hook/hook_a1b2c` → HookRelay forwards to your target URL
4. **Inspect deliveries**: `GET /api/endpoints/hook_a1b2c/deliveries` → see status codes, bodies, and delivery results
5. **Manage**: `GET /api/endpoints`, `GET /api/endpoints/<handle>`, `DELETE /api/endpoints/<handle>`

## Principles

- **Plain text by default** — one labeled, grepable line per record. JSON on demand via `Accept: application/json` or `?format=json`.
- **Instructive errors** — every 4xx includes a hint telling the agent what to do next.
- **Self-documenting** — `GET /help` returns a one-page operating manual.
- **Simple auth** — OTP via email → long-lived bearer token.
- **Single static binary** — Go, zero external dependencies, deploys as one file.
- **Zero config defaults** — runs out of the box. Config: defaults < env < flags.
- **Multi-tenant** — workspaces isolate endpoints and deliveries per tenant.
- **Short stable handles** — every endpoint gets a handle like `hook_k7m2q`, every delivery gets `del_x1y2z`.

## Configuration

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `-addr` | `HOOKRELAY_ADDR` | `:8080` | Listen address |
| `-db` | `HOOKRELAY_DB` | `hookrelay.json` | Data file path |
| `-secret` | `HOOKRELAY_SECRET` | random | Token signing secret |

## Build

```bash
make build    # CGO_ENABLED=0, single static binary
make test     # go test ./...
make vet      # go vet ./...
```

## API Reference

### Authentication

```
POST /auth/request   email=<email>&workspace=<handle>  → OTP code
POST /auth/verify    email=<email>&code=<code>          → Bearer token
```

### Endpoints (requires Bearer token)

```
POST   /api/endpoints          target_url=<https://...>&description=<optional>  → handle=hook_xxx
GET    /api/endpoints                                              → list all endpoints
GET    /api/endpoints/<handle>                                     → endpoint details
DELETE /api/endpoints/<handle>                                     → delete endpoint
GET    /api/endpoints/<handle>/deliveries                          → list deliveries
GET    /api/deliveries/<handle>                                    → delivery details
GET    /api/workspace                                              → workspace info
```

### Webhook Receiver (public, no auth)

```
POST /hook/<handle>  → Forwards body to target_url, returns delivery handle + status
```

Any HTTP method is accepted (GET, POST, PUT, DELETE, PATCH).

### Response Formats

- **Plain text** (default): `handle=hook_a1b2c target_url=https://example.com/webhook deliveries=3`
- **JSON**: add `Accept: application/json` or `?format=json`

## License

MIT
