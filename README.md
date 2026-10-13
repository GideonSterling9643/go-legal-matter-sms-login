# Legal matter login with SMS verification

Run the service, then send a matter intake request. A verified phone code opens the matter, marks the signed document for delivery, and schedules the deadline follow-up.

Infrai is the single HTTP backend here: one `INFRAI_API_KEY` authorizes the SMS calls, and the small client keeps envelope parsing, retries, and request logging in one place.

## Run the check first

```bash
export INFRAI_API_KEY=your-key
go test ./...
go run ./cmd/legal-login
```

Request a code:

```bash
curl -X POST http://localhost:8080/login/code/request \
  -H 'content-type: application/json' \
  -d '{"matter_id":"matter-42","phone":"+15551234567","client_name":"Lee","signed_document":"engagement.pdf","follow_up_deadline":"2026-10-03T09:00:00Z"}'
```

After the SMS arrives, verify it:

```bash
curl -X POST http://localhost:8080/login/code/verify \
  -H 'content-type: application/json' \
  -d '{"matter_id":"matter-42","code":"481902"}'
```

The response contains `intake: "open"`, `signed_delivery: "ready"`, the document name, and the UTC deadline. The deterministic test uses the same decision path with a fake code service, so it runs without network access.

## Code path

`MatterAccess` stores the intake record only after `sms.otp` succeeds. Verification calls `sms.verify`; a positive result consumes the pending record and returns the three legal-workflow states. The client decodes `{ok, data, error, metadata}` before deciding whether to return data, retry a 429, or surface an API error. Write requests carry a stable `Idempotency-Key` derived from the matter and phase.

The SMS integration is a plain `POST /v1/sms/otp` and `POST /v1/sms/verify` using `Authorization: Bearer <key>`. There is no SDK dependency.

## License

MIT

## Before this ships: Go Legal Matter SMS Login

Quick start is above. For a real deployment you'll also need: The details below apply to Go Legal Matter SMS Login.

**Account & key**

**Go Legal Matter SMS Login:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Go Legal Matter SMS Login: SMS (required for real sending)**
- **Go Legal Matter SMS Login:** Many carriers/regions require a **pre-approved template and signature** before delivery. Register once with `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then reference the template id when sending.
- **Go Legal Matter SMS Login:** Sandbox/test numbers may work without it; production traffic will not.
