# FetchQuest

A CLI tool that pulls videos, screenshots, and photos off Meta Quest headsets and syncs them to cloud/NAS destinations.

## Install

### Download a binary

Grab the latest release for your platform from [**Releases**](https://github.com/FluidXR/fetchquest/releases):

- **macOS (Apple Silicon):** `fetchquest-x.x.x-darwin-arm64.tar.gz`
- **Linux (x86_64):** `fetchquest-x.x.x-linux-amd64.tar.gz`
- **Windows (x86_64):** `fetchquest-x.x.x-windows-amd64.zip`

Extract it and put the binary somewhere on your PATH.

> FetchQuest will prompt you to install ADB and rclone on first run if they're missing.

### Or build from source

Requires [Go 1.24+](https://go.dev/dl/):

```bash
go install github.com/FluidXR/fetchquest@latest
```

## Quick Start

**1. Plug in your Quest via USB** (with [USB debugging enabled](https://developer.oculus.com/documentation/native/android/mobile-device-setup/))

**2. Check that it's detected:**

```bash
fetchquest devices
```

FetchQuest will ask you to nickname the device the first time it sees it.

**3. Set up a sync destination** (Google Drive, NAS, S3, Dropbox, etc.):

```bash
rclone config                                          # create an rclone remote
fetchquest config add-dest my-nas "nas:share/FetchQuest"  # tell fetchquest about it
```

<details>
<summary><b>Example: Google Drive</b></summary>

```bash
rclone config create gdrive drive                      # opens browser to authorize
fetchquest config add-dest gdrive "gdrive:FetchQuest"
```
</details>

<details>
<summary><b>Example: Dropbox</b></summary>

```bash
rclone config create dropbox dropbox                   # opens browser to authorize
fetchquest config add-dest dropbox "dropbox:FetchQuest"
```
</details>

<details>
<summary><b>Example: SMB/NAS</b></summary>

```bash
rclone config create nas smb host 192.168.1.100 user myuser
rclone config password nas pass "mypassword"
fetchquest config add-dest nas "nas:share/FetchQuest"
```
</details>

**4. Sync!**

```bash
fetchquest stream   # pull one file at a time, upload, delete local copy (low disk usage)
# or
fetchquest sync     # pull all files locally first, then upload
```

**5. Free up space on the Quest** (only deletes files confirmed synced to ALL destinations):

```bash
fetchquest clean
```

That's it. Run `fetchquest stream` whenever you plug in your Quest to back up new recordings.

## Commands

| Command | Description |
|---------|-------------|
| `fetchquest stream` | Pull one file → upload → delete local copy → repeat (recommended) |
| `fetchquest sync` | Pull all media locally, then upload to all destinations |
| `fetchquest pull` | Only pull media from Quest to local directory |
| `fetchquest push` | Only upload local media to destinations |
| `fetchquest clean` | Delete already-synced media from Quest (`--dry-run` to preview) |
| `fetchquest devices` | List connected Quests and their sync status |
| `fetchquest config` | View/manage configuration |

### Config subcommands

```bash
fetchquest config init                              # create default config
fetchquest config add-dest <name> <rclone_remote>   # add a sync destination
fetchquest config remove-dest <name>                # remove a destination
fetchquest config nickname <serial> <name>          # name a device
fetchquest config set-wifi <serial> <ip>            # set WiFi IP for wireless ADB
fetchquest config restore [destination-name]        # restore manifest DB from backup
```

### Flags

Most commands accept `-d <serial>` to target a specific device.

`fetchquest clean` also supports:
- `--dry-run` — preview what would be deleted
- `--confirm` — skip the interactive confirmation

## Features

- **Multi-device** — handles multiple Quests without interference
- **Streaming mode** — one-file-at-a-time for machines with limited disk space
- **Multiple destinations** — sync to Google Drive, SMB/NAS, S3, or any [rclone-supported backend](https://rclone.org/overview/)
- **Smart dedup** — never syncs the same file twice
- **Safe cleanup** — only deletes from Quest after confirmed sync to ALL destinations
- **Date preservation** — original recording timestamps are kept on synced files
- **Manifest backup** — sync state is automatically backed up to your destinations and can be restored with `fetchquest config restore`
- **Cross-platform** — pure Go, no CGO, runs on macOS, Linux, and Windows

## Config File

Located at `~/.config/fetchquest/config.yaml`:

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
