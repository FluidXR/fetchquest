package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	gosync "sync"
	"time"

	"github.com/FluidXR/fetchquest/internal/adb"
	"github.com/FluidXR/fetchquest/internal/config"
	"github.com/FluidXR/fetchquest/internal/manifest"
	"github.com/FluidXR/fetchquest/internal/rclone"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App exposes methods to the frontend.
type App struct {
	ctx context.Context
}

// SetContext is called by Wails on startup. Do not call from the frontend.
func (a *App) SetContext(ctx context.Context) {
	a.ctx = ctx
}

// NewApp returns an App instance for Wails binding.
func NewApp() *App {
	return &App{}
}

// DeviceInfo is sent to the frontend.
type DeviceInfo struct {
	Serial   string `json:"serial"`
	Model    string `json:"model"`
	Status   string `json:"status"`
	Nickname string `json:"nickname"`
	Stats    string `json:"stats"`
}

// DepInfo describes a missing dependency.
type DepInfo struct {
	Name    string `json:"name"`
	Install string `json:"install"`
}

// DestinationEntry is one destination for the UI (name + remote).
type DestinationEntry struct {
	Name   string `json:"name"`
	Remote string `json:"remote"`
}

// ConfigSummary is sent to the frontend.
type ConfigSummary struct {
	SyncDir          string             `json:"syncDir"`
	Destinations     []string           `json:"destinations"`
	DestinationsList []DestinationEntry `json:"destinationsList"`
	RcloneRemotes    []string           `json:"rcloneRemotes"` // for dropdown: remotes from "rclone listremotes"
	MissingDeps      []DepInfo          `json:"missingDeps"`
	HasDestinations  bool               `json:"hasDestinations"`
}

// GetDevices returns connected Quest devices and their sync stats.
func (a *App) GetDevices() ([]DeviceInfo, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	adbClient := adb.NewClient()
	devs, err := adbClient.Devices()
	if err != nil {
		return nil, err
	}

	db, err := manifest.Open(config.ConfigDir())
	if err != nil {
		return nil, fmt.Errorf("open manifest: %w", err)
	}
	defer db.Close()

	numDests := len(cfg.Destinations)
	var result []DeviceInfo

	for _, d := range devs {
		nickname := ""
		if dc, ok := cfg.Devices[d.Serial]; ok && dc.Nickname != "" {
			nickname = dc.Nickname
		}

		status := d.State
		if !d.IsOnline() {
			status = "OFFLINE"
		}

		stats := ""
		if d.IsOnline() {
			onDevice := 0
			for _, mp := range cfg.MediaPaths {
				if files, err := adbClient.ListFilesRecursive(d.Serial, mp); err == nil {
					onDevice += len(files)
				}
			}
			if s, err := db.GetDeviceStats(d.Serial, numDests); err == nil {
				stats = fmt.Sprintf("%d on device · %d backed up", onDevice, s.SyncedFiles)
			} else if onDevice > 0 {
				stats = fmt.Sprintf("%d on device", onDevice)
			}
		}

		result = append(result, DeviceInfo{
			Serial:   d.Serial,
			Model:    d.Model,
			Status:   status,
			Nickname: nickname,
			Stats:    stats,
		})
	}

	return result, nil
}

// GetConfig returns a summary of config for the UI.
func (a *App) GetConfig() (ConfigSummary, error) {
	cfg, err := config.Load()
	if err != nil {
		return ConfigSummary{}, err
	}

	var dests []string
	var destsList []DestinationEntry
	for _, d := range cfg.Destinations {
		dests = append(dests, d.Name+": "+d.RcloneRemote)
		destsList = append(destsList, DestinationEntry{Name: d.Name, Remote: d.RcloneRemote})
	}

	missing := checkDeps()
	var rcloneRemotes []string
	if rc := rclone.NewClient(); rc != nil {
		if list, err := rc.ListRemotes(); err == nil {
			rcloneRemotes = list
		}
	}

	return ConfigSummary{
		SyncDir:          cfg.SyncDir,
		Destinations:     dests,
		DestinationsList: destsList,
		RcloneRemotes:    rcloneRemotes,
		MissingDeps:      missing,
		HasDestinations:  len(cfg.Destinations) > 0,
	}, nil
}

