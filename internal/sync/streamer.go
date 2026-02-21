package sync

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/FluidXR/fetchquest/internal/adb"
	"github.com/FluidXR/fetchquest/internal/config"
	"github.com/FluidXR/fetchquest/internal/manifest"
	"github.com/FluidXR/fetchquest/internal/rclone"
)

// Streamer handles one-file-at-a-time streaming mode.
type Streamer struct {
	ADB       *adb.Client
	Rclone    *rclone.Client
	Manifest  *manifest.DB
	Config    *config.Config
	SkipLocal bool
}

// StreamResult summarizes a stream operation.
type StreamResult struct {
	DeviceSerial string
	FilesStreamed int
	FilesSkipped int
	Errors       []string
}

// StreamAll streams files from all connected devices.
func (s *Streamer) StreamAll() ([]StreamResult, error) {
	devices, err := s.ADB.Devices()
	if err != nil {
		return nil, err
	}
	var results []StreamResult
	for _, d := range devices {
		if !d.IsOnline() {
			continue
		}
		r, err := s.StreamDevice(d.Serial)
		if err != nil {
			results = append(results, StreamResult{
				DeviceSerial: d.Serial,
				Errors:       []string{err.Error()},
			})
			continue
		}
		results = append(results, r)
	}
	return results, nil
}

// StreamDevice streams files from a specific device, one at a time.
func (s *Streamer) StreamDevice(serial string) (StreamResult, error) {
	result := StreamResult{DeviceSerial: serial}

	// Pre-check which destinations are reachable
	var reachableDests []config.Destination
	for _, dest := range s.Config.Destinations {
		fmt.Printf("Checking %s... ", dest.Name)
		if s.Rclone.IsReachable(dest.RcloneRemote) {
			fmt.Printf("ok\n")
			reachableDests = append(reachableDests, dest)
		} else {
			fmt.Printf("unreachable, skipping\n")
		}
	}
	if len(reachableDests) == 0 {
		return result, fmt.Errorf("no destinations are reachable")
	}

	var baseDir string
	var cleanupDir string
	if s.SkipLocal {
		tmpDir, err := os.MkdirTemp("", "fetchquest-stream-*")
		if err != nil {
			return result, fmt.Errorf("create temp dir: %w", err)
		}
		cleanupDir = tmpDir
		baseDir = tmpDir
	} else {
		baseDir = s.Config.ExpandSyncDir()
	}
	if cleanupDir != "" {
		defer os.RemoveAll(cleanupDir)
	}

	// Use only reachable destinations for pushing
	streamConfig := *s.Config
	streamConfig.Destinations = reachableDests
	pusher := &Pusher{
		Rclone:   s.Rclone,
		Manifest: s.Manifest,
		Config:   &streamConfig,
	}

	for _, mediaPath := range s.Config.MediaPaths {
		files, err := s.ADB.ListFilesRecursive(serial, mediaPath)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("list %s: %v", mediaPath, err))
			continue
		}

		for _, f := range files {
			pulled, err := s.Manifest.IsPulled(serial, f.Path, f.Size, f.MTime.Unix())
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("check %s: %v", f.Path, err))
				continue
			}
			if pulled {
				result.FilesSkipped++
				continue
			}

			// Pull to local location
			mediaType := mediaTypeFromPath(mediaPath)
			localDir := filepath.Join(baseDir, mediaType)
			if err := os.MkdirAll(localDir, 0o755); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("mkdir %s: %v", localDir, err))
				continue
			}
			localPath := filepath.Join(localDir, filepath.Base(f.Path))

			fmt.Printf("  [stream] Pulling %s\n", f.Path)
			if err := s.ADB.Pull(serial, f.Path, localPath); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("pull %s: %v", f.Path, err))
				continue
			}

			// Preserve original modification time from Quest
			if err := os.Chtimes(localPath, f.MTime, f.MTime); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("chtimes %s: %v", localPath, err))
			}

			// Record in manifest
			manifestLocalPath := localPath
			if s.SkipLocal {
				manifestLocalPath = "" // temp file will be deleted
			}
			fileID, err := s.Manifest.RecordPull(serial, f.Path, manifestLocalPath, f.Size, f.MTime.Unix())
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("record %s: %v", f.Path, err))
				continue
			}

			// Push to all destinations
			fmt.Printf("  [stream] Pushing %s to all destinations\n", filepath.Base(f.Path))
			pushResults, err := pusher.PushFile(fileID, localPath, baseDir)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("push %s: %v", f.Path, err))
				continue
			}

			// Check if all pushes succeeded
			allPushed := true
			for _, pr := range pushResults {
				if len(pr.Errors) > 0 {
					allPushed = false
					result.Errors = append(result.Errors, pr.Errors...)
				}
			}

			// Delete local copy if skipping local and all destinations succeeded
			if s.SkipLocal && allPushed {
				if err := os.Remove(localPath); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("delete local %s: %v", localPath, err))
				} else {
					fmt.Printf("  [stream] Deleted local copy: %s\n", localPath)
				}
			}
			result.FilesStreamed++
		}
	}
	return result, nil
}
