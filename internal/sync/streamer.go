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
	ADB      *adb.Client
	Rclone   *rclone.Client
	Manifest *manifest.DB
	Config   *config.Config
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

	// Use a temp directory for streaming — files are deleted after sync
	tmpDir, err := os.MkdirTemp("", "fetchquest-stream-*")
	if err != nil {
		return result, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	pusher := &Pusher{
		Rclone:   s.Rclone,
		Manifest: s.Manifest,
		Config:   s.Config,
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

			// Pull to temp location
			mediaType := mediaTypeFromPath(mediaPath)
			localDir := filepath.Join(tmpDir, mediaType)
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

			// Record in manifest (no local_path since stream deletes it after sync)
			fileID, err := s.Manifest.RecordPull(serial, f.Path, "", f.Size, f.MTime.Unix())
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("record %s: %v", f.Path, err))
				continue
			}

			// Push to all destinations
			fmt.Printf("  [stream] Pushing %s to all destinations\n", filepath.Base(f.Path))
			pushResults, err := pusher.PushFile(fileID, localPath, tmpDir)
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

			// Delete local copy if all destinations succeeded
			if allPushed {
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
