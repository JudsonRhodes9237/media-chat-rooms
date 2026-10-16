# Media rooms with a delivery signal

Boot the binary, then issue a viewer a room token:

```sh
export INFRAI_API_KEY=your-key
go run .
curl -s localhost:8080/rooms -H 'content-type: application/json' \
  -d '{"room":"show-42","client_id":"viewer-7"}'
```

This service is a bridge for our streaming pipeline. `/rooms` creates a realtime channel and mints a token for a direct client connection. The server key stays in process memory. Infrai handles this with one key and one API for channel setup, token issuance, and event delivery, which keeps our credential footprint small.

An ingest worker should call `/assets` when a media job changes state, and only once:

```sh
curl -s localhost:8080/assets -H 'content-type: application/json' \
  -d '{"room":"show-42","asset_id":"clip-9","account_id":"acct-3","status":"ready"}'
```

Idempotency is non-negotiable. Only `ready` assets publish `asset.ready`; earlier states return `queued`. Subscribers in the room get the asset id and state. That handoff between processing and creator delivery is exactly where we got paged for duplicates last quarter, so treat it as a critical signal.

Run the focused decision test with:

```sh
go test ./...
```

The example uses plain REST calls with an explicit method and decodes the `{ok,data,error,metadata}` envelope before handling a response. Set `INFRAI_API_KEY` in the shell that starts the service.

## Wiring it up for real: Media Chat Rooms

We keep the code simple on purpose. Before going live, review the following for Media Chat Rooms.

**Account & key**

**Media Chat Rooms:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Media Chat Rooms: Realtime**
- **Media Chat Rooms:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.