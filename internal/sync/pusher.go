package sync

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/FluidXR/fetchquest/internal/config"
	"github.com/FluidXR/fetchquest/internal/manifest"
	"github.com/FluidXR/fetchquest/internal/rclone"
)

// Pusher handles uploading local media to rclone destinations.
type Pusher struct {
	Rclone   *rclone.Client
	Manifest *manifest.DB
	Config   *config.Config
}

// PushResult summarizes a push operation.
type PushResult struct {
	Destination  string
	FilesPushed  int
	FilesSkipped int
	Errors       []string
}

// PushAll uploads unpushed files to all configured destinations.
func (p *Pusher) PushAll() ([]PushResult, error) {
	var results []PushResult
	for _, dest := range p.Config.Destinations {
		fmt.Printf("Checking %s... ", dest.Name)
		if !p.Rclone.IsReachable(dest.RcloneRemote) {
			fmt.Printf("unreachable, skipping\n")
			results = append(results, PushResult{
				Destination: dest.Name,
				Errors:      []string{"destination unreachable"},
			})
			continue
		}
		fmt.Printf("ok\n")
		r, err := p.PushToDest(dest)
		if err != nil {
			results = append(results, PushResult{
				Destination: dest.Name,
				Errors:      []string{err.Error()},
			})
			continue
		}
		results = append(results, r)
	}
	return results, nil
}

// PushToDest uploads unpushed files to a specific destination.
func (p *Pusher) PushToDest(dest config.Destination) (PushResult, error) {
	result := PushResult{Destination: dest.Name}

	entries, err := p.Manifest.GetUnpushedFiles(dest.Name)
	if err != nil {
		return result, err
	}

	syncDir := p.Config.ExpandSyncDir()
	for _, entry := range entries {
		if entry.LocalPath == "" {
			result.FilesSkipped++
			continue
		}

		// Build rclone dest path: remote:path/serial/MediaType/filename
		relPath, err := filepath.Rel(syncDir, entry.LocalPath)
		if err != nil {
			relPath = filepath.Base(entry.LocalPath)
		}
		remoteDest := dest.RcloneRemote
		if !strings.HasSuffix(remoteDest, "/") {
			remoteDest += "/"
		}
		remoteDest += filepath.ToSlash(relPath)

		fmt.Printf("  Uploading %s -> %s\n", entry.LocalPath, remoteDest)
		if err := p.Rclone.Copy(entry.LocalPath, remoteDest); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("push %s: %v", entry.LocalPath, err))
			continue
		}

		if err := p.Manifest.RecordDestSync(entry.ID, dest.Name); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("record sync %s: %v", entry.LocalPath, err))
			continue
		}
		result.FilesPushed++
	}
	return result, nil
}

// PushFile uploads a single file to all destinations and records it.
// If baseDir is empty, the config's sync dir is used to compute relative paths.
func (p *Pusher) PushFile(fileID int64, localPath, baseDir string) ([]PushResult, error) {
	var results []PushResult
	syncDir := baseDir
	if syncDir == "" {
		syncDir = p.Config.ExpandSyncDir()
	}

	for _, dest := range p.Config.Destinations {
		result := PushResult{Destination: dest.Name}

		relPath, err := filepath.Rel(syncDir, localPath)
		if err != nil {
			relPath = filepath.Base(localPath)
		}
		remoteDest := dest.RcloneRemote
		if !strings.HasSuffix(remoteDest, "/") {
			remoteDest += "/"
		}
		remoteDest += filepath.ToSlash(relPath)

		fmt.Printf("  Uploading %s -> %s\n", localPath, remoteDest)
		if err := p.Rclone.Copy(localPath, remoteDest); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("push %s: %v", localPath, err))
			results = append(results, result)
			continue
		}

		if err := p.Manifest.RecordDestSync(fileID, dest.Name); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("record sync %s: %v", localPath, err))
		}
		result.FilesPushed = 1
		results = append(results, result)
	}
	return results, nil
}
