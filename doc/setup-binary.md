# Frozen Fortress — Binary Setup Guide (Legacy)

> **Note**: The recommended deployment method is [Docker](setup-docker.md). This guide covers the legacy binary installation for users who cannot use Docker or who prefer to run Frozen Fortress directly on a host system.

---

## Prerequisites

- **Go 1.24.3** or higher
- **Redis** server (required for session management)
- **Tesseract OCR** (optional, for local OCR fallback)
- A reverse proxy such as **nginx** (recommended for HTTPS)
- Linux (Debian/Ubuntu or Fedora supported by the install scripts)

---

## Installing Development Dependencies

Helper scripts are provided to install all dependencies automatically.

### Debian / Ubuntu

```bash
./install-dev-deps-debian.sh
```

### Fedora

```bash
./install-dev-deps-fedora.sh
```

Both scripts install:
- Go 1.24.3
- Redis server
- Tesseract OCR with language packs
- All required development tools

---

## Building

### Build All Components

```bash
./build-all.sh
```

### Build Individual Components

```bash
# Web UI only
./build-webui.sh

# CLI only
./build-cli.sh
```

### Build Options

```bash
# Include debug symbols
./build-all.sh --debug

# Build without Tesseract OCR support
./build-all.sh --notesseract

# Both options
./build-all.sh --debug --notesseract
```

After a successful build, binaries are placed in `bin/`:
- `bin/ffwebui` — Web server
- `bin/ffcli` — Command-line interface

---

## Initial Configuration

1. **Run the interactive setup wizard**:
   ```bash
   ./bin/ffcli setup
   ```
   This prompts for configuration values and writes a shell script with the required `export` statements.

2. **Apply the generated configuration**:
   ```bash
   # Linux / macOS
   source frozenfortress-config.sh
   ```

3. **Create your first user**:
   ```bash
   ./bin/ffcli user create <username> <password>
   ./bin/ffcli user activate <username>
   ```

4. **Start the web server**:
   ```bash
   ./run-webui.sh
   # or directly:
   ./bin/ffwebui
   ```
   The web interface is available at `http://localhost:8080` (or the port you configured).

To view the active configuration at any time:
```bash
./bin/ffcli setup --read
```

---

## Environment Variables

All configuration is provided via environment variables. Set them before starting `ffwebui`, either by sourcing the generated config script or exporting them in your shell / service unit.

| Variable | Description | Default |
|----------|-------------|---------|
| `FF_DATABASE_PATH` | Path to SQLite database | `~/.config/frozenfortress/frozenfortress.db` |
| `FF_MAX_SIGN_IN_ATTEMPTS` | Maximum sign-in attempts before account lockout | `3` |
| `FF_SIGN_IN_ATTEMPT_WINDOW` | Time window in minutes for counting sign-in attempts | `30` |
| `FF_REDIS_ADDRESS` | Redis server address | `localhost:6379` |
| `FF_REDIS_USER` | Redis username (leave empty if not required) | `""` |
| `FF_REDIS_PASSWORD` | Redis password (leave empty if not required) | `""` |
| `FF_REDIS_SIZE` | Redis connection pool size | `10` |
| `FF_REDIS_NETWORK` | Redis network type (`tcp`/`unix`) | `tcp` |
| `FF_SIGNING_KEY` | Session signing key (leave empty to auto-generate) | `""` |
| `FF_ENCRYPTION_KEY` | Session encryption key (leave empty to auto-generate) | `""` |
| `FF_KEY_DIR` | Directory to store persistent key files (empty = OS default) | `""` |
| `FF_WEB_UI_PORT` | Web UI server port | `8080` |
| `FF_LOG_LEVEL` | Log level (`Debug`, `Info`, `Warn`, `Error`) | `Info` |
| `FF_BACKUP_ENABLED` | Enable automatic backups | `false` |
| `FF_BACKUP_INTERVAL_DAYS` | Backup interval in days (`0` = disabled) | `7` |
| `FF_BACKUP_DIRECTORY` | Directory where backup files are stored | `~/.config/frozenfortress/backups` |
| `FF_BACKUP_MAX_GENERATIONS` | Maximum number of backup files to keep | `10` |
| `FF_OCR_ENABLED` | Enable OCR functionality | `true` |
| `FF_OCR_PROVIDER` | OCR provider: `ollama-tesseract`, `ollama`, `tesseract`, `nop` | `ollama-tesseract` |
| `FF_OCR_LANGUAGES` | Tesseract languages (comma-separated, e.g. `eng,deu`) | `eng` |
| `FF_OCR_OLLAMA_URL` | Ollama API base URL | `http://ollama:11434` |
| `FF_OCR_OLLAMA_MODEL` | Ollama OCR model | `glm-ocr:q8_0` |
| `FF_OCR_OLLAMA_KEEP_ALIVE` | Ollama model keep-alive value | `5m` |
| `FF_OCR_OLLAMA_TIMEOUT_SECONDS` | Ollama OCR request timeout in seconds | `300` |
| `FF_OCR_IMAGE_MAX_DIMENSION` | Maximum image width/height sent to Ollama | `640` |
| `FF_OCR_MAX_ATTEMPTS` | Maximum best-effort OCR attempts per upload | `3` |
| `FF_OCR_RETRY_INITIAL_BACKOFF_SECONDS` | Initial async OCR retry backoff | `2` |
| `FF_OCR_RETRY_MAX_BACKOFF_SECONDS` | Maximum async OCR retry backoff | `30` |

