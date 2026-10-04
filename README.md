<div align="center">
  <img src="https://raw.githubusercontent.com/Yeti47/frozenfortress/master/resources/ff_igloo_logo.png" alt="Frozen Fortress logo" width="300">
  <h1>Frozen Fortress</h1>
</div>

Frozen Fortress is a lightweight, self-hosted manager for your secrets and documents. It runs on your own hardware, keeps your data encrypted at rest, and needs no cloud services.

- **Secrets**: passwords, API keys, and other sensitive values
- **Documents**: files with tags and notes, with text extracted from images and PDFs by OCR in the background
- **Multiple users**: each user's data is encrypted with a key derived from their own password
- **Android companion app**: scan paper documents with your phone straight into Frozen Fortress
- **Web UI and CLI**: everyday use in the browser, administration from the command line

## Quick Start

Docker is the recommended way to run Frozen Fortress. Nothing else needs to be installed: no Go, no Redis.

**You will need**

- Docker 24 or newer with the Compose v2 plugin (Docker Desktop includes it). Check with `docker compose version`.
- Linux, or Windows with WSL 2, on an x86-64 CPU. The published images are `linux/amd64` only.
- About 2 GB of downloads on the first start, most of it the OCR model. OCR runs on the CPU; a [GPU setup](https://github.com/Yeti47/frozenfortress/blob/master/doc/setup-docker.md#gpu-ocr) is optional.

### 1. Download

On the [Releases page](https://github.com/Yeti47/frozenfortress/releases), open the newest release named **Frozen Fortress vX.Y.Z** and download `frozenfortress-docker-vX.Y.Z.zip`. Extract it. The folder contains the `compose.yaml` that defines the whole stack.

### 2. Start

Open a terminal in the extracted folder and run:

```bash
docker compose up -d
```

This starts four containers: nginx (the HTTPS entry point), the web app, Redis (sessions), and Ollama (OCR). The first start takes a few minutes. OCR results appear once the OCR model has finished downloading; `docker compose logs -f ollama` shows its progress.

### 3. Create your account

1. Open <https://localhost:8443>. Your browser will warn that the connection isn't trusted, because Frozen Fortress creates its own certificate on first start. Continue anyway, or [use your own certificate](https://github.com/Yeti47/frozenfortress/blob/master/doc/setup-docker.md#tls-certificates).
2. Click **Request access** and pick a username (3-20 letters, numbers, or underscores) and a password. Passwords need at least 16 characters, including an uppercase letter, a lowercase letter, a digit, and a special character. Only letters, digits, and these special characters are allowed: `@ $ ! % * ? & # _ - . , ; : + § / [ ] ( ) { } =`
3. Save the **recovery code** shown afterwards. It lets you reset a forgotten password without losing your data.
4. New accounts can't sign in until an administrator (you) activates them:

   ```bash
   docker compose exec webui /app/ffcli user activate <username>
   ```

Now sign in with your new account.

> **Prefer the terminal?** `docker compose exec webui /app/ffcli user create <username> '<password>'` creates an account too (it still needs activating). Keep the password in single quotes: otherwise the shell silently rewrites characters such as `$`, and the account gets a different password than the one you typed. The CLI does not show a recovery code, so generate one in your account settings after signing in.

## Day-to-day use

Run these in the folder containing `compose.yaml`:

```bash
docker compose down            # stop (your data is kept)
docker compose up -d           # start again
docker compose ps              # is everything running?
docker compose logs -f webui   # application logs
```

> **Never add `-v` to `docker compose down`.** It deletes the volumes that hold your data.

**Locked out?** After 3 failed sign-ins within 30 minutes an account is locked. Unlock it with `docker compose exec webui /app/ffcli user unlock <username>`. `user list` shows every account and its status.

### Settings

To change a setting, create a file named `.env` next to `compose.yaml` containing only what you want to change, then run `docker compose up -d` again:

```env
FF_HTTPS_PORT=9443
FF_BACKUP_ENABLED=true
```

All options are listed in the [Docker guide](https://github.com/Yeti47/frozenfortress/blob/master/doc/setup-docker.md#configuration).

### Backups

Backups are **off by default**. Turn them on with `FF_BACKUP_ENABLED=true` in `.env` (see above): a backup is then made every 7 days, the last 10 are kept, and you can make one on demand. They are stored inside the Docker volume, so copy them somewhere else:

```bash
docker compose exec webui /app/ffcli backup create
docker compose cp webui:/data/backups ./backups
```

To restore, follow the [restore steps in the Docker guide](https://github.com/Yeti47/frozenfortress/blob/master/doc/setup-docker.md#restoring-from-backup). Copying the file in by hand leaves the database read-only for the app.

### Updating

1. In the old folder, run `docker compose down`.
2. Download and extract the newer release, and copy your `.env` file into the new folder if you have one.
3. In the new folder, run `docker compose up -d`.

Your data lives in Docker volumes and carries over. Don't rely on `docker compose pull` to update: the images in a release are pinned to exact versions, so it can only fetch the same ones again.

## Android companion app

The companion app scans a paper document with your phone's camera and sends it to Frozen Fortress, either as a new document or into an existing document's Files tab. It needs Android 8.0 or newer with Google Play Services, is installed from an APK rather than Google Play, and is currently a pre-release. Download the APK from the [Releases page](https://github.com/Yeti47/frozenfortress/releases) (releases tagged `android-v...`).

Your phone must be able to reach your Frozen Fortress instance, which by default only accepts connections from the machine it runs on. See the [companion app guide](https://github.com/Yeti47/frozenfortress/blob/master/doc/setup-companion-app.md) for setup.

## Documentation

| Guide | Contents |
|-------|----------|
| [Docker guide](https://github.com/Yeti47/frozenfortress/blob/master/doc/setup-docker.md) | TLS certificates, all configuration options, CLI reference, backup and restore |
| [Companion app guide](https://github.com/Yeti47/frozenfortress/blob/master/doc/setup-companion-app.md) | Installing the Android app and scanning documents |
| [Binary setup guide](https://github.com/Yeti47/frozenfortress/blob/master/doc/setup-binary.md) | Legacy: running without Docker |
| [Binary to Docker migration](https://github.com/Yeti47/frozenfortress/blob/master/doc/migration-binary-to-docker.md) | Moving an existing binary installation to Docker |

## Security

- **Encrypted at rest.** Secrets, and document titles, descriptions, issuers, files, extracted text, and notes, are encrypted with a key derived from the owner's password, so a copy of the database or a backup can't be read without it. Tag names are not encrypted.
- **Your key stays out of the session store.** While you are signed in, Redis holds your key only in encrypted form; the key that unlocks it lives in a cookie in your browser, so Redis alone reveals nothing. As with any app that decrypts on the server, the machine running Frozen Fortress should be one you trust.
- **HTTPS only.** nginx is the only exposed container and listens on `127.0.0.1` by default. The web app, Redis, and Ollama sit on an internal Docker network.
- **Sign-in protection.** Accounts are locked after repeated failed sign-ins, and new accounts stay inactive until an administrator activates them.

## Contributing

Issues and pull requests are welcome. Frozen Fortress is built with Go, SQLite, Redis, Gin, and Ollama (GLM-OCR).

To build from source, install the development dependencies (`./install-dev-deps-debian.sh` or `./install-dev-deps-fedora.sh`) and run `./build-all.sh`. For a complete local stack (web UI, Redis and OCR) with one command, run `./dev.sh`; see the [dev stack guide](https://github.com/Yeti47/frozenfortress/blob/master/doc/dev-stack.md). To build the Docker images from a clone instead of downloading a release, run `docker compose up -d --build`.

Work on the upcoming v2 (Go JSON API plus Angular UI) happens on the `feature/v2` branch; see the [v2 architecture and conventions](doc/v2-architecture.md) before contributing there.

## License

MIT. See [LICENSE](LICENSE.md).

------

© 2026 Yetibyte
