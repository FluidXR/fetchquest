package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FluidXR/fetchquest/internal/adb"
	"github.com/FluidXR/fetchquest/internal/config"
	"github.com/FluidXR/fetchquest/internal/manifest"

	"github.com/spf13/cobra"
)

var (
	cleanDevice  string
	cleanConfirm bool
	cleanDryRun  bool
	cleanAny     bool
	cleanLocal   bool
)

var cleanCmd = &cobra.Command{
	Use:               "clean",
	Short:             "Delete already-synced media from Quest(s)",
	PersistentPreRunE: requireDeps(),
	Long: `Removes files from Quest that have been confirmed synced to all configured destinations.
Use --any to delete files synced to at least one destination instead.
Use --local to clean the local sync directory instead of the Quest.
Shows a dry-run summary first unless --confirm is passed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cleanLocal {
			return cleanLocalDir()
		}
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		if len(cfg.Destinations) == 0 {
			return fmt.Errorf("no destinations configured — nothing is considered fully synced")
		}
		db, err := manifest.Open(config.ConfigDir())
		if err != nil {
			return fmt.Errorf("open manifest: %w", err)
		}
		defer db.Close()

		adbClient := adb.NewClient()
		destNames := make([]string, len(cfg.Destinations))
		for i, d := range cfg.Destinations {
			destNames[i] = d.Name
		}

		var serials []string
		if cleanDevice != "" {
			serials = []string{cleanDevice}
		} else {
			devices, err := adbClient.Devices()
			if err != nil {
				return err
			}
			for _, d := range devices {
				if d.IsOnline() {
					serials = append(serials, d.Serial)
				}
			}
		}

		if len(serials) == 0 {
			fmt.Println("No connected devices found.")
			return nil
		}

		for _, serial := range serials {
			var entries []manifest.Entry
			var err error
			if cleanAny {
				entries, err = db.GetAnySyncedFiles(serial)
			} else {
				entries, err = db.GetFullySyncedFiles(serial, destNames)
			}
			if err != nil {
				return fmt.Errorf("get synced files for %s: %w", serial, err)
			}

			if len(entries) == 0 {
				fmt.Printf("Device %s: no files eligible for cleanup\n", serial)
				continue
			}

			fmt.Printf("\nDevice %s: %d files eligible for cleanup:\n", serial, len(entries))
			for _, e := range entries {
				fmt.Printf("  %s (%d bytes)\n", e.RemotePath, e.Size)
			}

			if cleanDryRun {
				fmt.Println("  (dry run — no files deleted)")
				continue
			}

			if !cleanConfirm {
				fmt.Print("\nDelete these files from the Quest? [y/N] ")
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "y" && answer != "yes" {
					fmt.Println("Skipped.")
					continue
				}
			}

			deleted := 0
			for _, e := range entries {
				if err := adbClient.Remove(serial, e.RemotePath); err != nil {
					fmt.Fprintf(os.Stderr, "  Error deleting %s: %v\n", e.RemotePath, err)
					continue
				}
				deleted++
			}
			fmt.Printf("  Deleted %d files from device %s\n", deleted, serial)
		}
		return nil
	},
}

// isCloudSyncedDir checks if a path is inside a known cloud sync folder.
// Returns the name of the service if detected, or empty string if not.
func isCloudSyncedDir(dir string) string {
	home, _ := os.UserHomeDir()
	absDir, _ := filepath.Abs(dir)

	cloudDirs := []struct {
		path string
		name string
	}{
		{filepath.Join(home, "Dropbox"), "Dropbox"},
		{filepath.Join(home, "Google Drive"), "Google Drive"},
		{filepath.Join(home, "OneDrive"), "OneDrive"},
		{filepath.Join(home, "iCloud Drive"), "iCloud Drive"},
		{filepath.Join(home, "Library", "Mobile Documents"), "iCloud Drive"},
		{filepath.Join(home, "Library", "CloudStorage"), "cloud storage"},
	}

	for _, cd := range cloudDirs {
		if strings.HasPrefix(absDir, cd.path+string(filepath.Separator)) || absDir == cd.path {
			return cd.name
		}
	}
	return ""
}

func cleanLocalDir() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if len(cfg.Destinations) == 0 {
		return fmt.Errorf("no destinations configured — nothing is considered synced")
	}

	db, err := manifest.Open(config.ConfigDir())
	if err != nil {
		return fmt.Errorf("open manifest: %w", err)
	}
	defer db.Close()

	syncDir := cfg.ExpandSyncDir()

	// Check if sync dir is inside a cloud-synced folder
	if service := isCloudSyncedDir(syncDir); service != "" {
		fmt.Fprintf(os.Stderr, "Warning: your sync directory (%s) is inside a %s folder.\n", syncDir, service)
		fmt.Fprintf(os.Stderr, "Deleting local files here will also delete them from %s.\n", service)
		if !cleanConfirm {
			fmt.Fprint(os.Stderr, "Continue anyway? [y/N] ")
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				fmt.Println("Aborted.")
				return nil
			}
		}
	}
	destNames := make([]string, len(cfg.Destinations))
	for i, d := range cfg.Destinations {
		destNames[i] = d.Name
	}

	// Find local files that have been synced to destinations
	entries, err := db.GetLocalSyncedFiles(destNames, cleanAny)
	if err != nil {
		return fmt.Errorf("get synced local files: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No local files eligible for cleanup.")
		return nil
	}

	fmt.Printf("\n%d local files eligible for cleanup:\n", len(entries))
	for _, e := range entries {
		fmt.Printf("  %s (%d bytes)\n", e.LocalPath, e.Size)
	}

	if cleanDryRun {
		fmt.Println("  (dry run — no files deleted)")
		return nil
	}

	if !cleanConfirm {
		fmt.Print("\nDelete these local files? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Skipped.")
			return nil
		}
	}

	deleted := 0
	for _, e := range entries {
		if err := os.Remove(e.LocalPath); err != nil {
			if os.IsNotExist(err) {
				deleted++
				continue
			}
			fmt.Fprintf(os.Stderr, "  Error deleting %s: %v\n", e.LocalPath, err)
			continue
		}
		deleted++
	}
	fmt.Printf("  Deleted %d local files\n", deleted)

	// Clean up empty directories in sync dir
	filepath.Walk(syncDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() || path == syncDir {
			return nil
		}
		// Try to remove — will only succeed if empty
		os.Remove(path)
		return nil
	})

	return nil
}

func init() {
	cleanCmd.Flags().StringVarP(&cleanDevice, "device", "d", "", "Device serial (default: all)")
	cleanCmd.Flags().BoolVar(&cleanConfirm, "confirm", false, "Skip confirmation prompt")
	cleanCmd.Flags().BoolVar(&cleanDryRun, "dry-run", false, "Show what would be deleted without deleting")
	cleanCmd.Flags().BoolVar(&cleanAny, "any", false, "Delete files synced to at least one destination (default: all)")
	cleanCmd.Flags().BoolVar(&cleanLocal, "local", false, "Clean local sync directory instead of Quest")
	rootCmd.AddCommand(cleanCmd)
}