**Key directory defaults** (when `FF_KEY_DIR` is empty):
- **Linux**: `$XDG_CONFIG_HOME/frozenfortress` or `~/.config/frozenfortress`

---

## HTTPS with nginx (Recommended)

The Go application serves plain HTTP only. Use nginx as a reverse proxy for HTTPS termination:

```nginx
server {
    listen 443 ssl;
    server_name your-domain.local;

    ssl_certificate     /path/to/frozenfortress.crt;
    ssl_certificate_key /path/to/frozenfortress.key;

    location / {
        proxy_pass         http://localhost:8080;
        proxy_set_header   Host $host;
        proxy_set_header   X-Real-IP $remote_addr;
        proxy_set_header   X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
    }
}
```

---

## CLI Administration

```bash
# User management
./bin/ffcli user create <username> <password>
./bin/ffcli user activate <username>
./bin/ffcli user deactivate <username>
./bin/ffcli user lock <username>
./bin/ffcli user unlock <username>
./bin/ffcli user list
./bin/ffcli user delete <username>

# Backup management
./bin/ffcli backup create
./bin/ffcli backup list
./bin/ffcli backup cleanup
./bin/ffcli backup delete <backup-id>

# View current configuration
./bin/ffcli setup --read
```

Use `--verbose` for detailed output or `--help` on any command for usage information.

### CLI Encryption Boundaries

Even with CLI access, **encrypted user data remains protected** by user-specific encryption keys derived from each user's password. Administrators can manage accounts but cannot read any user's encrypted content without that user's password.

---

## Release Packages

Frozen Fortress ships pre-built release archives for Linux that include both binaries and an automated setup script.

### Installing from a Release Archive

1. Download the release archive (e.g., `frozenfortress-release-linux-amd64-v1.0.0.zip`)
2. Extract: `unzip frozenfortress-release-linux-amd64-v1.0.0.zip`
3. Enter the directory: `cd frozenfortress-release-linux-amd64-v1.0.0/`
4. Run the setup script: `./ff-setup.sh`
5. Follow the interactive prompts

The `ff-setup.sh` script handles dependency installation, optional nginx configuration with self-signed certificates, and binary placement automatically.

### Creating a Release (Maintainers)

```bash
# Standard release
./release-linux.sh --arch amd64 --version 1.0.0

# Without Tesseract
./release-linux.sh --arch amd64 --version 1.0.0 --notesseract
```

Supported architectures: `amd64`, `386`, `arm64`, `arm`

---

## Additional Scripts

| Script            | Purpose                             |
|-------------------|-------------------------------------|
| `./run-webui.sh`  | Build and start the web UI          |
| `./stop-webui.sh` | Stop running web UI processes       |
| `./clean.sh`      | Remove build artifacts              |

---

## Migrating to Docker

If you want to move from a binary installation to the Docker-native stack, see the [Binary to Docker Migration Guide](migration-binary-to-docker.md).
