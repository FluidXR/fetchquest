# FetchQuest

Fetches the recordings and screenshots from your Meta Quest and syncs them to your computer or cloud/network storage to free up space on your headset. Works with Google Drive, Dropbox, NASes running SMB, and more.

```bash
fetchquest sync
```

To use FetchQuest, plug in your Quest via USB and run `fetchquest sync`. It pulls all the VideoShots and Screenshots off the headset and syncs them to all of your configured destinations. Once everything is synced, you can run `fetchquest clean` to delete the files from the Quest. It checks the manifest first and only deletes files that have made it to all your destinations.

## Install

Grab a binary from [**Releases**](https://github.com/FluidXR/fetchquest/releases):

| Platform | Download |
|----------|----------|
| macOS (Apple Silicon) | [fetchquest-0.4.0-darwin-arm64.tar.gz](https://github.com/FluidXR/fetchquest/releases/download/v0.4.0/fetchquest-0.4.0-darwin-arm64.tar.gz) |
| Linux (x86_64) | [fetchquest-0.4.0-linux-amd64.tar.gz](https://github.com/FluidXR/fetchquest/releases/download/v0.4.0/fetchquest-0.4.0-linux-amd64.tar.gz) |
| Windows (x86_64) | [fetchquest-0.4.0-windows-amd64.zip](https://github.com/FluidXR/fetchquest/releases/download/v0.4.0/fetchquest-0.4.0-windows-amd64.zip) |

Extract it, put it on your PATH. It'll prompt you to install ADB and rclone on first run if they're missing.

Or with Go 1.21+:

```bash
go install github.com/FluidXR/fetchquest@latest
```

**Building from source** requires **Go 1.21 or newer**. If you see `package cmp is not in GOROOT` or `package slices is not in GOROOT`, your `go` is too old. Install a newer Go:

```bash
# Linux: install Go 1.24 to ~/.local (no sudo)
curl -sL https://go.dev/dl/go1.24.4.linux-amd64.tar.gz | tar -C "$HOME/.local" -xzf -
export PATH="$HOME/.local/go/bin:$PATH"
go version   # should show go1.24.4
```

Then run `go build` from the repo root.

## Quick Start

**1. Plug in your Quest via USB** ([USB debugging](https://developer.oculus.com/documentation/native/android/mobile-device-setup/) needs to be on)

**2. Add a destination:**

```bash
fetchquest config add-dest
```

Walks you through connecting a local folder, Google Drive, Dropbox, a NAS, or S3. You can paste a folder link or browse.

**3. Sync:**

```bash
fetchquest sync
```

**4. Free up space:**

```bash
fetchquest clean
```

That's it. Run `fetchquest sync` whenever you plug in.

## Desktop App (GUI)

A pixel-art themed desktop app lets you sync, browse files, manage destinations, and clean your Quest — no terminal required.

- **Sync** — one-click sync with real-time progress bars and per-file transfer percentages
- **Scout Ahead** — preview what will be synced before committing
- **Files** — browse files on the Quest or locally, with sync status for each file
- **Places** — add and manage destinations (Google Drive, Dropbox, NAS/SMB, or any rclone remote)
- **Clean Quest** — delete fully backed-up files from the headset, with a confirmation modal showing exactly what will be removed
- **Settings** — set your local sync directory and nickname your devices

### Running the app

Double-click **FetchQuest** (or `FetchQuest.exe` on Windows). Plug in your Quest via USB, add a destination from the Places tab, and hit **Begin Quest!**

### Building from source

Requires [Go 1.21+](https://go.dev/dl/) and the [Wails CLI](https://wails.io/docs/gettingstarted/installation):

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

On **Linux**, install WebKitGTK first:
```bash
sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev  # Ubuntu/Debian
```

Then build:
```bash
cd ui
wails build
```

The built app will be in `ui/build/bin/`.

<details>
<summary><b>Or add a destination manually</b></summary>

```bash
rclone config                                            # set up an rclone remote
fetchquest config add-dest my-nas "nas:share/FetchQuest" # register it
```
</details>

## How It Works

FetchQuest uses ADB to pull media off the Quest (USB or WiFi) and rclone to sync it to your destinations. A manifest tracks what's been synced so nothing gets transferred twice.

`fetchquest sync` pulls everything locally first, then syncs to all destinations. Pass `--skip-local` if you don't want to keep local copies — it will pull and sync one file at a time straight to your destinations.

`fetchquest clean` only deletes files from the Quest that are confirmed synced to *all* destinations. Pass `--any` to delete files synced to at least one destination instead. `--local` cleans up the local sync directory instead of the Quest. `--dry-run` to preview.

Original file timestamps are preserved.

## Commands

| Command | Description |
|---------|-------------|
| `fetchquest sync` | Pull all media from Quest, then sync to all destinations |
| `fetchquest sync --skip-local` | Sync straight to destinations without keeping local copies |
| `fetchquest pull` | Pull media from Quest to local directory |
| `fetchquest push` | Sync local media to destinations |
| `fetchquest clean` | Delete synced media from Quest |
| `fetchquest clean --local` | Delete local files that have already been synced to destinations |
| `fetchquest devices` | List connected Quests and sync stats |
| `fetchquest config` | View/manage config |
| `fetchquest config add-dest` | Add a destination (interactive) |

## Features

- Works with multiple Quests — each device is tracked separately so files don't get mixed up
- Sync to Google Drive, Dropbox, NAS, S3, or any of the [70+ backends rclone supports](https://rclone.org/overview/), and you can sync to more than one at a time
- `--skip-local` mode syncs straight to destinations without keeping local copies, for machines with limited disk space
- `fetchquest clean` won't delete anything from the Quest unless it's been synced to every destination you've configured (or at least one, with `--any`)
- Keeps track of what's already been synced so it doesn't transfer the same file twice
- Preserves the original recording timestamps on synced files
- The sync manifest is automatically backed up to your destinations — restore it with `fetchquest config restore` if you lose your local config
- Single binary for macOS, Linux, and Windows
- **Desktop GUI** — same sync workflow in a windowed app (no terminal required)

## Config

`~/.config/fetchquest/config.yaml`:

```yaml
sync_dir: ~/FetchQuest
destinations:
  - name: my-nas
    rclone_remote: "nas:share/FetchQuest"
  - name: google-drive
    rclone_remote: "gdrive:FetchQuest"
devices:
  ABC123:
    nickname: "John's Quest 3"
    wifi_ip: "192.168.1.42"
media_paths:
  - /sdcard/Oculus/VideoShots/
  - /sdcard/Oculus/Screenshots/
```

## Building from Source

**CLI only:**

```bash
git clone https://github.com/FluidXR/fetchquest.git
cd fetchquest
go build -o fetchquest .
```

**Desktop app (GUI):** See [Desktop App](#desktop-app-gui) above for Wails build steps.

Cross-compile CLI:

```bash
GOOS=windows GOARCH=amd64 go build -o fetchquest.exe .
GOOS=linux   GOARCH=amd64 go build -o fetchquest-linux .
GOOS=linux   GOARCH=arm64 go build -o fetchquest-linux-arm64 .
```

## License

MIT
