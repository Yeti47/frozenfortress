# Frozen Fortress v2: architecture and conventions

This is the in-repo, condensed version of the v2 reference document. Read it before working on any v2 change. The full document, including the v1 feature parity inventory (the checklist the feature issues refer to), lives in Linear:

- [v2 Overhaul: Architecture, Decisions & v1 Parity Inventory](https://linear.app/yetibyte/document/v2-overhaul-architecture-decisions-and-v1-parity-inventory-87687145daec)
- Parent issue: YETI-78 (it holds the index from issue code, e.g. "v2 [1/e]", to YETI number, plus the blocking order)

The integration branch is `feature/v2`. Work branches are named `feature/YETI-<n>_<short-kebab-slug>`, are cut from `feature/v2` and open their PR against `feature/v2`, not `master`. Merge `master` into `feature/v2` regularly.

## 1. Why v2

v1's web UI is Gin server-side rendering (`webui/views/**`) with Alpine.js and many helpers attached to `window`. It works, but almost none of it is testable: there is one Go handler test, no front-end tests, and no workflow runs `go test` automatically.

v2 keeps `core/` and `cli/` as they are and replaces the web layer:

- **`frozenfortress` container:** Go JSON API (no server-side rendering), plus `ffcli` and the background workers (backup, update check).
- **`frozenfortress-ui` container:** Angular SPA served as static files.
- **Tests:** high coverage on both sides, enforced in CI, plus Playwright E2E.
- **Auth:** the session-based model stays (section 3).
- **Features:** full parity with v1. The UX may be redesigned and does not need to be a 1:1 port.

## 2. Decisions

| Topic | Decision |
| -- | -- |
| Legacy binary distribution | Dropped. v2 is Docker only. `build-*.sh`, `release-linux.sh`, `ff-setup.sh`, `run-webui.sh`, `stop-webui.sh`, `clean.sh`, `legacy-*.yml` and `doc/setup-binary.md` are removed in the release issue. |
| API contract | OpenAPI generated from the Go code. Huma v2 (`github.com/danielgtaylor/huma/v2`) with the `humagin` adapter. A snapshot is committed at `api/openapi.yaml` and CI checks for drift. The UI gets its TypeScript types via `openapi-typescript`. Fallback only if Huma truly cannot do something: swaggo/swag v2. |
| UI toolkit | Tailwind v4 + Angular CDK. Keep the v1 design tokens and identity. Build our own `ff-*` components on CDK primitives (dialog and focus trap, overlay, listbox/combobox, a11y). No Angular Material. |
| State | `@ngrx/signals` SignalStore. Signals throughout. Nothing on `window`. |
| Unit/component tests | Vitest through Angular's `unit-test` builder, plus `@testing-library/angular`. |
| E2E | Playwright against the composed stack, plus `@axe-core/playwright` a11y smoke tests. |
| Companion app | Not a frozen contract. v2 may change the scan-handoff routes and protocol. `companion-app-android/` is updated to match and released together with v2.0.0. |
| API versioning | Unversioned `/api/...`. There is one first-party client, and both images are released together, pinned by digest. |
| Tags | No routes of their own. Create and edit happen in a dialog. |
| Uploads | JPEG, PNG and PDF only, as in v1. The type is sniffed from the bytes. |
| Proxy trust | Compose trusts the private IP ranges via `FF_TRUSTED_PROXIES` instead of pinning a fixed subnet. |

## 3. Target architecture

```
browser ──https──> edge nginx (docker/nginx, TLS, 127.0.0.1:8443)
                     ├── /api/*  ──> frozenfortress:8080     (Go API + ffcli + workers)
                     └── /*      ──> frozenfortress-ui:8080  (nginx-unprivileged, Angular dist, SPA fallback)
frozenfortress ──> redis (sessions, scan handoff), ollama (OCR)
```

**Same origin is the central design decision. Do not break it.**

- There is no CORS.
- The cookies keep working as they are: the session cookie and the wrapping-key cookie `frozenfortress_swk` (`SameSite=Strict`, HttpOnly).
- Angular's built-in XSRF support works, because it only attaches the header to relative URLs.
- In dev, the Angular dev-server proxy (`proxy.conf.json`, `/api` to the Go API) gives the same property.

### How auth works (the two-cookie MEK model)

- The Redis-backed gorilla session `frozenfortress_session` stores `userId` and `ffmekw`, which is the MEK encrypted with a per-session wrapping key.
- The wrapping key lives only in the HttpOnly cookie `frozenfortress_swk` (`core/auth/cookiewrappingkeystore.go`). **Both cookies are needed for every authenticated request.**
- `GetCurrentUser` treats "no usable MEK" as signed out.
- Every encrypt/decrypt goes through `dataprotection.CreateMekDataProtectorForRequest(mekStore, enc, r)`.
- Session lifetime is an absolute `FF_SESSION_MAX_AGE_DAYS` (default 30), with no sliding renewal.
- There is no idle lock or vault lock in v1, and none is planned in v2.
- v2 adds cookie flags and CSRF protection (double-submit XSRF).

### Repo layout on `feature/v2`

- `api/`: the new Go main. It replaces `webui/`. Wiring is reused from `webui/ioc.go`; workers move from `webui/workers/`. Handlers go in `api/handlers/<resource>/`, shared test fakes in `api/internal/testutil/`, and the spec dump command in `api/cmd/openapi`.
- `ui/`: the Angular workspace.
- `webui/`: stays untouched until UI parity is reached, then is deleted in the release issue. This keeps merges of v1 hotfixes from `master` painless.
- `core/`, `cli/`: unchanged.

## 4. Conventions for every v2 change

### AGENTS.md is mandatory

- Docker images are pinned by digest.
- GitHub Actions are pinned by commit SHA, with the tag in a comment.
- npm uses exact versions (`.npmrc` `save-exact=true`), a committed `package-lock.json`, and `npm ci` in CI and Docker.
- Go uses exact versions in `go.mod` and `go.sum`.
- Containers run as non-root, with multi-stage builds.
- Never commit secrets, and never log `Authorization`, password or token values.
- Workflows declare explicit least-privilege `permissions:`.
- Verify every new dependency or digest against its official release.

### Tests are part of every change

- **Go:** `httptest` handler tests per endpoint, following the pattern in `webui/views/scanhandoff/scanhandoff_test.go` (fakes for SignInManager and MekStore, in-memory stores, a router built by the same registration function as production). Every endpoint covers:
  - the happy path;
  - validation errors;
  - 401 when unauthenticated;
  - cross-user access (user A must not be able to read or modify user B's IDs; expect 404);
  - CSRF rejection on unsafe methods.
- **UI:** Vitest unit tests for stores, services, pipes and pure functions, and Testing Library component tests for components and pages.
- **Coverage gates:** UI lines, statements and functions at least 95%, branches at least 90%. Go `api/...` at least 85%.
- Redis-dependent Go tests skip when no local Redis is available (see `requireLocalRedis` in `core/scanhandoff/redisscanhandoffstore_test.go`). CI provides a Redis service container.

### API conventions

- JSON is camelCase, using explicit json tags on API DTOs in `api/`. Core DTOs (`core/documents/datacontracts.go` etc.) have no json tags and serialize as PascalCase, so do not expose them directly; map them.
- Pagination envelope: `{ items, total, page, pageSize }`. Pages are 1-based. The default page size is 20, as in v1.
- Errors use RFC 9457 `application/problem+json` (the Huma default). Map `ccc.ApiError` (`core/ccc/errors.go`, used by `webui/middleware/errorhandling.go`) to its status and user message in **one** place. The `ccc` invalid-input errors carry the field name; expose it in the problem `errors[]` so the UI can show the message next to the field.
- Unauthenticated requests get 401 problem+json, never a redirect.
- Every `/api` response sends `Cache-Control: no-store`, `X-Content-Type-Options: nosniff` and `X-Frame-Options: DENY`.
- Validation limits are declared on the Huma input structs (`maxLength`, `pattern`, ...), so they appear in the OpenAPI spec and the UI can mirror them. Core is authoritative (section 5).

### UI conventions

- Standalone components, zoneless change detection, signals, `input()`/`output()`/`model()`, and the new control flow.
- No `window.*` globals. Inject `WINDOW`/`LOCATION`/`NAVIGATOR` tokens so code can be tested.
- List filters, sort and page live in URL query params. Stores derive server calls from them.
- Never persist secrets or decrypted data in browser storage. Reset all feature stores on logout.
- No inline `style` attributes or inline scripts that would need CSP `unsafe-inline`. The only exception is the hashed theme-boot snippet.

### Definition of done

- CI is green.
- Tests are added and the coverage gates hold.
- The OpenAPI snapshot is regenerated if the API changed.
- Docs are updated if behaviour changed.
- The PR links to the YETI issue and ticks the relevant parity checklist items.

## 5. Authoritative validation rules

These come from `core/`. v1's HTML used looser limits in places (password min 8, username max 64, tag name 64, title 200). **v2 must use the core rules.**

| Field | Rule | Source |
| -- | -- | -- |
| Username | `^[a-zA-Z0-9_]{3,20}$` | `core/auth/defaultusermanager.go:503` |
| Password | at least 16 characters; at least one lowercase, uppercase, digit and special character; only `A-Za-z0-9` plus `@ $ ! % * ? & # _ . , ; : + § / [ ] ( ) { } = -` | `core/auth/defaultusermanager.go:16,515-564` |
| Secret name / value | max 200 / max 1000 | `core/secrets/defaultsecretmanager.go:36-37` |
| Secret sort keys | `Name`, `CreatedAt`, `ModifiedAt` (also `Id`) | `core/secrets/defaultsecretmanager.go:341-353` |
| Tag name / colour | max 20 / `^#[0-9a-fA-F]{6}$` | `core/documents/tagmanager.go:34-44` |
| Document title / description | max 50 / max 200 | `core/documents/defaultdocumentmanager.go:489-490` |
| Document sort keys | `title`, `created_at`, `modified_at`, `issue_date`, `relevance` (search only) | `core/documents/sorting.go` |
| File name | max 255 | `core/documents/defaultdocumentfilecreator.go:180` |
| File size | 30 MB per file | `webui/views/documents/documents.go:19-24` |
| Note content | max 250 | `core/documents/notemanager.go:35` |
| Scan handoff TTL | 5 min | `core/scanhandoff/defaultscanhandoffservice.go:17` |

## 6. Pitfalls

1. **Companion app:** it is side-loaded, so there are no automatic updates. Release the updated APK together with v2.0.0 and say so prominently in the upgrade notes. Consider a protocol version so the server can reject outdated apps with a clear message.
2. **Two-cookie MEK in an SPA:**
   - The SPA cannot see HttpOnly cookies, so session state comes from `GET /api/auth/session`.
   - A 401 interceptor handles sessions that expire mid-use (absolute 30-day limit).
   - Guards must not flash protected content while the session check is still running.
3. **CSRF is new.** Login, register and recover are unauthenticated POSTs, so the XSRF cookie must be issued before login (on `GET /api/auth/session`).
4. **Session invalidation on upgrade:** cookie and route changes sign everyone out. Mention this in the release notes.
5. **Plaintext exposure:**
   - List endpoints never return secret values.
   - Secret responses are `no-store`.
   - Stores are wiped on logout.
   - Clipboard auto-clear for secrets.
6. **Huma specifics:**
   - The default `MaxBodyBytes` is 1 MB; raise it on upload operations.
   - Multipart and streaming need Huma's dedicated types.
   - Session and cookie code needs `humagin.Unwrap(ctx)` for the raw request and response.
7. **Version skew:** the UI build version is compared against `/api/system/info`, and the UI warns on a mismatch.
8. **Breaking ops changes:**
   - The compose service `webui` is renamed to `frozenfortress`, so `docker compose exec webui /app/ffcli ...` becomes `docker compose exec frozenfortress /app/ffcli ...`.
   - New image names and a new container.
   - The binary distribution is gone.
   - This needs an "Upgrading from v1" doc.
9. **CSP:**
   - Strict CSP: no `unsafe-inline` or `unsafe-eval`.
   - Dynamic tag colours go through CSS custom properties.
   - The theme-boot script is hashed.
   - Inline document views are sandboxed.
10. **Scope creep:** do parity first, then polish, in each feature PR.
11. **Long-lived branch:** merge `master` into `feature/v2` often. `webui/` is deleted only in the final release issue.

The v1 parity checklist and the list of latent v1 bugs to fix rather than port are in the Linear document linked at the top.