// SetSyncDir sets the local sync folder and saves config.
func (a *App) SetSyncDir(path string) error {
	if path == "" {
		return fmt.Errorf("path is required")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.SyncDir = path
	return config.Save(cfg)
}

// AddDestination adds an rclone destination (name + remote, e.g. "gdrive:FetchQuest").
// The rclone remote must already exist (run "rclone config" in a terminal if needed).
func (a *App) AddDestination(name string, remote string) error {
	name = strings.TrimSpace(name)
	remote = strings.TrimSpace(remote)
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if remote == "" {
		return fmt.Errorf("remote is required (e.g. gdrive:FetchQuest)")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	for _, d := range cfg.Destinations {
		if d.Name == name {
			return fmt.Errorf("a destination named %q already exists", name)
		}
	}
	cfg.Destinations = append(cfg.Destinations, config.Destination{Name: name, RcloneRemote: remote})
	return config.Save(cfg)
}

// RemoveDestination removes a destination by name.
func (a *App) RemoveDestination(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	var kept []config.Destination
	for _, d := range cfg.Destinations {
		if d.Name != name {
			kept = append(kept, d)
		}
	}
	if len(kept) == len(cfg.Destinations) {
		return fmt.Errorf("destination %q not found", name)
	}
	cfg.Destinations = kept
	return config.Save(cfg)
}

// SetupOAuthRemote creates (or reuses) an OAuth-based rclone remote.
// rcloneType must be "drive" or "dropbox". Opens the user's browser for authorization.
// Returns the rclone remote name (e.g. "gdrive").
func (a *App) SetupOAuthRemote(rcloneType string) (string, error) {
	nameMap := map[string]string{
		"drive":   "gdrive",
		"dropbox": "dropbox",
	}
	baseName, ok := nameMap[rcloneType]
	if !ok {
		return "", fmt.Errorf("unsupported type: %s", rcloneType)
	}

	// Reuse existing remote if already configured
	rc := rclone.NewClient()
	existing, _ := rc.ListRemotes()
	for _, e := range existing {
		if strings.TrimSuffix(e, ":") == baseName {
			return baseName, nil
		}
	}

	// Create new remote — opens browser for OAuth
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	out, err := exec.CommandContext(ctx, "rclone", "config", "create", baseName, rcloneType).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("authorization failed — check your browser and try again: %w\n%s", err, out)
	}
	return baseName, nil
}

// SetupSMBRemote creates an SMB/NAS rclone remote with the given credentials.
// Returns the rclone remote name (e.g. "nas").
func (a *App) SetupSMBRemote(host, user, pass string) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", fmt.Errorf("server address is required")
	}

	remoteName := "nas"
	rc := rclone.NewClient()
	existing, _ := rc.ListRemotes()
	remoteName = uniqueRemoteName(remoteName, existing)

	args := []string{"config", "create", remoteName, "smb", "host", host}
	if user = strings.TrimSpace(user); user != "" {
		args = append(args, "user", user)
	}
	out, err := exec.Command("rclone", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to connect: %w\n%s", err, out)
	}

	if user != "" && pass != "" {
		out, err = exec.Command("rclone", "config", "password", remoteName, "pass", pass).CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to set password: %w\n%s", err, out)
		}
	}
	return remoteName, nil
}

// RemoteFolder is a folder listed on an rclone remote.
type RemoteFolder struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// ListRemoteFolders lists directories at a path on a remote.
// destType is used to apply backend-specific overrides (e.g. clearing
// root_folder_id on Google Drive so the browser shows the full drive).
// rootFolderID optionally scopes a Google Drive listing to a specific folder.
func (a *App) ListRemoteFolders(remoteName, path, destType, rootFolderID string) ([]RemoteFolder, error) {
	var remote string
	if destType == "drive" {
		if rootFolderID != "" {
			// Browse inside a specific Google Drive folder (from a pasted URL)
			remote = remoteName + ",root_folder_id=" + rootFolderID + ":" + path
		} else {
			// Clear root_folder_id so we browse the full drive
			remote = remoteName + ",root_folder_id=:" + path
		}
	} else {
		remote = remoteName + ":" + path
	}
	out, err := exec.Command("rclone", "lsd", remote).CombinedOutput()
	if err != nil {
		return []RemoteFolder{}, nil // empty, not an error (remote might be empty)
	}
	var folders []RemoteFolder
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// rclone lsd format: "  -1 2025-01-01 00:00:00  -1 dirname"
		parts := strings.Fields(line)
		if len(parts) >= 5 {
			dirName := strings.Join(parts[4:], " ")
			fullPath := dirName
			if path != "" {
				fullPath = path + "/" + dirName
			}
			folders = append(folders, RemoteFolder{Name: dirName, Path: fullPath})
		}
	}
	if folders == nil {
		folders = []RemoteFolder{}
	}
	return folders, nil
}

