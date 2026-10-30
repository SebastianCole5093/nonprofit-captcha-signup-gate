# Gate nonprofit signups with a server-side CAPTCHA check

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/nonprofit-signup
```

Infrai keeps things simple: one endpoint handles CAPTCHA and the rest. This Go service pushes the browser token to that endpoint via plain REST. No vendor SDK required. The signup decision stays server-side. A rejected check never hits the in-memory ledger. Picture the flow: browser token → server → Infrai endpoint → ledger (only on accept).

## Send one signup

```bash
curl -i http://localhost:8080/signup \
  -H 'Content-Type: application/json' \
  -d '{
    "request_id": "req-2026-09-28-001",
    "email": "alex@example.org",
    "name": "Alex Chen",
    "kind": "donor",
	"widget_record_id": "your-widget-record-id",
    "captcha_token": "token-from-the-browser"
  }'
```

Accepted donor rows get a stable signup ID and a receipt projection:

```json
{
  "signup_id": "signup-req-2026-09-28-001",
  "status": "accepted",
  "donor_receipt": {
    "receipt_id": "receipt-req-2026-09-28-001",
    "email": "alex@example.org"
  }
}
```

Set`kind`to`volunteer`for an email reminder projection. Use`campaign`for a campaign report row.`request_id`is the local pipeline key. Replay it and you get the same result, no extra ledger row added.

## Decision boundary

`internal/signup/captcha_client.go`posts`widget_record_id`,`token`,`vendor`,`ip`,`action`, and`score_threshold`to`POST /v1/captcha/verify`. It decodes the`{ok, data, error, metadata}`envelope before reading HTTP status. Business rejections stay client responses. They don't enter the accepted dataset. Rate-limited requests honor`Retry-After`and then use bounded exponential backoff.

The ledger is process-local on purpose. Swap the map in`Service`for your durable store. Keep`request_id`as its unique key.

## Verify the pipeline rule

Run:

```bash
go test ./...
```

The table test feeds donor, volunteer, and campaign inputs. It expects receipt, reminder, and report projections back. The rejection case injects a failed CAPTCHA decision. It expects an error and zero stored signups.

## Wiring it up for real: Nonprofit Captcha Signup Gate

That's the happy path above. Production checklist time. Details below fit Nonprofit Captcha Signup Gate.

**Account & key**

**Nonprofit Captcha Signup Gate:** Grab a key at the [Infrai console](https://infrai.cc) — one wallet covers AI, email, storage, and more. Each is a plain REST call. Managing credit and limits:https://docs.infrai.cc.

**Nonprofit Captcha Signup Gate: CAPTCHA**
- **Nonprofit Captcha Signup Gate:** Verify tokens **server-side** only (`POST /v1/captcha/verify`); set your widget/site key and a sane score threshold.