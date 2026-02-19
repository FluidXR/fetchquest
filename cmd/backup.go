package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/FluidXR/fetchquest/internal/config"
	"github.com/FluidXR/fetchquest/internal/manifest"
	"github.com/FluidXR/fetchquest/internal/rclone"
)

// backupManifest copies the manifest DB to all configured destinations.
func backupManifest(db *manifest.DB, cfg *config.Config, rc *rclone.Client) {
	if len(cfg.Destinations) == 0 {
		return
	}
	dbPath := db.Path()
	if _, err := os.Stat(dbPath); err != nil {
		return
	}
	fmt.Println("\n=== Backing up manifest ===")
	for _, dest := range cfg.Destinations {
		remote := dest.RcloneRemote
		if !strings.HasSuffix(remote, "/") {
			remote += "/"
		}
		remote += ".fetchquest/manifest.db"
		fmt.Printf("  %s -> %s\n", dest.Name, remote)
		if err := rc.Copy(dbPath, remote); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: manifest backup to %s failed: %v\n", dest.Name, err)
		}
	}
}