// AddDestinationAuto adds a destination with an auto-generated name.
// folderPath can be a plain path (e.g. "FetchQuest") or a Google Drive / Dropbox URL.
// rootFolderID, if set, scopes a Google Drive remote to a specific folder (from a pasted URL).
// Creates the folder on the remote if needed.
func (a *App) AddDestinationAuto(remoteName, folderPath, destType, rootFolderID string) error {
	// If the frontend already resolved a URL and provided rootFolderID, set it on the remote
	if rootFolderID != "" && destType == "drive" {
		out, err := exec.Command("rclone", "config", "update", remoteName, "root_folder_id", rootFolderID).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to configure Google Drive folder: %w\n%s", err, out)
		}
	} else {
		// No rootFolderID — check if the folder path is a pasted URL
		resolved, err := resolveInputPath(remoteName, folderPath, destType)
		if err != nil {
			return err
		}
		folderPath = resolved
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	nameMap := map[string]string{
		"drive":   "google-drive",
		"dropbox": "dropbox",
		"smb":     "nas",
	}
	baseName := nameMap[destType]
	if baseName == "" {
		baseName = destType
	}
	destName := uniqueDestName(baseName, cfg.Destinations)

	remoteStr := remoteName + ":"
	if folderPath != "" {
		remoteStr += folderPath
	}

	// Create folder on remote
	if folderPath != "" {
		out, err := exec.Command("rclone", "mkdir", remoteStr).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to create folder: %w\n%s", err, out)
		}
	}

	cfg.Destinations = append(cfg.Destinations, config.Destination{
		Name:         destName,
		RcloneRemote: remoteStr,
	})
	return config.Save(cfg)
}

var driveURLPattern = regexp.MustCompile(`drive\.google\.com/drive/.*folders/([a-zA-Z0-9_-]+)`)
var dropboxURLPattern = regexp.MustCompile(`dropbox\.com/home(.*)`)

// resolveInputPath handles pasted URLs and returns a clean folder path.
// For Google Drive URLs, it sets root_folder_id on the rclone remote.
func resolveInputPath(remoteName, input, destType string) (string, error) {
	input = strings.TrimSpace(input)

	if m := driveURLPattern.FindStringSubmatch(input); m != nil {
		folderID := m[1]
		out, err := exec.Command("rclone", "config", "update", remoteName, "root_folder_id", folderID).CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to set Google Drive folder: %w\n%s", err, out)
		}
		return "FetchQuest", nil
	}

	if m := dropboxURLPattern.FindStringSubmatch(input); m != nil {
		path := strings.TrimPrefix(m[1], "/")
		if path == "" {
			return "FetchQuest", nil
		}
		return path + "/FetchQuest", nil
	}

	if strings.Contains(input, "://") {
		return "", fmt.Errorf("unrecognized URL — paste a Google Drive or Dropbox folder URL, or type a folder path")
	}

	return input, nil
}

func uniqueRemoteName(base string, existing []string) string {
	name := base
	for i := 2; ; i++ {
		found := false
		for _, e := range existing {
			if strings.TrimSuffix(e, ":") == name {
				found = true
				break
			}
		}
		if !found {
			return name
		}
		name = fmt.Sprintf("%s%d", base, i)
	}
}

func uniqueDestName(base string, existing []config.Destination) string {
	name := base
	for i := 2; ; i++ {
		found := false
		for _, d := range existing {
			if d.Name == name {
				found = true
				break
			}
		}
		if !found {
			return name
		}
		name = fmt.Sprintf("%s-%d", base, i)
	}
}

// OpenFolderDialog opens a native folder picker and returns the selected path, or empty if cancelled.
func (a *App) OpenFolderDialog() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("app not ready")
	}
	home, _ := os.UserHomeDir()
	defaultDir := home
	if dir := os.Getenv("HOME"); dir != "" {
		defaultDir = dir
	}
	path, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            "Choose folder for Quest recordings",
		DefaultDirectory: defaultDir,
	})
	if err != nil {
		return "", err
	}
	// Normalize to forward slashes for config (rclone and our code accept both)
	if path != "" && filepath.Separator != '/' {
		path = filepath.ToSlash(path)
	}
	return path, nil
}

// SyncProgress is emitted as a Wails event during sync.
type SyncProgress struct {
	Phase       string `json:"phase"`       // "scan", "pull", or "push"
	File        string `json:"file"`        // current filename
	Current     int    `json:"current"`     // files processed so far
	Total       int    `json:"total"`       // total files to process
	FilePercent int    `json:"filePercent"` // 0-100 progress of current file
}

