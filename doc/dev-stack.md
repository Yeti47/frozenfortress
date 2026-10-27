# Frozen Fortress — Local Dev Stack

This guide is for contributors. It starts a complete, testable stack with one command, using an [Aspire](https://aspire.dev) AppHost written in TypeScript (`dev/aspire/apphost.mts`). The web UI runs natively with `go run`, so a code change only needs a restart, not an image rebuild. Redis and Ollama run as containers.

The dev stack is for development only. Releases and deployments still use `compose.yaml`.

## Prerequisites

- **Go** (the version in `go.mod`) and a C compiler, because SQLite needs cgo
- **Node.js** 22 or 24
- **Docker** or **Podman**. For Podman, set `ASPIRE_CONTAINER_RUNTIME=podman`.
- **Aspire CLI** 13.6 or newer. Install it as described at [aspire.dev](https://aspire.dev/get-started/install-cli/), and update it with `aspire update --self`. Check your version with `aspire --version`.

## Start

From the repository root:

```bash
aspire run --apphost dev/aspire
```

Or run `aspire run` inside `dev/aspire/`.

The first run installs the AppHost's npm packages, generates the Aspire SDK into `dev/aspire/.aspire/` and builds the Ollama image. The terminal shows a link to the **Aspire dashboard**, where you can see each resource's state, logs and environment, and stop or restart it.

| Resource | What it is | Address |
|----------|------------|---------|
| `webui`  | The web app, run with `go run` from `webui/` | <http://localhost:18080> |
| `redis`  | Session store, the same pinned image as `compose.yaml` | Random localhost port |
| `ollama` | OCR, the CPU target of `docker/ollama/Dockerfile` | Random localhost port |

On the first start Ollama downloads the OCR model (about 1.6 GB, shown in the `ollama` logs). OCR results appear once the download has finished. The model is kept in the `frozenfortress-dev-ollama` volume, so later starts don't download it again.

Press Ctrl+C to stop everything. To run the stack in the background instead, use `aspire run --apphost dev/aspire --detach` and stop it with `aspire stop --apphost dev/aspire`.

**After changing Go code**, restart `webui` from the dashboard (or restart the whole stack). Changes to templates and static files under `webui/` only need a browser reload.

## Accounts

The dev stack has its own database in `dev/aspire/.data/`, separate from any other installation. Create an account at <http://localhost:18080/register>, then activate it with the CLI wrapper, which points the CLI at the dev database:

```bash
dev/aspire/ffcli.sh user activate <username>
```

Any other CLI command works the same way, e.g. `dev/aspire/ffcli.sh user list`.

To start over with an empty database, stop the stack and delete `dev/aspire/.data/`.

## Options

Set these environment variables when starting the stack, e.g. `FF_DEV_TLS=1 aspire run --apphost dev/aspire`:

| Variable | Effect |
|----------|--------|
| `FF_DEV_OCR=ollama` | Default. Ollama container, provider `ollama-tesseract`, as in the release. |
| `FF_DEV_OCR=tesseract` | No Ollama. The web UI is built with Tesseract, which needs the Tesseract development packages (`./install-dev-deps-fedora.sh` or `./install-dev-deps-debian.sh`). |
| `FF_DEV_OCR=nop` | No OCR. Fastest start. |
| `FF_DEV_TLS=1` | Adds the release nginx image in front of the web UI at <https://localhost:18443>, with a self-signed certificate. Use this to test HTTPS-only behaviour or the companion app. |
| `FF_DEV_WEBUI=container` | Runs the web UI from the root `Dockerfile` instead of `go run`, to smoke-test the real image before a release. Its data lives in the `frozenfortress-dev-data` volume, not in `dev/aspire/.data/`, so `dev/aspire/ffcli.sh` doesn't apply. Use `docker exec <webui container> /app/ffcli ...` instead. |
| `FF_DEV_PORT` | Web UI port (default `18080`) |
| `FF_DEV_HTTPS_PORT` | nginx port with `FF_DEV_TLS=1` (default `18443`) |

The ports differ from the release stack's `8443`, so a release installation and the dev stack can run side by side.

## Debugging

With the [Aspire extension for VS Code](https://aspire.dev/get-started/aspire-vscode-extension/) and the Go extension installed, start the AppHost from VS Code ("Aspire: Run AppHost" / F5 on `dev/aspire/apphost.mts`). The web UI is then started under Delve, and breakpoints in Go code are hit as usual.

## Updating Aspire

The Aspire SDK and the Go integration are pinned to exact versions in `dev/aspire/aspire.config.json`, and the npm packages to exact versions in `dev/aspire/package.json` and `dev/aspire/package-lock.json`. To update, run `aspire update` in `dev/aspire/`, then check the new versions against the release notes and commit the config and lockfile together (see `AGENTS.md`).
