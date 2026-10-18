# shitmail


Disposable mailboxes behind your SSO. Fork of [MailTub](https://github.com/DML-Labs/MailTub)
by DML Labs (Apache 2.0), reworked for a single deployment model: one container,
receive-only SMTP fed by an upstream relay, web UI behind a reverse proxy that
authenticates users (Authentik forward-auth headers).

## Provenance: vibe coded

Everything on top of MailTub v1.0.0 in this repository was written by an AI coding
agent (Claude Code, Anthropic, model Claude Fable 5.1) in one session on
2026-09-21, directed by marc-schuetze. The human wrote the requirements, looked at
screenshots and test output, and decided; the agent wrote the code, the tests, the
commit messages and this README. Nobody has line-by-line reviewed the diff.

What that means for you: the Go tests cover the MIME parser, storage, TTL
resolution, the origin check and the SSRF guard; the SMTP session, the HTTP
handlers and the WebSocket path have no automated tests beyond the smoke script.
It runs in production for one household behind an SSO proxy. Read the code before
you expose it to anyone you care about.

## What changed against upstream

- **Ownership.** Every mailbox belongs to the SSO user who created it. Identity
  comes from `X-Authentik-Uid` / `X-Authentik-Username` / `X-Authentik-Groups`;
  requests without a uid get 401. Users only see and delete their own boxes,
  members of `ADMIN_GROUP` see all.
- **Collision-free addresses.** Addresses are `<name>-<tag>@<domain>`, the tag is
  `base36(sha256(uid))[:5]`. Two users may both pick `slack1`.
- **Server-side mailbox list.** `GET /api/v1/mailboxes` replaces the browser
  localStorage tab list. Nothing is auto-created; a deleted mailbox is gone, not
  replaced by a random one.
- **Text-only reader.** Uses plain text when available and extracts readable text
  and web links from HTML-only messages. Sender HTML never enters the page:
  scripts, CSS, images, embeds, and remote-loading attributes are discarded.
  Web links open only when clicked, in a new tab without a referrer.
- **`DROP_ATTACHMENTS=true`** discards attachment parts at parse time.
- **Keep forever.** `ttlHours: -1` on create or `PATCH /api/v1/mailbox/{address}`
  pins a mailbox (subscriptions); the UI shows a pin and a toggle. Everything else
  expires after 1h to 7d.
- No connection-status chrome: the WebSocket just runs; the sidebar lists mailboxes
  as rows (name with hash suffix, unread, copy); copying uses the full address.
- Same-host `Origin` check on the WebSocket, owner check on `subscribe`.
- SQLite pragmas set through the DSN so `foreign_keys` holds on every connection.
- `shitmail health` subcommand for the distroless `HEALTHCHECK`.
- Google Fonts preconnect removed; goreleaser, installers and Windows resources dropped.

- **IMAP pull** (`IMAP_HOST`) as an alternative ingest path: instead of the relay
  pushing SMTP to this host, shitmail logs into a catch-all mailbox, delivers
  every message through the same code as an SMTP session, then deletes it.
  For internal hosts with no inbound route. The original recipient comes from
  `X-Original-To` / `Delivered-To` / `X-Envelope-To` / `X-Forwarded-To`
  (`IMAP_ENVELOPE_HEADERS`), `To`/`Cc` as last resort — so Bcc-only mail through
  a catch-all cannot be attributed and is discarded. Messages are deleted only
  after storage succeeded; polling, not IDLE, so worst-case latency is
  `IMAP_POLL_INTERVAL` (default 60s).

Unknown addresses at the domain are accepted at SMTP and dropped silently
(upstream behaviour), so the upstream relay never generates bounces.

## Run

```
docker build -t shitmail .
docker run -p 8080:8080 -p 2525:2525 -v shitmail:/data \
  -e MAILTUB_DOMAIN=example.org -e DROP_ATTACHMENTS=true \
  shitmail
```

The HTTP port must only be reachable through the authenticating proxy; the app
trusts the identity headers unconditionally. Point the SMTP port at your MX
relay (`transport_maps` in Postfix) over a private network.

Configuration keys keep their upstream names (`MAILTUB_DOMAIN`, `MAILBOX_TTL`,
`SMTP_PORT`, ...), see `docs/configuration.md`. New: `DROP_ATTACHMENTS`,
`ADMIN_GROUP` (default `authentik Admins`).

## Develop

```
make build      # frontend + Go binary
go test ./...   # needs web/dist to exist (make frontend)
cd web && pnpm test
```

### Agent API (per-user bearer tokens)

In **Settings → Agent API tokens**, create a named token (7, 30, 90 or 365
 days). Copy the secret immediately; it is shown once. Revoke it in the same
panel. Tokens grant read-only access to all mailboxes owned by that user,
including future mailboxes, with no admin privileges. Only SHA-256 hashes are
stored. Revocation and expiry take effect on the next request.

The agent endpoint is `https://mail.example.com/api/agent/v1` and authenticates
only `Authorization: Bearer …`. It does not require a browser session or the
optional global `X-API-Key`. Token management remains behind browser SSO at
`/api/v1/tokens` (GET, POST, DELETE /{id}). POST accepts `name` and
`expiresInDays` (1–365, default 30).

```sh
# Supply SHITMAIL_TOKEN through your agent's secret store.
curl --fail --silent --show-error \
  -H "Authorization: Bearer $SHITMAIL_TOKEN" \
  https://mail.example.com/api/agent/v1/mailboxes

curl --fail --silent --show-error \
  -H "Authorization: Bearer $SHITMAIL_TOKEN" \
  'https://mail.example.com/api/agent/v1/mailbox/NAME-HASH@mail.example.com/emails?limit=20&offset=0'
```

Read a complete message with `GET /mailbox/{address}/emails/{id}`; attachment
 downloads use `GET /mailbox/{address}/emails/{id}/attachments/{attachmentId}`.
Responses use the existing mail JSON format (`bodyText`, `bodyHtml`, `subject`,
`from`, `receivedAt`). For magic-link login flows, poll for the expected new
message and extract the link from its body. The API never opens links or loads
remote email content, and agent reads do not mark messages as read. Mailbox
creation, deletion, token management and admin functions are unavailable to
agent tokens. Invalid, expired or revoked tokens return 401; inaccessible
mailboxes return 404.

Deployment: route only `/api/agent/v1/` past the SSO proxy to this service and
strip incoming `X-Authentik-*` identity headers on that route. Keep all other
paths behind SSO. The dedicated agent router validates bearer tokens itself.