// Sync runs a full sync (pull from all devices, push to all destinations)
// with progress events emitted to the frontend.
// If skipLocal is true, files are pulled to a temp dir, pushed, then deleted locally.
func (a *App) Sync(skipLocal bool) (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	if len(cfg.Destinations) == 0 {
		return "", fmt.Errorf("no destinations configured — add one in Destinations")
	}

	db, err := manifest.Open(config.ConfigDir())
	if err != nil {
		return "", fmt.Errorf("open manifest: %w", err)
	}
	defer db.Close()

	adbClient := adb.NewClient()
	rc := rclone.NewClient()

	emit := func(p SyncProgress) {
		if a.ctx != nil {
			wailsruntime.EventsEmit(a.ctx, "sync:progress", p)
		}
	}

	// ── Phase 1: Pull ──

	devs, err := adbClient.Devices()
	if err != nil {
		return "", err
	}

	// First scan all devices to find new files
	type pendingFile struct {
		serial    string
		mediaPath string
		info      adb.FileInfo
	}
	var toPull []pendingFile

	for _, d := range devs {
		if !d.IsOnline() {
			continue
		}
		emit(SyncProgress{Phase: "scan", File: d.Model, Current: 0, Total: 0})
		for _, mp := range cfg.MediaPaths {
			files, err := adbClient.ListFilesRecursive(d.Serial, mp)
			if err != nil {
				continue
			}
			for _, f := range files {
				pulled, _ := db.IsPulled(d.Serial, f.Path, f.Size, f.MTime.Unix())
				if !pulled {
					toPull = append(toPull, pendingFile{serial: d.Serial, mediaPath: mp, info: f})
				}
			}
		}
	}

	totalPulled := 0
	syncDir := cfg.ExpandSyncDir()

	// In skip-local mode, pull to a temp dir instead
	var tmpDir string
	pullDir := syncDir
	if skipLocal {
		tmpDir, err = os.MkdirTemp("", "fetchquest-stream-*")
		if err != nil {
			return "", fmt.Errorf("create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)
		pullDir = tmpDir
	}

	for i, pf := range toPull {
		fname := filepath.Base(pf.info.Path)
		emit(SyncProgress{Phase: "pull", File: fname, Current: i + 1, Total: len(toPull), FilePercent: 0})

		mediaType := classifyMediaPath(pf.mediaPath)
		localDir := filepath.Join(pullDir, mediaType)
		if err := os.MkdirAll(localDir, 0o755); err != nil {
			continue
		}
		localPath := filepath.Join(localDir, fname)

		// Run adb pull with file-size progress monitoring
		cmd := exec.Command("adb", "-s", pf.serial, "pull", pf.info.Path, localPath)
		if err := cmd.Start(); err != nil {
			continue
		}
		done := make(chan struct{})
		go func() {
			ticker := time.NewTicker(300 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					if info, err := os.Stat(localPath); err == nil && pf.info.Size > 0 {
						pct := int(info.Size() * 100 / pf.info.Size)
						if pct > 100 {
							pct = 100
						}
						emit(SyncProgress{Phase: "pull", File: fname, Current: i + 1, Total: len(toPull), FilePercent: pct})
					}
				}
			}
		}()
		pullErr := cmd.Wait()
		close(done)
		if pullErr != nil {
			continue
		}

		emit(SyncProgress{Phase: "pull", File: fname, Current: i + 1, Total: len(toPull), FilePercent: 100})
		_ = os.Chtimes(localPath, pf.info.MTime, pf.info.MTime)

		// In skip-local mode, record empty local path (temp file will be cleaned up)
		manifestLocalPath := localPath
		if skipLocal {
			manifestLocalPath = ""
		}
		fileID, err := db.RecordPull(pf.serial, pf.info.Path, manifestLocalPath, pf.info.Size, pf.info.MTime.Unix())
		if err != nil {
			continue
		}

		// In skip-local mode, push immediately after pulling each file
		if skipLocal {
			for _, dest := range cfg.Destinations {
				if rc == nil || !rc.IsReachable(dest.RcloneRemote) {
					continue
				}
				mediaType := classifyMediaPath(pf.mediaPath)
				remoteDest := dest.RcloneRemote
				if !strings.HasSuffix(remoteDest, "/") {
					remoteDest += "/"
				}
				remoteDest += mediaType + "/" + fname

				pushCmd := exec.Command("rclone", "copyto",
					"--stats-one-line", "--stats", "1s",
					"--stats-log-level", "NOTICE", "--log-level", "NOTICE",
					localPath, remoteDest)
				pStderr, _ := pushCmd.StderrPipe()
				if err := pushCmd.Start(); err != nil {
					continue
				}
				var wg gosync.WaitGroup
				wg.Add(1)
				go func(r io.Reader, idx int, total int, name string) {
					defer wg.Done()
					scanner := bufio.NewScanner(r)
					for scanner.Scan() {
						line := scanner.Text()
						if pct := parseRclonePercent(line); pct >= 0 {
							emit(SyncProgress{Phase: "push", File: name, Current: idx, Total: total, FilePercent: pct})
						}
					}
				}(pStderr, i+1, len(toPull), fname)
				wg.Wait()
				if err := pushCmd.Wait(); err != nil {
					continue
				}
				_ = db.RecordDestSync(fileID, dest.Name)
			}
			// Delete temp file after pushing
			os.Remove(localPath)
		}

		totalPulled++
	}

	// ── Phase 2: Push (only in normal mode — skip-local pushes inline above) ──

	totalPushed := 0
	if !skipLocal {
		for _, dest := range cfg.Destinations {
			if rc == nil || !rc.IsReachable(dest.RcloneRemote) {
				continue
			}
			unpushed, err := db.GetUnpushedFiles(dest.Name)
			if err != nil || len(unpushed) == 0 {
				continue
			}

			for i, entry := range unpushed {
				if entry.LocalPath == "" {
					continue
				}
				fname := filepath.Base(entry.LocalPath)
				emit(SyncProgress{Phase: "push", File: fname, Current: i + 1, Total: len(unpushed), FilePercent: 0})

				relPath, err := filepath.Rel(syncDir, entry.LocalPath)
				if err != nil {
					relPath = fname
				}
				remoteDest := dest.RcloneRemote
				if !strings.HasSuffix(remoteDest, "/") {
					remoteDest += "/"
				}
				remoteDest += filepath.ToSlash(relPath)

				// Run rclone with stats as log lines to stderr (not -P which needs a terminal)
				cmd := exec.Command("rclone", "copyto",
					"--stats-one-line", "--stats", "1s",
					"--stats-log-level", "NOTICE", "--log-level", "NOTICE",
					entry.LocalPath, remoteDest)
				stderr, _ := cmd.StderrPipe()
				if err := cmd.Start(); err != nil {
					continue
				}
				// Must drain stderr BEFORE cmd.Wait() per Go docs
				var wg gosync.WaitGroup
				wg.Add(1)
				go func(r io.Reader, idx int, total int, name string) {
					defer wg.Done()
					scanner := bufio.NewScanner(r)
					for scanner.Scan() {
						line := scanner.Text()
						if pct := parseRclonePercent(line); pct >= 0 {
							emit(SyncProgress{Phase: "push", File: name, Current: idx, Total: total, FilePercent: pct})
						}
					}
				}(stderr, i+1, len(unpushed), fname)
				wg.Wait()
				if err := cmd.Wait(); err != nil {
					continue
				}
				emit(SyncProgress{Phase: "push", File: fname, Current: i + 1, Total: len(unpushed), FilePercent: 100})
				_ = db.RecordDestSync(entry.ID, dest.Name)
				totalPushed++
			}
		}
	}

	backupManifest(db, cfg, rc)

	if len(devs) == 0 {
		return "No connected devices found. Plug in your Quest via USB.", nil
	}
	return fmt.Sprintf("Pulled %d new files, uploaded %d to destinations.", totalPulled, totalPushed), nil
}

// scanCR is a bufio.SplitFunc that splits on \r or \n (rclone uses \r for progress).
func scanCR(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i, b := range data {
		if b == '\r' || b == '\n' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// rclonePercentRe matches the percentage in rclone stats-one-line output.
var rclonePercentRe = regexp.MustCompile(`(\d+)%`)

// parseRclonePercent extracts the transfer percentage from an rclone stats line.
func parseRclonePercent(line string) int {
	m := rclonePercentRe.FindStringSubmatch(line)
	if m == nil {
		return -1
	}
	pct, err := strconv.Atoi(m[1])
	if err != nil {
		return -1
	}
	return pct
}

// classifyMediaPath returns a friendly label from a Quest media path.
func classifyMediaPath(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.Contains(lower, "videoshots"):
		return "Videos"
	case strings.Contains(lower, "screenshots"):
		return "Screenshots"
	case strings.Contains(lower, "photos"):
		return "Photos"
	default:
		return "Other"
	}
}

func backupManifest(db *manifest.DB, cfg *config.Config, rc *rclone.Client) {
	if len(cfg.Destinations) == 0 {
		return
	}
	dbPath := db.Path()
	if _, err := os.Stat(dbPath); err != nil {
		return
	}
	for _, dest := range cfg.Destinations {
		remote := dest.RcloneRemote
		if !strings.HasSuffix(remote, "/") {
			remote += "/"
		}
		remote += ".fetchquest/manifest.db"
		_ = rc.Copy(dbPath, remote)
	}
}

// PreviewItem is one device or destination in the preview summary.
type PreviewItem struct {
	Label    string `json:"label"`
	NewFiles int    `json:"newFiles"`
	Pending  int    `json:"pending"`
}

// PreviewResult is the dry-run summary sent to the frontend.
type PreviewResult struct {
	Devices      []PreviewItem `json:"devices"`
	Destinations []PreviewItem `json:"destinations"`
	TotalNew     int           `json:"totalNew"`
	TotalPending int           `json:"totalPending"`
}

// PreviewSync performs a dry run: scans devices for new files and checks
// how many files are waiting to be pushed to each destination.
func (a *App) PreviewSync() (PreviewResult, error) {
	cfg, err := config.Load()
	if err != nil {
		return PreviewResult{}, fmt.Errorf("load config: %w", err)
	}

	db, err := manifest.Open(config.ConfigDir())
	if err != nil {
		return PreviewResult{}, fmt.Errorf("open manifest: %w", err)
	}
	defer db.Close()

	adbClient := adb.NewClient()
	devs, err := adbClient.Devices()
	if err != nil {
		return PreviewResult{}, err
	}

	var result PreviewResult

	// Scan each online device for new (unpulled) files
	for _, d := range devs {
		if !d.IsOnline() {
			continue
		}
		nickname := ""
		if dc, ok := cfg.Devices[d.Serial]; ok && dc.Nickname != "" {
			nickname = dc.Nickname
		}
		label := nickname
		if label == "" {
			label = d.Model
		}
		if label == "" {
			label = d.Serial
		}

		newCount := 0
		for _, mp := range cfg.MediaPaths {
			files, err := adbClient.ListFilesRecursive(d.Serial, mp)
			if err != nil {
				continue
			}
			for _, f := range files {
				pulled, _ := db.IsPulled(d.Serial, f.Path, f.Size, f.MTime.Unix())
				if !pulled {
					newCount++
				}
			}
		}
		result.Devices = append(result.Devices, PreviewItem{Label: label, NewFiles: newCount})
		result.TotalNew += newCount
	}

	// Check unpushed files per destination
	for _, dest := range cfg.Destinations {
		unpushed, err := db.GetUnpushedFiles(dest.Name)
		pending := 0
		if err == nil {
			pending = len(unpushed)
		}
		result.Destinations = append(result.Destinations, PreviewItem{Label: dest.Name, Pending: pending})
		result.TotalPending += pending
	}

	return result, nil
}

// FileEntry is a file for the frontend file browser.
type FileEntry struct {
	FileName   string   `json:"fileName"`
	Path       string   `json:"path"`
	LocalPath  string   `json:"localPath"`
	Size       int64    `json:"size"`
	MTime      int64    `json:"mtime"`
	MediaType  string   `json:"mediaType"`
	IsPulled   bool     `json:"isPulled"`
	SyncedDests []string `json:"syncedDests"`
	TotalDests int      `json:"totalDests"`
}

// DestinationStatus is reachability + sync count for a destination card.
type DestinationStatus struct {
	Name      string `json:"name"`
	Reachable bool   `json:"reachable"`
	FileCount int    `json:"fileCount"`
}

// classifyMedia returns a media type label based on file extension.
func classifyMedia(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp4", ".mkv", ".mov", ".avi", ".webm":
		return "Videos"
	case ".jpg", ".jpeg", ".png", ".webp", ".bmp":
		return "Screenshots"
	default:
		return "Other"
	}
}

// GetDeviceFiles lists files on a connected Quest device with their sync status.
func (a *App) GetDeviceFiles(serial string) ([]FileEntry, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	adbClient := adb.NewClient()
	if serial == "" {
		devs, err := adbClient.Devices()
		if err != nil {
			return nil, err
		}
		for _, d := range devs {
			if d.IsOnline() {
				serial = d.Serial
				break
			}
		}
		if serial == "" {
			return nil, fmt.Errorf("no device connected")
		}
	}

	db, err := manifest.Open(config.ConfigDir())
	if err != nil {
		return nil, fmt.Errorf("open manifest: %w", err)
	}
	defer db.Close()

	var allFiles []adb.FileInfo
	for _, mp := range cfg.MediaPaths {
		files, err := adbClient.ListFilesRecursive(serial, mp)
		if err != nil {
			continue
		}
		allFiles = append(allFiles, files...)
	}

	destNames := make([]string, len(cfg.Destinations))
	for i, d := range cfg.Destinations {
		destNames[i] = d.Name
	}

	var entries []FileEntry
	for _, f := range allFiles {
		pulled, _ := db.IsPulled(serial, f.Path, f.Size, f.MTime.Unix())
		var syncedDests []string
		if pulled {
			// Check which destinations this file has been synced to
			for _, dn := range destNames {
				synced, _ := db.IsFullySynced(serial, f.Path, []string{dn})
				if synced {
					syncedDests = append(syncedDests, dn)
				}
			}
		}
		entries = append(entries, FileEntry{
			FileName:    filepath.Base(f.Path),
			Path:        f.Path,
			Size:        f.Size,
			MTime:       f.MTime.Unix(),
			MediaType:   classifyMedia(f.Path),
			IsPulled:    pulled,
			SyncedDests: syncedDests,
			TotalDests:  len(cfg.Destinations),
		})
	}
	return entries, nil
}

// GetLocalFiles lists files in the local sync directory with their sync status.
func (a *App) GetLocalFiles() ([]FileEntry, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	syncDir := cfg.ExpandSyncDir()
	if _, err := os.Stat(syncDir); os.IsNotExist(err) {
		return []FileEntry{}, nil
	}

	db, err := manifest.Open(config.ConfigDir())
	if err != nil {
		return nil, fmt.Errorf("open manifest: %w", err)
	}
	defer db.Close()

	destNames := make([]string, len(cfg.Destinations))
	for i, d := range cfg.Destinations {
		destNames[i] = d.Name
	}

	var entries []FileEntry
	filepath.Walk(syncDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		// Skip hidden files and manifest
		base := filepath.Base(path)
		if strings.HasPrefix(base, ".") {
			return nil
		}

		relPath, _ := filepath.Rel(syncDir, path)
		mediaType := classifyMedia(path)

		// Look up sync status in manifest by local_path
		var syncedDests []string
		rows, qerr := db.QueryByLocalPath(path)
		if qerr == nil && rows != nil {
			for _, dn := range destNames {
				for _, row := range rows {
					synced, _ := db.IsFullySynced(row.DeviceSerial, row.RemotePath, []string{dn})
					if synced {
						syncedDests = append(syncedDests, dn)
						break
					}
				}
			}
		}

		entries = append(entries, FileEntry{
			FileName:    base,
			Path:        relPath,
			LocalPath:   path,
			Size:        info.Size(),
			MTime:       info.ModTime().Unix(),
			MediaType:   mediaType,
			IsPulled:    true,
			SyncedDests: syncedDests,
			TotalDests:  len(cfg.Destinations),
		})
		return nil
	})
	return entries, nil
}

