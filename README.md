# openproject-tracker

A Linux desktop time-tracking application written in Go using the [Fyne](https://fyne.io/) UI toolkit. It integrates with [OpenProject](https://www.openproject.org/) via its REST API v3 and handles SSO authentication through an [Authentik](https://goauthentik.io/) proxy using an external Chromium window driven by [Playwright for Go](https://github.com/playwright-community/playwright-go).

## Features

- **SSO login via external browser** – A headful Chromium window opens so you can complete Authentik SSO; the app detects success and captures session cookies automatically.
- **Persistent cookie jar** – Cookies are stored at `~/.config/openproject-tracker/cookies.json` (mode `0600`) and loaded on startup so you stay logged in across runs.
- **Assigned work packages** – Dropdown listing all OpenProject work packages assigned to you (`#ID Subject (Project)`).
- **Activity selection** – Second dropdown showing available time-entry activity types, required before stopping the timer.
- **Start / Stop timer** – Start begins a local clock; Stop computes elapsed time, sends a `POST /api/v3/time_entries` request, and resets the UI.
- **Comment field** – Multi-line text area for describing work done, included in the time entry.
- **SSO bypass / TLS options** – Settings window lets you change the base URL, skip TLS verification (with warning), supply a custom CA certificate, and add arbitrary extra HTTP request headers. Settings persist to `~/.config/openproject-tracker/config.json`.

## Requirements

| Dependency | Notes |
|---|---|
| Go 1.21+ | <https://go.dev/dl/> |
| GCC / CGO toolchain | Required by Fyne (OpenGL) |
| X11 / Wayland dev libs | `libgl1-mesa-dev xorg-dev` on Debian/Ubuntu |
| Playwright Chromium | Installed via `go run github.com/playwright-community/playwright-go/cmd/playwright@v0.4701.0 install --with-deps chromium` |

### Install system dependencies (Debian / Ubuntu)

```bash
sudo apt-get update
sudo apt-get install -y gcc libgl1-mesa-dev xorg-dev
```

## Setup

```bash
git clone https://github.com/pjanzen/openproject-tracker.git
cd openproject-tracker

# Download Go module dependencies
go mod download

# Install Playwright + Chromium (downloads ~170 MB)
go run github.com/playwright-community/playwright-go/cmd/playwright@v0.4701.0 install --with-deps chromium
```

## Running the application

```bash
go run .
```

Or build a binary first:

```bash
go build -o openproject-tracker .
./openproject-tracker
```

### First-time login

1. Click **"Login via SSO"**.
2. A Chromium window opens and navigates to `https://projects.unified.services`.
3. Complete the Authentik SSO flow in the browser.
4. The app polls `/api/v3/my_preferences` every 2 seconds. When it returns HTTP 200 the login is considered successful.
5. Cookies are exported and saved to `~/.config/openproject-tracker/cookies.json`.
6. The task and activity dropdowns are populated automatically.

On subsequent launches the stored cookies are loaded and the session is verified silently. If cookies have expired you will be prompted to log in again.

## Tracking time

1. Select a **task** from the first dropdown (or click **Refresh** to reload).
2. Select an **activity** from the second dropdown.
3. Click **Start** – the HH:MM:SS timer begins.
4. (Optional) Type a comment in the **Comment** box.
5. Click **Stop** – elapsed time is posted to OpenProject as a new time entry.

## Settings

Click **Settings** to open the configuration window:

| Field | Description |
|---|---|
| Base URL | OpenProject root URL (default `https://projects.unified.services`) |
| Skip TLS Verification | Disables certificate validation (insecure – use only for self-signed certs) |
| CA Certificate Path | Path to a PEM CA bundle to trust in addition to system roots |
| Extra Headers | One `Header-Name: value` pair per line, added to every API request |

Settings are saved to `~/.config/openproject-tracker/config.json`.

## Cookie security

- `cookies.json` is written with permissions `0600` (readable only by you).
- Cookies are equivalent to your login session – treat them like a password.
- The config directory `~/.config/openproject-tracker/` is created with permissions `0700`.

## Troubleshooting

### Playwright / Chromium not found

Run the install command again:

```bash
go run github.com/playwright-community/playwright-go/cmd/playwright@v0.4701.0 install --with-deps chromium
```

### Login window opens but never detects success

- Make sure you are fully logged in – the app waits up to **5 minutes**.
- Check that the base URL in Settings is correct and resolves to the OpenProject instance.
- If the Authentik callback redirects to a different domain, all cookies from that domain are captured automatically.

### API calls return 401 / redirect after a successful login

Authentik session cookies can be short-lived depending on server policy. Click **Login via SSO** again to refresh them.

### TLS certificate errors

If the server uses a self-signed certificate, either:
- Supply the CA bundle path in Settings → **CA Certificate Path**, or
- Enable **Skip TLS Verification** in Settings (not recommended for production).

### Fyne build errors on Linux

Ensure `libgl1-mesa-dev` and `xorg-dev` are installed, and that `CGO_ENABLED=1` (the default). Wayland sessions may require `libwayland-dev` as well.