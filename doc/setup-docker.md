# Frozen Fortress — Docker Setup Guide

This guide covers everything you need to deploy Frozen Fortress using Docker and Docker Compose, which is the recommended deployment method.

## Prerequisites

- **Docker** 24+ and **Docker Compose** v2
- A Linux host (or WSL2 on Windows)

No Go installation, Redis, or Tesseract setup is required — the Compose stack includes everything.

---

## Architecture Overview

The default Compose stack starts four services on a dedicated `frozenfortress` Docker network:

| Service  | Purpose                                          | Default exposure              |
|----------|--------------------------------------------------|-------------------------------|
| `nginx`  | HTTPS entrypoint, SSL termination, reverse proxy | `127.0.0.1:8443` (host)       |
| `webui`  | Frozen Fortress web application                  | Internal Docker network only  |
| `redis`  | Session store                                    | Internal Docker network only  |
| `ollama` | GLM OCR inference service                        | Internal Docker network only  |

Only nginx is exposed to the host. All other services communicate on the internal network.

---

## Quick Start

1. **Clone the repository** (or download the source):
   ```bash
   git clone https://github.com/Yeti47/frozenfortress.git
   cd frozenfortress
   ```

2. **Start the stack**:
   ```bash
   docker compose up -d
   ```

3. **Create your first user** using the CLI container:
   ```bash
   docker compose exec webui /app/ffcli user create <username> <password>
   docker compose exec webui /app/ffcli user activate <username>
   ```