// CleanPreview returns info about what CleanQuest would delete.
type CleanPreviewResult struct {
	Eligible  int   `json:"eligible"`  // files that would be deleted
	Unsynced  int   `json:"unsynced"`  // files NOT yet fully backed up
	TotalSize int64 `json:"totalSize"` // total bytes of eligible files
}

func (a *App) PreviewClean() (CleanPreviewResult, error) {
	var result CleanPreviewResult
	cfg, err := config.Load()
	if err != nil {
		return result, err
	}
	if len(cfg.Destinations) == 0 {
		return result, fmt.Errorf("no destinations configured")
	}
	db, err := manifest.Open(config.ConfigDir())
	if err != nil {
		return result, err
	}
	defer db.Close()

	adbClient := adb.NewClient()
	devs, err := adbClient.Devices()
	if err != nil {
		return result, err
	}

	destNames := make([]string, len(cfg.Destinations))
	for i, d := range cfg.Destinations {
		destNames[i] = d.Name
	}

	for _, dev := range devs {
		if !dev.IsOnline() {
			continue
		}
		synced, err := db.GetFullySyncedFiles(dev.Serial, destNames)
		if err != nil {
			continue
		}
		result.Eligible += len(synced)
		for _, e := range synced {
			result.TotalSize += e.Size
		}
		stats, err := db.GetDeviceStats(dev.Serial, len(destNames))
		if err != nil {
			continue
		}
		result.Unsynced += stats.TotalFiles - len(synced)
	}
	return result, nil
}

