# FetchQuest

Fetches the recordings and screenshots from your Meta Quest and syncs them to your computer and cloud/network storage to free up space on your headset. Works with Google Drive, Dropbox, NASes running SMB, and more.

```bash
fetchquest stream
```

To use FetchQuest, plug in your Quest via USB and run `fetchquest stream`. It pulls each file off the headset one at a time and syncs it to all of your configured destinations. Once everything is synced, you can run `fetchquest clean` to delete the files from the Quest. It checks the manifest first and only deletes files that have made it to all your destinations.

## Install

Grab a binary from [**Releases**](https://github.com/FluidXR/fetchquest/releases):

| Platform | File |
|----------|------|
| macOS (Apple Silicon) | `fetchquest-x.x.x-darwin-arm64.tar.gz` |
| Linux (x86_64) | `fetchquest-x.x.x-linux-amd64.tar.gz` |
| Windows (x86_64) | `fetchquest-x.x.x-windows-amd64.zip` |

Extract it, put it on your PATH. It'll prompt you to install ADB and rclone on first run if they're missing.

Or with Go 1.24+:

```bash
go install github.com/FluidXR/fetchquest@latest
```

## Quick Start

**1. Plug in your Quest via USB** ([USB debugging](https://developer.oculus.com/documentation/native/android/mobile-device-setup/) needs to be on)

**2. Add a destination:**

```bash
fetchquest config add-dest
```

Walks you through connecting Google Drive, Dropbox, a NAS, or S3. You can paste a folder link or browse.

**3. Sync:**

```bash
fetchquest stream
```

**4. Free up space:**

```bash
fetchquest clean
```

That's it. Run `fetchquest stream` whenever you plug in.

<details>
<summary><b>Or add a destination manually</b></summary>

```bash
rclone config                                            # set up an rclone remote
fetchquest config add-dest my-nas "nas:share/FetchQuest" # register it
```
</details>

## How It Works

FetchQuest uses ADB to pull media off the Quest (USB or WiFi) and rclone to sync it to your destinations. A manifest tracks what's been synced so nothing gets transferred twice.

`fetchquest stream` does one file at a time — pull, sync, delete local copy — so it works fine on machines with limited disk. `fetchquest sync` pulls everything first, then syncs.

`fetchquest clean` only deletes files from the Quest that are confirmed synced to *all* destinations. Pass `--any` to delete files synced to at least one destination instead. `--local` cleans up the local sync directory instead of the Quest. `--dry-run` to preview.

Original file timestamps are preserved.

## Commands

| Command | Description |
|---------|-------------|
| `fetchquest stream` | Pull and sync one file at a time (recommended) |
| `fetchquest sync` | Pull all, then sync to all destinations |
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
- Streaming mode pulls and syncs one file at a time, so you don't need much local disk space
- `fetchquest clean` won't delete anything from the Quest unless it's been synced to every destination you've configured (or at least one, with `--any`)
- Keeps track of what's already been synced so it doesn't transfer the same file twice
- Preserves the original recording timestamps on synced files
- The sync manifest is automatically backed up to your destinations — restore it with `fetchquest config restore` if you lose your local config
- Single binary for macOS, Linux, and Windows

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

```bash
git clone https://github.com/FluidXR/fetchquest.git
cd fetchquest
go build -o fetchquest .
```

Cross-compile:

```bash
GOOS=windows GOARCH=amd64 go build -o fetchquest.exe .
GOOS=linux   GOARCH=amd64 go build -o fetchquest-linux .
GOOS=linux   GOARCH=arm64 go build -o fetchquest-linux-arm64 .
```

## License

MIT
