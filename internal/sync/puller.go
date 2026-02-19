package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FluidXR/fetchquest/internal/adb"
	"github.com/FluidXR/fetchquest/internal/config"
	"github.com/FluidXR/fetchquest/internal/manifest"
)

// Puller handles pulling media from Quest devices.
type Puller struct {
	ADB      *adb.Client
	Manifest *manifest.DB
	Config   *config.Config
}

// PullResult summarizes a pull operation.
type PullResult struct {
	DeviceSerial string
	FilesPulled  int
	FilesSkipped int
	Errors       []string
}

// PullAll pulls media from all connected devices.
func (p *Puller) PullAll() ([]PullResult, error) {
	devices, err := p.ADB.Devices()
	if err != nil {
		return nil, err
	}
	var results []PullResult
	for _, d := range devices {
		if !d.IsOnline() {
			continue
		}
		r, err := p.PullDevice(d.Serial)
		if err != nil {
			results = append(results, PullResult{
				DeviceSerial: d.Serial,
				Errors:       []string{err.Error()},
			})
			continue
		}
		results = append(results, r)
	}
	return results, nil
}

// PullDevice pulls media from a specific device.
func (p *Puller) PullDevice(serial string) (PullResult, error) {
	result := PullResult{DeviceSerial: serial}
	syncDir := p.Config.ExpandSyncDir()

	for _, mediaPath := range p.Config.MediaPaths {
		files, err := p.ADB.ListFilesRecursive(serial, mediaPath)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("list %s: %v", mediaPath, err))
			continue
		}

		for _, f := range files {
			pulled, err := p.Manifest.IsPulled(serial, f.Path, f.Size, f.MTime.Unix())
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("check %s: %v", f.Path, err))
				continue
			}
			if pulled {
				result.FilesSkipped++
				continue
			}

			// Determine local path: sync_dir/MediaType/filename
			mediaType := mediaTypeFromPath(mediaPath)
			localDir := filepath.Join(syncDir, mediaType)
			if err := os.MkdirAll(localDir, 0o755); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("mkdir %s: %v", localDir, err))
				continue
			}
			localPath := filepath.Join(localDir, filepath.Base(f.Path))

			fmt.Printf("  Pulling %s -> %s\n", f.Path, localPath)
			if err := p.ADB.Pull(serial, f.Path, localPath); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("pull %s: %v", f.Path, err))
				continue
			}

			// Preserve original modification time from Quest
			if err := os.Chtimes(localPath, f.MTime, f.MTime); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("chtimes %s: %v", localPath, err))
			}

			if _, err := p.Manifest.RecordPull(serial, f.Path, localPath, f.Size, f.MTime.Unix()); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("record %s: %v", f.Path, err))
				continue
			}
			result.FilesPulled++
		}
	}
	return result, nil
}

// mediaTypeFromPath returns a friendly name based on the Quest media path.
func mediaTypeFromPath(path string) string {
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