// CleanQuest deletes fully-synced files from connected Quest devices.
// Returns a summary string of what was deleted.
func (a *App) CleanQuest() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	if len(cfg.Destinations) == 0 {
		return "", fmt.Errorf("no destinations configured — nothing is considered fully synced")
	}
	db, err := manifest.Open(config.ConfigDir())
	if err != nil {
		return "", fmt.Errorf("open manifest: %w", err)
	}
	defer db.Close()

	adbClient := adb.NewClient()
	devs, err := adbClient.Devices()
	if err != nil {
		return "", err
	}

	destNames := make([]string, len(cfg.Destinations))
	for i, d := range cfg.Destinations {
		destNames[i] = d.Name
	}

	totalDeleted := 0
	for _, dev := range devs {
		if !dev.IsOnline() {
			continue
		}
		entries, err := db.GetFullySyncedFiles(dev.Serial, destNames)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if err := adbClient.Remove(dev.Serial, e.RemotePath); err != nil {
				continue
			}
			totalDeleted++
		}
	}

	if totalDeleted == 0 {
		return "No files to clean — everything on Quest is either new or not yet fully backed up.", nil
	}
	return fmt.Sprintf("Cleaned %d file%s from Quest.", totalDeleted, pluralS(totalDeleted)), nil
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// OpenSyncFolder opens the sync directory in the OS file manager.
func (a *App) OpenSyncFolder() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	syncDir := cfg.ExpandSyncDir()
	if err := os.MkdirAll(syncDir, 0o755); err != nil {
		return err
	}
	return openInOS(syncDir)
}

