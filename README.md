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
- **No remote loads by default.** The HTML view injects a CSP that blocks every
  remote fetch. "Load images" routes them through `GET /api/v1/proxy?u=`, fetched
  server-side with an SSRF guard (no private, loopback, link-local or CGNAT
  targets, no redirects, images only, 5 MB cap).
- **`DROP_ATTACHMENTS=true`** discards attachment parts at parse time.
- **Keep forever.** `ttlHours: -1` on create or `PATCH /api/v1/mailbox/{address}`
  pins a mailbox (subscriptions); the UI shows a pin and a toggle. Everything else
  expires after 1h to 7d.
- No connection-status chrome: the WebSocket just runs; the sidebar lists mailboxes
  as rows (name, unread, copy) and hides the per-user tag.
- Same-host `Origin` check on the WebSocket, owner check on `subscribe`.
- SQLite pragmas set through the DSN so `foreign_keys` holds on every connection.
- `shitmail health` subcommand for the distroless `HEALTHCHECK`.
- Google Fonts preconnect removed; goreleaser, installers and Windows resources dropped.

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