4. **Open the web UI**:

   Navigate to `https://127.0.0.1:8443`. Accept the self-signed certificate warning on first use (see [TLS Certificates](#tls-certificates) for how to use your own certificate).

---

## Data Storage

All application state lives under a single Docker volume mounted at `/data` inside the `webui` container:

| Path inside container        | Purpose                              |
|------------------------------|--------------------------------------|
| `/data/frozenfortress.db`    | SQLite database                      |
| `/data/keys/`                | Session signing and encryption keys  |
| `/data/backups/`             | Automatic and manual backups         |
| `/data/certs/`               | TLS certificate and private key      |

The Ollama model cache is stored in a separate volume so `glm-ocr:q8_0` is not re-downloaded on every restart.

---

## TLS Certificates

nginx handles TLS termination. Certificates are read from `/data/certs/` (mapped from the host volume):

- **No certificate present**: a self-signed certificate is generated automatically on first startup.
- **Both files present** (`frozenfortress.crt` and `frozenfortress.key`): they are used as-is.
- **Only one file present**: startup fails deliberately to prevent accidental use of a partial pair.

### Using Your Own Certificate

Place your certificate and private key in the data volume's `certs/` directory before starting the stack:

```
/data/certs/frozenfortress.crt   ← full chain certificate
/data/certs/frozenfortress.key   ← private key (keep secure, never commit)
```

With Docker the default host-side volume path is typically a Docker-managed volume, or you can bind-mount a host directory. Refer to `compose.yaml` for the exact volume configuration.

---

## Configuration

Frozen Fortress is configured via environment variables. Set them in a `.env` file in the project root (Docker Compose picks it up automatically) or pass them directly to `docker compose up`.

### Example `.env` file

```env
# Port nginx binds on the host (default: 8443)
FF_HTTPS_PORT=8443

# Use an external Ollama instance instead of the bundled container
# FF_OCR_OLLAMA_URL=http://gpu-host:11434
```

### All Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `FF_DATABASE_PATH` | Path to SQLite database | `/data/frozenfortress.db` |
| `FF_MAX_SIGN_IN_ATTEMPTS` | Maximum sign-in attempts before account lockout | `3` |
| `FF_SIGN_IN_ATTEMPT_WINDOW` | Time window in minutes for counting sign-in attempts | `30` |
| `FF_REDIS_ADDRESS` | Redis server address | `redis:6379` |
| `FF_REDIS_USER` | Redis username (leave empty if not required) | `""` |
| `FF_REDIS_PASSWORD` | Redis password (leave empty if not required) | `""` |
| `FF_REDIS_SIZE` | Redis connection pool size | `10` |
| `FF_REDIS_NETWORK` | Redis network type (`tcp`/`unix`) | `tcp` |
| `FF_SIGNING_KEY` | Session signing key (leave empty to auto-generate) | `""` |
| `FF_ENCRYPTION_KEY` | Session encryption key (leave empty to auto-generate) | `""` |
| `FF_KEY_DIR` | Directory to store persistent key files | `/data/keys` |
| `FF_WEB_UI_PORT` | Internal web UI port | `8080` |
| `FF_LOG_LEVEL` | Log level (`Debug`, `Info`, `Warn`, `Error`) | `Info` |
| `FF_BACKUP_ENABLED` | Enable automatic backups | `false` |
| `FF_BACKUP_INTERVAL_DAYS` | Backup interval in days (`0` = disabled) | `7` |
| `FF_BACKUP_DIRECTORY` | Directory where backup files are stored | `/data/backups` |
| `FF_BACKUP_MAX_GENERATIONS` | Maximum number of backup files to keep | `10` |
| `FF_OCR_ENABLED` | Enable OCR functionality | `true` |
| `FF_OCR_PROVIDER` | OCR provider: `ollama-tesseract`, `ollama`, `tesseract`, `nop` | `ollama` |
| `FF_OCR_LANGUAGES` | Tesseract languages (comma-separated, e.g. `eng,deu`) | `eng` |
| `FF_OCR_OLLAMA_URL` | Ollama API base URL | `http://ollama:11434` |
| `FF_OCR_OLLAMA_MODEL` | Ollama OCR model | `glm-ocr:q8_0` |
| `FF_OCR_OLLAMA_KEEP_ALIVE` | Ollama model keep-alive value | `5m` |
| `FF_OCR_OLLAMA_TIMEOUT_SECONDS` | Ollama OCR request timeout in seconds | `300` |
| `FF_OCR_IMAGE_MAX_DIMENSION` | Maximum image width/height sent to Ollama | `640` |
| `FF_OCR_MAX_ATTEMPTS` | Maximum best-effort OCR attempts per upload | `3` |
| `FF_OCR_RETRY_INITIAL_BACKOFF_SECONDS` | Initial async OCR retry backoff | `2` |
| `FF_OCR_RETRY_MAX_BACKOFF_SECONDS` | Maximum async OCR retry backoff | `30` |
| `FF_HTTPS_PORT` | Host port nginx binds for HTTPS | `8443` |

---

## Using an External Ollama Instance

If you already have Ollama running elsewhere (e.g., a dedicated GPU workstation), you can skip the bundled Ollama container:

```bash
FF_OCR_OLLAMA_URL=http://gpu-host:11434 docker compose up -d
```

Or set it permanently in your `.env` file and disable the `ollama` service in `compose.yaml` by removing it or adding a profile.

---

## Exposing on a Local Network or the Internet

By default nginx only listens on `127.0.0.1:8443`, accessible from the local machine only. To expose the service on your LAN, change the port binding in `compose.yaml`:

```yaml
ports:
  - "0.0.0.0:8443:443"
```

> **Security note**: If exposing beyond localhost, use a valid TLS certificate, apply firewall rules, and review the [Security Considerations](#security-considerations) section below.

---

## CLI Administration

Administrative tasks are performed via the `ffcli` binary inside the running `webui` container:

```bash
# User management
docker compose exec webui /app/ffcli user create <username> <password>
docker compose exec webui /app/ffcli user activate <username>
docker compose exec webui /app/ffcli user deactivate <username>
docker compose exec webui /app/ffcli user lock <username>
docker compose exec webui /app/ffcli user unlock <username>
docker compose exec webui /app/ffcli user list
docker compose exec webui /app/ffcli user delete <username>

# Backup management
docker compose exec webui /app/ffcli backup create
docker compose exec webui /app/ffcli backup list
docker compose exec webui /app/ffcli backup cleanup

# View current configuration
docker compose exec webui /app/ffcli setup --read
```

### CLI Encryption Boundaries

Even with CLI access, **encrypted user data (secrets, documents) remains protected** by user-specific encryption keys derived from each user's password. An administrator can manage accounts but cannot read any user's encrypted content without knowing that user's password.

---

## Backup and Restore

### Creating a Backup

```bash
docker compose exec webui /app/ffcli backup create
```

Backups are written to `/data/backups/` inside the container, which is part of the persisted volume.

### Restoring from Backup

1. Stop the stack: `docker compose down`
2. Replace `/data/frozenfortress.db` with the desired backup file.
3. Restart: `docker compose up -d`

---

## Stopping and Updating

```bash
# Stop the stack
docker compose down

# Pull updated images and restart
docker compose pull && docker compose up -d
```

---

## Security Considerations

- **HTTPS only**: the nginx container enforces HTTPS. The `webui` service is not reachable outside the Docker network.
- **Private key protection**: `/data/certs/frozenfortress.key` is sensitive. Do not log, commit, or copy it into images.
- **Non-root containers**: all containers run as non-root users.
- **Network isolation**: Redis and Ollama are not exposed to the host by default.
- **Session key rotation**: if `FF_SIGNING_KEY` and `FF_ENCRYPTION_KEY` are left empty, keys are auto-generated and persisted in `/data/keys/`. Deleting this directory invalidates all active sessions.