// OpenFileInOS opens a file with the default application.
func (a *App) OpenFileInOS(path string) error {
	return openInOS(path)
}

func openInOS(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

// ShowInFolder reveals a file in the OS file manager.
func (a *App) ShowInFolder(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-R", path)
	case "windows":
		cmd = exec.Command("explorer", "/select,", path)
	default:
		cmd = exec.Command("xdg-open", filepath.Dir(path))
	}
	return cmd.Start()
}

// GetDestinationStatuses checks reachability and sync counts for all destinations.
func (a *App) GetDestinationStatuses() ([]DestinationStatus, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	db, err := manifest.Open(config.ConfigDir())
	if err != nil {
		return nil, fmt.Errorf("open manifest: %w", err)
	}
	defer db.Close()

	rc := rclone.NewClient()
	var statuses []DestinationStatus
	for _, d := range cfg.Destinations {
		reachable := false
		if rc != nil {
			reachable = rc.IsReachable(d.RcloneRemote)
		}

		// Count files synced to this destination
		fileCount := 0
		var count int
		err := db.CountSyncedTo(d.Name, &count)
		if err == nil {
			fileCount = count
		}

		statuses = append(statuses, DestinationStatus{
			Name:      d.Name,
			Reachable: reachable,
			FileCount: fileCount,
		})
	}
	return statuses, nil
}

