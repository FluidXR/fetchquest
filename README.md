# FetchQuest

A CLI tool that pulls videos, screenshots, and photos off Meta Quest headsets via ADB and syncs them to cloud/NAS destinations via rclone.

## Features

- **Multi-device support** — sync from multiple Quests simultaneously, each tracked by serial number
- **Streaming mode** — one-file-at-a-time transfer for machines with limited disk space
- **Multiple destinations** — sync to Google Drive, SMB/NAS, S3, or any rclone-supported backend
- **Smart deduplication** — SQLite manifest tracks every file so nothing gets synced twice
- **Safe cleanup** — only deletes files from Quest after confirming they've reached ALL destinations
- **File metadata preservation** — original creation/modification dates from the Quest are preserved on local and remote copies, so your files sort correctly by recording date
- **Manifest backup** — manifest DB is automatically backed up to your destinations

## Prerequisites

- [Go 1.24+](https://go.dev/dl/) (for building)
- [ADB](https://developer.android.com/tools/adb) (Android Debug Bridge)
- [rclone](https://rclone.org/install/) (for cloud/NAS uploads)

### Install prerequisites

**macOS:**
```bash
brew install android-platform-tools rclone
```

**Windows:**
- ADB: Download [Android SDK Platform Tools](https://developer.android.com/tools/releases/platform-tools) and add to PATH
- rclone: `winget install Rclone.Rclone` or download from [rclone.org](https://rclone.org/install/)

**Linux:**
```bash
sudo apt install android-tools-adb   # Debian/Ubuntu
sudo pacman -S android-tools         # Arch
curl https://rclone.org/install.sh | sudo bash
```

## Building

```bash
git clone https://github.com/FluidXR/fetchquest.git
cd fetchquest
go build -ldflags "-X github.com/FluidXR/fetchquest/cmd.Version=0.1.0" -o fetchquest .
```

Or install directly:

```bash
go install github.com/FluidXR/fetchquest@latest
```

### Cross-compile

FetchQuest is pure Go with no CGO dependencies, so cross-compiling is straightforward:

```bash
# Windows
GOOS=windows GOARCH=amd64 go build -o fetchquest.exe .

# Linux
GOOS=linux GOARCH=amd64 go build -o fetchquest-linux .

# Linux (ARM, e.g. Raspberry Pi)
GOOS=linux GOARCH=arm64 go build -o fetchquest-linux-arm64 .
```

## Quick Start

### 1. Initialize config

```bash
fetchquest config init
```

This creates `~/.config/fetchquest/config.yaml` with default settings.

### 2. Set up an rclone destination

First configure an rclone remote (e.g., SMB, Google Drive, S3):

```bash
rclone config
```

Then add it as a FetchQuest destination:

```bash
fetchquest config add-dest my-nas "nas:share/QuestMedia"
```

### 3. Connect your Quest

Connect your Quest via USB and enable USB debugging. Verify it's detected:

```bash
fetchquest devices
```

### 4. Sync

Full sync (pull from Quest, then push to all destinations):

```bash
fetchquest sync
```

Or use streaming mode if you're low on disk space:

```bash
fetchquest stream
```

## Commands

| Command | Description |
|---------|-------------|
| `fetchquest pull` | Pull media from Quest(s) to local sync directory |
| `fetchquest push` | Upload local media to rclone destinations |
| `fetchquest sync` | Pull + push in one command |
| `fetchquest stream` | One-file-at-a-time: pull, push, delete local copy |
| `fetchquest clean` | Delete already-synced media from Quest(s) |
| `fetchquest devices` | List connected Quests and sync status |
| `fetchquest config` | View current configuration |

### Config subcommands

| Command | Description |
|---------|-------------|
| `fetchquest config init` | Create default config file |
| `fetchquest config add-dest <name> <remote>` | Add an rclone destination |
| `fetchquest config remove-dest <name>` | Remove a destination |
| `fetchquest config nickname <serial> <name>` | Set a device nickname |
| `fetchquest config set-wifi <serial> <ip>` | Set WiFi IP for wireless ADB |

### Flags

Most commands support:
- `-d, --device <serial>` — target a specific device (default: all connected)

Clean command:
- `--dry-run` — show what would be deleted without deleting
- `--confirm` — skip the confirmation prompt

## Config File

Located at `~/.config/fetchquest/config.yaml`:

```yaml
sync_dir: ~/QuestMedia
destinations:
  - name: my-nas
    rclone_remote: "nas:share/QuestMedia"
  - name: google-drive
    rclone_remote: "gdrive:QuestMedia"
devices:
  ABC123:
    nickname: "John's Quest 3"
    wifi_ip: "192.168.1.42"
media_paths:
  - /sdcard/Oculus/VideoShots/
  - /sdcard/Oculus/Screenshots/
```

## How It Works

1. **Pull** — Lists files on Quest via ADB, diffs against the SQLite manifest, pulls only new files
2. **Push** — Uploads unpushed local files to each rclone destination, records sync status
3. **Stream** — Pulls one file, uploads to all destinations, deletes local copy, repeats
4. **Clean** — Checks manifest to find files synced to ALL destinations, deletes them from Quest

Files are organized locally as `<sync_dir>/<device_serial>/Videos/` and `<sync_dir>/<device_serial>/Screenshots/`.

## License

MIT
