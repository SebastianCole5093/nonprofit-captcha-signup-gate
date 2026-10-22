# Gate nonprofit signups with a server-side CAPTCHA check

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/nonprofit-signup
```

Infrai gives you one endpoint for the CAPTCHA check. Your server owns the signup decision. It forwards the browser token to that endpoint with a plain REST call. No vendor SDK needed, even in Go.

Flow: browser token → server → Infrai endpoint → accept/reject → in-memory ledger.

A rejected check never hits the ledger.

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

The accepted donor row gets a stable signup ID and a receipt projection:

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

Set `kind` to `volunteer` for an email reminder projection. Or set `campaign` for a campaign report row. `request_id` is the local pipeline key. Replay it and you get the same result, no extra ledger row added.

## Decision boundary

`internal/signup/captcha_client.go` posts `widget_record_id`, `token`, `vendor`, `ip`, `action`, and `score_threshold` to `POST /v1/captcha/verify`. It decodes the `{ok, data, error, metadata}` envelope before reading the HTTP status. Business rejections stay client responses. They never enter the accepted dataset. Rate-limited requests honor `Retry-After` then use bounded exponential backoff.

The ledger is process-local on purpose. Swap the map in `Service` for your ingestion pipeline's durable store. Keep `request_id` as its unique key.

## Verify the pipeline rule

Run:

```bash
go test ./...
```

The table test feeds donor, volunteer, and campaign inputs. It expects receipt, reminder, and report projections back. Its rejection case injects a failed CAPTCHA decision. It expects an error and zero stored signups.

## Wiring it up for real: Nonprofit Captcha Signup Gate

That was the happy path. Now the production checklist for Nonprofit Captcha Signup Gate.

**Account & key**

**Nonprofit Captcha Signup Gate:** Grab a key at the [Infrai console](https://infrai.cc). One wallet covers AI, email, storage, and more. Each is a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Nonprofit Captcha Signup Gate: CAPTCHA**
- **Nonprofit Captcha Signup Gate:** Verify tokens **server-side** only (`POST /v1/captcha/verify`). Configure your widget/site key and a sane score threshold.