# Docker-Native Migration Plan

This document captures the planned move to a Docker-first Frozen Fortress release and the accompanying OCR redesign. The goal is to make Docker and Docker Compose the default deployment path while preserving a clear migration path for existing binary installations.

## Goals

- Make Docker Compose the primary self-hosted deployment path for Frozen Fortress.
- Keep existing binary installations migratable by carrying forward the SQLite database, persistent session keys, and backups.
- Use a single Frozen Fortress data mount for application state.
- Run all Frozen Fortress containers in a dedicated Docker network.
- Add a small nginx container for HTTPS entry, SSL termination, and proxying to the internal web UI.
- Automatically create a self-signed certificate when no certificate is present.
- Support drop-in existing certificates for smooth migration or production deployments.
- Allow deployments to use a remote Ollama instance, such as a GPU workstation, by changing configuration only.
- Make image OCR fully asynchronous so uploads do not wait for OCR completion.
- Use GLM OCR through Ollama as the primary image OCR provider.
- Keep Tesseract available as the fallback image OCR provider.
- Leave PDF processing unchanged.

## Target Docker Topology

The default Compose stack should include four services:

| Service | Purpose | Default exposure |
| --- | --- | --- |
| `nginx` | HTTPS entrypoint, SSL termination, reverse proxy | Published to the host |
| `webui` | Frozen Fortress web application | Dedicated Docker network only |
| `redis` | Session store and short-lived cache state | Dedicated Docker network only |
| `ollama` | Focused GLM OCR inference service | Dedicated Docker network only |

All services in the Compose stack should join a dedicated network, for example `frozenfortress`. Redis and Ollama must not run in Docker's implicit default network. The internal `webui` service should listen on HTTP only inside the dedicated network; nginx is responsible for the public HTTPS endpoint.

Only nginx should publish ports by default. The default documented bind should be conservative, such as `127.0.0.1:8443:443`, with explicit instructions for users who want LAN or internet exposure. nginx can also publish port `80` only if it redirects to HTTPS or is needed for a documented deployment mode.

The bundled Ollama container is intentionally small in scope: it exists for Frozen Fortress OCR by default. It should persist its model cache in its own Docker volume so `glm-ocr:q8_0` is not downloaded on every restart.

Ollama should not be published to the host by default. If users want to inspect or manage the bundled Ollama service directly, that should be an explicit documented debugging option.

## TLS Termination

nginx should terminate TLS and proxy requests to `webui` over the dedicated Docker network. Frozen Fortress should own certificate bootstrap so the default setup works without manual OpenSSL commands.

Recommended certificate paths:

```text
FF_TLS_CERT_DIR=/data/certs
FF_TLS_CERT_FILE=/data/certs/frozenfortress.crt
FF_TLS_KEY_FILE=/data/certs/frozenfortress.key
FF_TLS_CERT_COMMON_NAME=frozenfortress.local
```

Startup behavior:

1. If both the certificate and private key exist, use them unchanged.
2. If neither exists, generate a self-signed certificate and private key.
3. If only one file exists, fail clearly rather than overwriting partial user-provided material.
4. Mount the certificate directory into nginx read-only.
5. Configure nginx to use the certificate and proxy HTTPS traffic to `webui`.

Implementation options:

- Provide a small Frozen Fortress certificate bootstrap command in the application image and run it as a one-shot Compose service before nginx starts.
- Or run the same bootstrap during web UI startup and make nginx wait for the certificate files before starting.

The one-shot bootstrap service is preferred because nginx needs certificate files before it can start cleanly. The bootstrap service should use the same image as `webui`, mount the same `/data` volume, and exit successfully after ensuring the certificate files exist.

Existing certificates should be migratable by dropping files into the configured certificate path before starting Compose:

```text
/data/certs/frozenfortress.crt
/data/certs/frozenfortress.key
```

The key file should be treated as sensitive data. It should not be logged, committed, or copied into container images.

## Data Layout

Frozen Fortress application state should live under `/data` inside the application container.

Recommended container defaults:

```text
FF_DATABASE_PATH=/data/frozenfortress.db
FF_KEY_DIR=/data/keys
FF_BACKUP_DIRECTORY=/data/backups
FF_REDIS_ADDRESS=redis:6379
FF_OCR_PROVIDER=ollama-tesseract
FF_OCR_OLLAMA_URL=http://ollama:11434
FF_OCR_OLLAMA_MODEL=glm-ocr:q8_0
FF_TLS_CERT_DIR=/data/certs
FF_TLS_CERT_FILE=/data/certs/frozenfortress.crt
FF_TLS_KEY_FILE=/data/certs/frozenfortress.key
```

