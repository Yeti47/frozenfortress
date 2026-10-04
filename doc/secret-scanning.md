# Secret scanning (gitleaks)

Frozen Fortress uses [gitleaks](https://github.com/gitleaks/gitleaks) to keep typical secrets (API keys, tokens, private keys, password assignments) out of the repository. It runs in two places:

- **Locally**, as a `pre-commit` hook that scans the staged changes.
- **In CI**, as the `Secret scan` workflow, which scans the full git history and the working tree on every push and pull request to `master` and `feature/v2`.

CI is the backstop: a local hook can be skipped or never installed.

## Set up the hook

```bash
./setup-hooks.sh --install
```

This downloads the pinned gitleaks release to `~/.local/bin`, verifies its SHA-256 checksum, and sets `core.hooksPath` to `.githooks`. If you already have gitleaks, run `./setup-hooks.sh` without `--install`. A commit is blocked when gitleaks is missing, so a skipped setup shows up immediately.

The pinned version and checksums live in one place, `.githooks/gitleaks.env`. The hook, `setup-hooks.sh` and CI all use it. The hook warns if your installed version differs from the pin.

## When the hook blocks a commit

1. Remove the secret from the change and load it at runtime instead (see AGENTS.md, section 2).
2. If the value was ever real, **rotate it**. Removing it from the commit is not enough, because anything that reached a remote, a log or a CI run must be treated as exposed.
3. If it is a verified false positive, add a justified entry to `.gitleaks.toml`. Every allowlist entry needs a comment saying what it matches and why it is not a secret.

## Bypassing the hook

`git commit --no-verify` skips the hook. Use it only in an emergency. CI scans the same content, so a bypassed commit still fails the pull request, and the secret still has to be rotated.

## Scanning by hand

```bash
gitleaks git --redact .   # full history
gitleaks dir --redact .   # working tree, including untracked files
```

Always pass `--redact` so findings do not print the secret into your terminal or CI logs.

## Updating gitleaks

1. Pick the new release and read its release notes.
2. Replace the version and all four checksums in `.githooks/gitleaks.env`, using `gitleaks_<version>_checksums.txt` from the official release. Cross-check them against the asset digests GitHub shows for the release.
3. Run a full scan and commit the change together with any `.gitleaks.toml` adjustments.
