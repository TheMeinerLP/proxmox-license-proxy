# REST API v1 (ACME-style)

The `/api/v1` API issues and manages subscription keys the way Let's Encrypt
issues certificates: each client (a Proxmox host) has an **account key**, every
request is **JWS-signed**, and **replay nonces** prevent capture-and-replay.
It is inspired by ACME (RFC 8555) but trimmed to what this tool needs.

The product version is **2.0.0**; this API is at its first version, `v1`. The
two are independent - a future breaking API change would add `/api/v2`, not a
new product major.

## Concepts

- **Account** - a client identity, keyed by the RFC 7638 JWK **thumbprint** of
  its Ed25519 public key. The thumbprint is the account id (`kid`). An account is
  `PENDING` until approved (by an admin, or automatically when it registered from
  an `auto_approve` network); only an `APPROVED` account may issue.
- **Subscription** - a lab key assigned to an account (and the host id it
  reported), one per product. The server can `REVOKE` it; `verify.php` then
  reports the host unsubscribed on its next check.

## Signing (clients)

1. `GET /api/v1/new-nonce` → read the `Replay-Nonce` response header.
2. Build a flattened JWS:
   - protected header: `{"alg":"EdDSA","nonce":"<nonce>","url":"<full request URL>", "kid":"<thumbprint>"}`
     (use `"jwk":{...}` instead of `kid` for `new-account`),
   - payload: base64url JSON (empty for POST-as-GET),
   - signature: Ed25519 over `base64url(protected) + "." + base64url(payload)`.
3. POST `{"protected":"…","payload":"…","signature":"…"}`.
4. Every response returns a fresh `Replay-Nonce` to chain the next call.

## Endpoints

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/api/v1/directory` | none | endpoint URL map |
| GET/HEAD | `/api/v1/new-nonce` | none | issue a replay nonce |
| POST | `/api/v1/new-account` | JWS (jwk) | register/refresh an account |
| POST | `/api/v1/new-order` | JWS (kid) | issue a subscription per product |
| POST | `/api/v1/subscriptions` | JWS (kid) | list the account's subscriptions |
| POST | `/api/v1/revoke` | JWS (kid) | revoke one of the account's own keys |

`new-order` payload: `{"serverid":"…","products":["pve","pbs"],"level":"c","sockets":"2"}`.
Response: `{"serverid","status":"valid|pending","subscriptions":[{product,key,…}],"pending":[…],"problems":{…}}`.
When the account is not approved yet, `status` is `pending` and `subscriptions`
is empty - poll again after approval.

## Admin endpoints (bearer token)

Set `api.admin_token` and send `Authorization: Bearer <token>`.

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/v1/admin/accounts` | list accounts |
| POST | `/api/v1/admin/accounts/{thumbprint}/approve` \| `/block` | gate self-issuance |
| GET | `/api/v1/admin/hosts` | list hosts |
| POST | `/api/v1/admin/hosts/{id}/approve` \| `/block` | host status |
| GET | `/api/v1/admin/subscriptions` | list all subscriptions |
| POST | `/api/v1/admin/subscriptions/{key}/revoke` | invalidate a key |
| DELETE | `/api/v1/admin/subscriptions/{key}` | remove a key |

The same operations are available locally on the proxy host via the CLI
(`account …`, `server …`, `subscription …`), which needs no token.

## Client

`proxmox-license-proxy client enroll` performs the whole flow (detect products,
trust the CA, account key, register, order, install each key). It is the
reference client; the wire format above lets you script your own.