Recommended persistent mounts:

| Container path | Owner | Purpose |
| --- | --- | --- |
| `/data/frozenfortress.db` | Frozen Fortress | SQLite application database |
| `/data/keys` | Frozen Fortress | Generated session signing/encryption keys |
| `/data/backups` | Frozen Fortress | Database backup files |
| `/data/certs` | Frozen Fortress and nginx | TLS certificate and private key |
| `/root/.ollama` | Ollama | Ollama model cache and manifests |

Redis is not treated as durable migration state. Existing sessions and cached master encryption keys may be lost during migration or restart, so users should expect to sign in again.

## Binary-To-Docker Migration

Existing binary installations usually store data under the OS-specific user data directory. On Linux this is normally:

```text
~/.config/frozenfortress/
```

Typical contents:

| Path | Required | Notes |
| --- | --- | --- |
| `frozenfortress.db` | Yes | Main SQLite database |
| `keys/` | Strongly recommended | Persistent session keys generated by Frozen Fortress |
| `backups/` | Optional | Existing backup generations |
| `certs/` | Optional | Existing TLS certificate and private key for nginx |
| `logs/` | Optional | Existing logs; not required for migration |

Migration checklist:

1. Stop the existing binary-based Frozen Fortress processes.
2. Back up the current data directory before moving or mounting it.
3. Mount or copy `frozenfortress.db` into `/data/frozenfortress.db`.
4. Mount or copy `keys/` into `/data/keys` if generated key files are used.
5. Mount or copy `backups/` into `/data/backups` if existing backups should be retained.
6. If an existing TLS certificate should be used, mount or copy it into `/data/certs/frozenfortress.crt` and `/data/certs/frozenfortress.key` before startup.
7. If no certificate is provided, let the Frozen Fortress certificate bootstrap create a self-signed certificate on first startup.
8. Start the Docker Compose stack.
9. Sign in through the nginx HTTPS endpoint and verify existing users, secrets, documents, and backups are visible.
10. Upload a test image and verify an OCR job is queued and later completed.

Example bind mount layout:

```yaml
services:
  webui:
    volumes:
      - /home/user/.config/frozenfortress/frozenfortress.db:/data/frozenfortress.db
      - /home/user/.config/frozenfortress/keys:/data/keys
      - /home/user/.config/frozenfortress/backups:/data/backups
      - /home/user/.config/frozenfortress/certs:/data/certs
```

For new installations, a single named volume or host directory can be used for `/data` instead of per-path bind mounts.

## Remote Ollama Inference

The default Compose stack should create a local private Ollama service. Deployments with stronger hardware can instead use a remote Ollama instance by overriding:

```text
FF_OCR_OLLAMA_URL=http://gpu-host:11434
```

In remote mode, the bundled `ollama` Compose service can be disabled or omitted. The remote Ollama host owns model storage, model downloads, and GPU acceleration. Frozen Fortress should treat `FF_OCR_OLLAMA_URL` as the service boundary.

Remote Ollama deployments need careful network exposure. Prefer a private network, VPN, firewall allowlist, or reverse proxy rather than exposing Ollama broadly.

## OCR Architecture

Image OCR should move out of the upload request path.

Current behavior blocks upload while image OCR runs. The new behavior should:

1. Validate and store the uploaded file.
2. Generate previews synchronously, as today.
3. For PDFs, keep the existing synchronous PDF text extraction path unchanged.
4. For images, create a durable pending OCR job and return the upload response immediately.
5. Let a background OCR worker process queued image OCR jobs.

The OCR provider chain should be:

1. Ollama GLM OCR, using `glm-ocr:q8_0`.
2. Tesseract fallback, using the existing Tesseract service.
3. NOP provider when OCR is disabled or unavailable by build tag.

Image preprocessing for Ollama should decode PNG/JPEG input, resize it so the larger dimension is at most 640 pixels while preserving aspect ratio, and encode it to a stable image format before sending it as base64 to Ollama.

The Ollama provider should use the local HTTP API with streaming disabled. It can use `/api/chat` or `/api/generate`, with image bytes in the `images` field and a deterministic OCR prompt. Frozen Fortress should request `glm-ocr:q8_0` by default.

## Async OCR Queue

OCR jobs should be stored in SQLite, not Redis or memory. This keeps OCR work durable across restarts and makes migration simple because the queue state travels with the database.

Recommended model:

