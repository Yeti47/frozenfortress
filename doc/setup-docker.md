# Frozen Fortress — Docker Setup Guide

This guide covers everything you need to deploy Frozen Fortress using Docker and Docker Compose, which is the recommended deployment method.

## Prerequisites

- **Docker** 24+ and **Docker Compose** v2
- A Linux host (or WSL2 on Windows) on an x86-64 CPU. The published images are `linux/amd64` only.

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

1. **Download the release**: on the [Releases page](https://github.com/Yeti47/frozenfortress/releases), download `frozenfortress-docker-vX.Y.Z.zip` from the newest release named *Frozen Fortress vX.Y.Z* and extract it.

2. **Start the stack** from the extracted folder:
   ```bash
   docker compose up -d
   ```
   The first start downloads about 2 GB, most of it the OCR model. OCR runs on the CPU; see [GPU OCR](#gpu-ocr) to use a GPU instead.

3. **Open the web UI** at `https://localhost:8443`. Accept the self-signed certificate warning on first use (see [TLS Certificates](#tls-certificates) for how to use your own certificate).

4. **Create your account**: click **Request access**, choose a username and password, and save the recovery code shown afterwards. Then activate the account:
   ```bash
   docker compose exec webui /app/ffcli user activate <username>
   ```

---

## Data Storage

State lives in three Docker volumes:

| Volume | Mounted at | Contents |
|--------|------------|----------|
| `frozenfortress_frozenfortress-data` | `/data` in `webui` | `frozenfortress.db` (SQLite database), `keys/` (session signing and encryption keys), `backups/` |
| `frozenfortress_frozenfortress-certs` | `/data/certs` in `nginx` | TLS certificate and private key |
| `frozenfortress_frozenfortress-ollama` | `/models` in `ollama` | OCR model cache, so `glm-ocr:q8_0` is not downloaded again on every start |

`docker compose down` keeps all three. `docker compose down -v` deletes them, including your database.

---

## TLS Certificates

nginx handles TLS termination. It reads the certificate from `/data/certs/` inside the `nginx` container (the `frozenfortress-certs` volume):

- **No certificate present**: a self-signed certificate is generated automatically on first startup.
- **Both files present** (`frozenfortress.crt` and `frozenfortress.key`): they are used as-is.
- **Only one file present**: startup fails deliberately to prevent accidental use of a partial pair.

The generated certificate is valid for `localhost` and `frozenfortress.local`. To reach Frozen Fortress under another name or IP address, set `FF_TLS_HOSTS` (comma-separated names and IPs) and, optionally, `FF_TLS_COMMON_NAME` in `.env`. An existing certificate is **not** regenerated when these change, so delete it first:

```bash
docker compose stop nginx
docker compose run --rm --no-deps --user root --entrypoint sh nginx -c 'rm -f /data/certs/frozenfortress.crt /data/certs/frozenfortress.key'
docker compose up -d nginx
```

### Using Your Own Certificate

Put `frozenfortress.crt` (full chain) and `frozenfortress.key` in a folder, for example `./certs`. There are two ways to give them to nginx.

**Copy them into the volume** (works everywhere, including Docker Desktop):

```bash
docker compose stop nginx
docker compose run --rm --no-deps --user root -v "$PWD/certs:/certs:ro" --entrypoint sh nginx -c 'cp /certs/frozenfortress.crt /certs/frozenfortress.key /data/certs/ && chown nginx:nginx /data/certs/frozenfortress.* && chmod 600 /data/certs/frozenfortress.key'
docker compose up -d nginx
```

Don't use `docker compose cp` for this: the files keep your host user's ownership, nginx runs as a different user (uid 101) and can't read the `600` key, so it fails to start with `Permission denied`.

**Or mount the folder directly**, which makes renewing easier: replace the files and run `docker compose restart nginx`. In `compose.yaml`, change nginx's `frozenfortress-certs:/data/certs` volume to:

```yaml
    volumes:
      - ./certs:/data/certs:ro
```

The folder is read-only for nginx, so it cannot generate a certificate there: both files must already exist. The key must also be readable by uid 101, so hand it over and keep it private:

```bash
sudo chown 101:101 certs/frozenfortress.crt certs/frozenfortress.key
sudo chmod 600 certs/frozenfortress.key
```

Making the key `644` also works, but then every user on the host can read it. Then run `docker compose up -d nginx`.

Keep the private key safe and never commit it.

---

## Configuration

Frozen Fortress is configured via environment variables. Any setting below can be changed in a `.env` file next to `compose.yaml` (Docker Compose picks it up automatically). Put in only what you want to change: everything else keeps its default.

### Example `.env` file

```env
# Port nginx binds on the host (default: 8443)
FF_HTTPS_PORT=8443

# Automatic backups (default: off)
# FF_BACKUP_ENABLED=true

# Names and IPs the generated TLS certificate is valid for
# FF_TLS_HOSTS=localhost,frozenfortress.local,192.168.1.50
```

To apply changes, run `docker compose up -d` again.

### All Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `FF_DATABASE_PATH` | Path to SQLite database | `/data/frozenfortress.db` |
| `FF_MAX_SIGN_IN_ATTEMPTS` | Maximum sign-in attempts before account lockout | `3` |
| `FF_SIGN_IN_ATTEMPT_WINDOW` | Time window in minutes for counting sign-in attempts | `30` |
| `FF_SESSION_MAX_AGE_DAYS` | How long a sign-in lasts, in days | `30` |
| `FF_REDIS_ADDRESS` | Redis server address | `redis:6379` |
| `FF_REDIS_USER` | Redis username (leave empty if not required) | `""` |
| `FF_REDIS_PASSWORD` | Redis password (leave empty if not required) | `""` |
| `FF_REDIS_SIZE` | Redis connection pool size | `10` |
| `FF_REDIS_NETWORK` | Redis network type (`tcp`/`unix`) | `tcp` |
| `FF_SIGNING_KEY` | Session signing key (leave empty to auto-generate) | `""` |
| `FF_ENCRYPTION_KEY` | Session encryption key (leave empty to auto-generate) | `""` |
| `FF_KEY_DIR` | Directory to store persistent key files | `/data/keys` |
| `FF_WEB_UI_PORT` | Internal web UI port. Fixed in Docker, because nginx forwards to port `8080`; `.env` cannot change it. | `8080` |
| `FF_LOG_LEVEL` | Log level (`Debug`, `Info`, `Warn`, `Error`) | `Info` |
| `FF_BACKUP_ENABLED` | Enable automatic backups (also required for `ffcli backup create`) | `false` |
| `FF_BACKUP_INTERVAL_DAYS` | Backup interval in days (`0` = disabled) | `7` |
| `FF_BACKUP_DIRECTORY` | Directory where backup files are stored | `/data/backups` |
| `FF_BACKUP_MAX_GENERATIONS` | Maximum number of backup files to keep | `10` |
| `FF_OCR_ENABLED` | Enable OCR functionality | `true` |
| `FF_OCR_PROVIDER` | OCR provider: `ollama-tesseract`, `ollama`, `tesseract`, `nop`. The Docker images are built without Tesseract, so `ollama-tesseract` behaves like `ollama` and `tesseract` is not available. | `ollama-tesseract` |
| `FF_OCR_LANGUAGES` | Tesseract languages (comma-separated, e.g. `eng,deu`). No effect in the Docker images. | `eng` |
| `FF_OCR_OLLAMA_URL` | Ollama API base URL | `http://ollama:11434` |
| `FF_OCR_OLLAMA_MODEL` | Ollama OCR model | `glm-ocr:q8_0` |
| `FF_OCR_OLLAMA_KEEP_ALIVE` | Ollama model keep-alive value | `5m` |
| `FF_OCR_OLLAMA_TIMEOUT_SECONDS` | Ollama OCR request timeout in seconds | `300` |
| `FF_OCR_IMAGE_MAX_DIMENSION` | Maximum image width/height sent to Ollama | `640` |
| `FF_OCR_MAX_ATTEMPTS` | Maximum best-effort OCR attempts per upload | `3` |
| `FF_OCR_RETRY_INITIAL_BACKOFF_SECONDS` | Initial async OCR retry backoff | `2` |
| `FF_OCR_RETRY_MAX_BACKOFF_SECONDS` | Maximum async OCR retry backoff | `30` |
| `FF_UPDATE_CHECK_ENABLED` | Check GitHub once a day for a newer release and show a "new version available" link to signed-in users. Makes one anonymous request to `api.github.com` per day; set to `false` to turn it off. Doesn't affect `ffcli version --check`, which only runs when you call it. | `true` |
| `FF_HTTPS_PORT` | Host port nginx binds for HTTPS | `8443` |
| `FF_TLS_COMMON_NAME` | Common name of the generated TLS certificate | `frozenfortress.local` |
| `FF_TLS_HOSTS` | Names and IPs the generated TLS certificate is valid for (comma-separated) | `localhost,frozenfortress.local` |

---

## GPU OCR

OCR runs on the CPU by default. The default `ollama` image contains only Ollama's CPU backends, about 130 MB. The GPU runtimes (CUDA, MLX, Vulkan) would add about 6 GB, so they come in a separate image.

To run OCR on a GPU:

1. Give the container access to the GPU. The release folder contains `compose.gpu.yaml`, which switches to the GPU image. For an NVIDIA GPU, install the [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html) on the host and uncomment the `deploy:` block in `compose.gpu.yaml`. Other GPUs need their own setup; see Docker's [GPU support](https://docs.docker.com/compose/how-tos/gpu-support/) guide.
2. Add this line to `.env` next to `compose.yaml` (create the file if it does not exist):
   ```env
   COMPOSE_FILE=compose.yaml:compose.gpu.yaml
   ```
   Every `docker compose` command in this folder now uses both files, so you do not need to pass them yourself. Alternatively, leave `.env` alone and pass both files to every command: `docker compose -f compose.yaml -f compose.gpu.yaml up -d`.
3. Run `docker compose up -d`. `docker compose logs ollama` should now report a CUDA (or other GPU) device under `inference compute` instead of `cpu`.

Without GPU access, the GPU image still runs OCR on the CPU, only with a much larger download.

To switch back, remove the line and run `docker compose up -d` again. The downloaded model is kept in the `frozenfortress-ollama` volume in both cases. `docker image prune` then removes the unused GPU image. This also applies after upgrading from v1.3.0 or earlier, which always used the large image.

---

## Using an External Ollama Instance

If you already have Ollama running elsewhere (e.g., a dedicated GPU workstation), you can use it instead of the bundled container.

1. Make sure the model is available there. The bundled container downloads the model itself, but Frozen Fortress does not, so on your Ollama server run `ollama pull glm-ocr:q8_0` (or the model set in `FF_OCR_OLLAMA_MODEL`).
2. Point Frozen Fortress at it in `.env`:
   ```env
   FF_OCR_OLLAMA_URL=http://gpu-host:11434
   ```
3. Optionally stop the bundled container from starting. Setting the URL alone does not: `ollama` still starts and downloads its image and model. In `compose.yaml`, delete the `ollama` service and the `- ollama` line under `webui` → `depends_on` (removing only the service makes Compose fail with `depends on undefined service "ollama"`). The unused `frozenfortress-ollama` volume at the bottom can go too.
4. Run `docker compose up -d`.

---

## Exposing on a Local Network or the Internet

By default nginx only listens on `127.0.0.1:8443`, accessible from the local machine only. To expose the service on your LAN, change the port binding in `compose.yaml`. The container port is `8443`:

```yaml
ports:
  - "0.0.0.0:${FF_HTTPS_PORT:-8443}:8443"
```

Then run `docker compose up -d`. The generated certificate only covers `localhost` and `frozenfortress.local`, so set `FF_TLS_HOSTS` to the name or IP address you will use and regenerate it (see [TLS Certificates](#tls-certificates)). This is also what the Android companion app needs, since your phone must reach the server.

> **Security note**: If exposing beyond localhost, use a valid TLS certificate, apply firewall rules, and review the [Security Considerations](#security-considerations) section below.

---

## CLI Administration

Administrative tasks are performed via the `ffcli` binary inside the running `webui` container:

```bash
# User management
docker compose exec webui /app/ffcli user create <username> '<password>'
docker compose exec webui /app/ffcli user activate <username>
docker compose exec webui /app/ffcli user deactivate <username>
docker compose exec webui /app/ffcli user lock <username>
docker compose exec webui /app/ffcli user unlock <username>
docker compose exec webui /app/ffcli user list
docker compose exec webui /app/ffcli user delete <username>

# Backup management (requires FF_BACKUP_ENABLED=true)
docker compose exec webui /app/ffcli backup create
docker compose exec webui /app/ffcli backup list
docker compose exec webui /app/ffcli backup cleanup

# View current configuration
docker compose exec webui /app/ffcli setup --read

# Show the version and check GitHub for a newer release
docker compose exec webui /app/ffcli version --check
```

Keep the password in single quotes: otherwise the shell silently rewrites characters such as `$`, and the account gets a different password than the one you typed. Accounts created this way don't get a recovery code shown; generate one in the account settings after signing in.

### CLI Encryption Boundaries

The CLI manages accounts but cannot read any user's encrypted content: decrypting it needs the user's password. This protects data at rest (the database and backups).

While a user is signed in, Redis holds their key only in encrypted form. The key that decrypts it lives in a cookie in the user's browser, so reading Redis alone (for example with a Redis GUI or a dump) reveals nothing. As with any app that decrypts on the server, the host running Frozen Fortress should be one you trust.

---

## Backup and Restore

### Creating a Backup

Backups are off by default. Set `FF_BACKUP_ENABLED=true` in `.env` and run `docker compose up -d`; `ffcli backup create` fails with "backups are disabled in configuration" until you do. Then:

```bash
docker compose exec webui /app/ffcli backup create
```

Backups are written to `/data/backups/` inside the `webui` container, which is part of the persisted volume. That is the same volume as the database, so also copy them somewhere else:

```bash
docker compose cp webui:/data/backups ./backups
```

### Restoring from Backup

Restoring replaces the whole database: anything created after the backup is lost. Stop the web app, replace `/data/frozenfortress.db`, and fix its ownership in one step. The app runs as an unprivileged user, so a database file owned by anyone else fails on every write with `attempt to write a readonly database`. Don't use `docker compose cp` for this, for that reason.

To restore a backup file from your host:

```bash
docker compose stop webui
docker compose run --rm --no-deps --user root -v "$PWD/backup.db:/restore.db:ro" --entrypoint sh webui -c 'cp /restore.db /data/frozenfortress.db && chown nonroot:nonroot /data/frozenfortress.db'
docker compose start webui
```

To roll back to a backup that is still inside the volume, use its path (see `ffcli backup list`) instead:

```bash
docker compose stop webui
docker compose run --rm --no-deps --user root --entrypoint sh webui -c 'cp /data/backups/<backup-file>.db /data/frozenfortress.db && chown nonroot:nonroot /data/frozenfortress.db'
docker compose start webui
```

---

## Stopping and Updating

```bash
# Stop the stack (your data is kept; never add -v, which deletes it)
docker compose down
```

To update, run `docker compose down` in the old folder, extract the newer release, copy your `.env` file into the new folder if you have one, and run `docker compose up -d` there. Your data is in Docker volumes and carries over. `docker compose pull` does not update a release: its images are pinned to exact versions, so it can only fetch the same ones again.

---

## Security Considerations

- **HTTPS only**: the nginx container enforces HTTPS. The `webui` service is not reachable outside the Docker network.
- **Private key protection**: `/data/certs/frozenfortress.key` is sensitive. Do not log, commit, or copy it into images.
- **Non-root containers**: all containers run as non-root users.
- **Network isolation**: Redis and Ollama are not exposed to the host by default.
- **Session key rotation**: if `FF_SIGNING_KEY` and `FF_ENCRYPTION_KEY` are left empty, keys are auto-generated and persisted in `/data/keys/`. Deleting this directory invalidates all active sessions.
