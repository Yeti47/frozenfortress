# Frozen Fortress — Companion App (Android) Setup Guide

The companion app lets you scan a physical document with your phone's camera (via Google's ML Kit Document Scanner) and send it straight into Frozen Fortress's "Create Document" form, without building a full native client or ever giving the app your Frozen Fortress login.

It's a small, side-loaded Android app — not distributed via the Play Store — published as an APK on this repository's [GitHub Releases](https://github.com/Yeti47/frozenfortress/releases) page, under tags starting with `android-v`.

---

## Requirements

- **Android 8.0 (API 26) or later**
- **Google Play Services** — see [Play Services & De-Googled Devices](#play-services--de-googled-devices) below
- A Frozen Fortress instance reachable from your phone (same Wi-Fi/LAN, a VPN, or a public address), signed in on your phone's browser

---

## Installing

1. On your phone, open the [GitHub Releases](https://github.com/Yeti47/frozenfortress/releases) page and download the latest `frozenfortress-companion-android-v*.apk` (a release tagged `android-vX.Y.Z`).
2. Android will likely block the install until you allow it. When prompted, enable **"Install unknown apps"** for the app you downloaded the APK with (usually your browser or file manager) — this is a per-app, one-time setting in Android's security settings, not a device-wide toggle.
3. Open the downloaded APK and confirm the install.

You do **not** need to sign in to anything inside the companion app itself — it has no login screen. It only ever talks to a Frozen Fortress instance your browser tells it about, for the duration of a single scan.

---

## Scanning a Document

1. On your phone (or any device where you're signed in to Frozen Fortress), open the **Create Document** page and tap **"Scan with companion app"**.
2. Your browser opens the companion app via a deep link. The first time you scan against a given Frozen Fortress instance, you'll see a certificate confirmation screen — see [Certificate Verification (TOFU)](#certificate-verification-tofu) below.
3. The Google ML Kit document scanner opens: capture the page(s), then confirm. The app produces a single PDF.
4. The app encrypts the PDF and uploads it, then shows a **"Return to FrozenFortress"** button. Tap it to jump back to your browser.
5. Back in Frozen Fortress, the scanned PDF is automatically added to the document you were creating — review it and submit as normal.

The whole handoff (from tapping "Scan with companion app" to the file appearing in your browser) expires after a few minutes if left incomplete, so if something goes wrong, just start again from step 1.

---

## Certificate Verification (TOFU)

Frozen Fortress instances are commonly self-hosted with a self-signed TLS certificate (or no TLS at all on a trusted LAN). The companion app can't validate a self-signed certificate against a public certificate authority the way a browser normally would, so instead it uses **Trust-On-First-Use (TOFU)** — the same idea as SSH's "unknown host key" prompt, or a browser's own self-signed certificate warning.

- **First connection to a given Frozen Fortress instance**: the app shows you the certificate's fingerprint and asks you to confirm it before proceeding. Only accept this if you recognize the server — if you're scanning against your own self-hosted instance, this is expected and safe to accept.
- **Every connection after that**: the app silently checks the fingerprint still matches what you confirmed. If it ever changes unexpectedly, the app refuses to connect rather than silently trusting the new certificate — a changed fingerprint on a previously-trusted server is exactly the kind of thing TOFU exists to catch (e.g. a man-in-the-middle attempting to intercept the scan).

If your Frozen Fortress instance's certificate legitimately changes (e.g. you regenerate a self-signed cert, or move to a new one), the app will refuse the connection until you clear its stored certificate pin — currently this means clearing the app's storage/data from Android's app settings, since there's no in-app "forget this server" option yet.

---

## Play Services & De-Googled Devices

The document scanner is powered by **Google Play Services** (`play-services-mlkit-document-scanner`). This means the companion app **will not work on de-Googled devices** (e.g. GrapheneOS without sandboxed Play Services, LineageOS without MicroG/Play Services, or similar) — the scan step will simply fail to open. This is a known limitation, not a bug; there's currently no plan to add a non-Play-Services scanning fallback.

If your phone has standard Google Play Services installed (the vast majority of Android phones, including all stock Android and most custom ROMs that bundle GApps), this isn't something you need to worry about.

---

## Updating

Starting with the app's first release, companion app builds are signed with a stable release key, so installing a newer release over an older one upgrades it in place — you don't need to uninstall first. Just download the newer APK from [GitHub Releases](https://github.com/Yeti47/frozenfortress/releases) and install it the same way as the first time.

---

## Security Notes

- The companion app never sees or stores your Frozen Fortress password or session — it only receives a one-time, single-use encryption key and upload token via the deep link from your already-authenticated browser.
- The scanned document is encrypted (AES-256-GCM) on your phone before it ever leaves the device, using a one-time key your browser generated. The companion app never receives this key from the server — it only gets it directly from your browser via the deep link, and only sends *encrypted* bytes to the server, never the key itself.
- The server does hold its own copy of that key (given to it directly by your browser when the scan started, with a short expiry) so it can decrypt the file once for your browser to fetch — the point of the encrypted handoff is to keep the plaintext scan off any endpoint the companion app itself can reach, not to keep it from the server your browser is already authenticated to.
- The key and upload token the companion app receives are held only in memory for the duration of the scan and are never written to disk or logged by the app.