// SetDeviceNickname saves a nickname for a device serial.
func (a *App) SetDeviceNickname(serial, nickname string) error {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return fmt.Errorf("serial is required")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Devices == nil {
		cfg.Devices = make(map[string]config.DeviceConfig)
	}
	dc := cfg.Devices[serial]
	dc.Nickname = strings.TrimSpace(nickname)
	cfg.Devices[serial] = dc
	return config.Save(cfg)
}

func checkDeps() []DepInfo {
	deps := []struct {
		name   string
		binary string
		install map[string]string
	}{
		{"ADB (Android Debug Bridge)", "adb", map[string]string{
			"darwin":  "brew install android-platform-tools",
			"linux":   "sudo apt install android-tools-adb",
			"windows": "winget install Google.PlatformTools",
		}},
		{"rclone", "rclone", map[string]string{
			"darwin":  "brew install rclone",
			"linux":   "curl https://rclone.org/install.sh | sudo bash",
			"windows": "winget install Rclone.Rclone",
		}},
	}

	var missing []DepInfo
	for _, d := range deps {
		if _, err := exec.LookPath(d.binary); err != nil {
			install := d.install[runtime.GOOS]
			if install == "" {
				install = "Install from https://developer.android.com/tools/adb and https://rclone.org"
			}
			missing = append(missing, DepInfo{Name: d.name, Install: install})
		}
	}
	return missing
}