- A dedicated OCR job table for worker bookkeeping.
- OCR status fields in or near document file metadata for user-facing state and search behavior.

Recommended statuses:

| Status | Meaning |
| --- | --- |
| `pending` | OCR job exists and has not started |
| `processing` | A worker has claimed the job |
| `completed` | Extracted text was persisted |
| `failed` | OCR failed after retry exhaustion |
| `skipped` | OCR was intentionally not run |

The worker should also track attempts, provider used, last error, and timestamps.

## Worker Behavior

The OCR worker should follow the existing backup worker pattern: background loop, context cancellation, periodic checks, and clean shutdown.

Worker loop:

1. Claim a small batch of pending or retryable image OCR jobs atomically.
2. Load the encrypted file data by file ID.
3. Decrypt the file data using the appropriate user data protector context.
4. Preprocess the image to max 640 pixels.
5. Send the OCR request to Ollama.
6. Fall back to Tesseract if Ollama is unavailable, times out, or fails in a usable way.
7. Encrypt and persist extracted text, confidence, provider, and final status.
8. Retry transient failures with bounded attempts.
9. Mark final failures without deleting files.

Recommended controls:

```text
FF_OCR_WORKER_CONCURRENCY=1
FF_OCR_MAX_ATTEMPTS=3
FF_OCR_OLLAMA_TIMEOUT_SECONDS=300
FF_OCR_OLLAMA_PULL_ON_START=true
FF_OCR_OLLAMA_KEEP_ALIVE=5m
```

## Ollama Model Bootstrap

Frozen Fortress should check Ollama availability on startup or worker startup. It can use `/api/version`, `/api/tags`, or both.

If `FF_OCR_OLLAMA_PULL_ON_START=true`, Frozen Fortress should request a pull for `glm-ocr:q8_0` through Ollama. Ollama handles download resume and SHA verification internally.

If Ollama bootstrap fails and Tesseract fallback is available, startup should remain non-fatal. The application should log a clear warning, continue serving requests, and either fall back to Tesseract or retry Ollama later through queued OCR jobs.

## Documentation Updates Needed During Implementation

- Add Docker-first quick start instructions to the main README.
- Add Compose setup instructions for the dedicated Docker network, nginx HTTPS entrypoint, and default local Ollama service.
- Document self-signed certificate bootstrap behavior and drop-in existing certificate migration.
- Document remote Ollama configuration for GPU devices.
- Document binary-to-Docker migration with bind mount examples.
- Document OCR status behavior so users understand queued and processing states.
- Update `.env.example` with Docker, TLS, and OCR/Ollama variables.

## Verification Plan

Before release, verify:

1. The Frozen Fortress Docker image builds from a clean context.
2. The default Compose stack creates and uses the dedicated Frozen Fortress network.
3. The default Compose stack starts `nginx`, `webui`, `redis`, and private `ollama` services.
4. Only nginx publishes host ports by default.
5. nginx terminates HTTPS and proxies successfully to the internal `webui` service.
6. A self-signed certificate is created when no certificate exists.
7. Existing dropped-in certificate files are used unchanged.
8. A partial certificate pair fails clearly and does not overwrite user-provided material.
9. `/data/frozenfortress.db`, `/data/keys`, `/data/backups`, and `/data/certs` persist across restarts.
10. Ollama pulls `glm-ocr:q8_0` into its own persistent volume.
11. Image upload returns before OCR completion.
12. OCR status moves from queued or processing to completed.
13. Extracted OCR text is encrypted at rest and becomes searchable after completion.
14. Stopping or breaking Ollama causes Tesseract fallback or a retryable job state.
15. PDF upload and text extraction behavior remains unchanged.
16. A copied binary-installation data directory works when mounted into `/data`.
17. A remote Ollama endpoint works when `FF_OCR_OLLAMA_URL` points to another host.

## Implementation Notes

- Do not link `go-llama.cpp` into the first Docker-native implementation unless Ollama proves unsuitable.
- Keep Tesseract installed in the Frozen Fortress runtime image for fallback compatibility.
- Keep nginx as the only public entrypoint in the default Compose stack.
- Keep `webui`, Redis, and Ollama private in the dedicated Frozen Fortress Docker network.
- Generate self-signed certificates only when no certificate/key pair exists.
- Never overwrite dropped-in certificate files automatically.
- Avoid GPU-specific Frozen Fortress images for the first release; remote Ollama covers GPU deployments through configuration.
- Preserve all existing environment variable overrides so current installations do not lose configurability.