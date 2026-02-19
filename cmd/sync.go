package cmd

import (
	"fmt"
	"os"

	"github.com/FluidXR/fetchquest/internal/adb"
	"github.com/FluidXR/fetchquest/internal/config"
	"github.com/FluidXR/fetchquest/internal/manifest"
	"github.com/FluidXR/fetchquest/internal/rclone"
	qsync "github.com/FluidXR/fetchquest/internal/sync"

	"github.com/spf13/cobra"
)

var (
	syncDevice    string
	syncSkipLocal bool
)

var syncCmd = &cobra.Command{
	Use:               "sync",
	Short:             "Pull media from Quest(s) then push to destination(s)",
	PersistentPreRunE: requireDeps(),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		if len(cfg.Destinations) == 0 {
			return fmt.Errorf("no destinations configured — run 'fetchquest config add-dest' first")
		}
		db, err := manifest.Open(config.ConfigDir())
		if err != nil {
			return fmt.Errorf("open manifest: %w", err)
		}
		defer db.Close()

		rc := rclone.NewClient()

		if syncSkipLocal {
			// Stream mode: pull and sync one file at a time, no local copies
			streamer := &qsync.Streamer{
				ADB:       adb.NewClient(),
				Rclone:    rc,
				Manifest:  db,
				Config:    cfg,
				SkipLocal: true,
			}

			if syncDevice != "" {
				fmt.Printf("Syncing media from device %s (skip-local)...\n", syncDevice)
				result, err := streamer.StreamDevice(syncDevice)
				if err != nil {
					return err
				}
				printStreamResult(result)
			} else {
				fmt.Println("Syncing media from all connected devices (skip-local)...")
				results, err := streamer.StreamAll()
				if err != nil {
					return err
				}
				for _, r := range results {
					printStreamResult(r)
				}
				if len(results) == 0 {
					fmt.Println("No connected devices found.")
				}
			}
		} else {
			// Normal mode: pull all, then push all
			puller := &qsync.Puller{
				ADB:      adb.NewClient(),
				Manifest: db,
				Config:   cfg,
			}

			fmt.Println("=== Pull Phase ===")
			if syncDevice != "" {
				result, err := puller.PullDevice(syncDevice)
				if err != nil {
					return err
				}
				printPullResult(result)
			} else {
				results, err := puller.PullAll()
				if err != nil {
					return err
				}
				for _, r := range results {
					printPullResult(r)
				}
				if len(results) == 0 {
					fmt.Println("No connected devices found.")
				}
			}

			pusher := &qsync.Pusher{
				Rclone:   rc,
				Manifest: db,
				Config:   cfg,
			}

			fmt.Println("\n=== Push Phase ===")
			pushResults, err := pusher.PushAll()
			if err != nil {
				return err
			}
			for _, r := range pushResults {
				fmt.Printf("\nDestination: %s\n", r.Destination)
				fmt.Printf("  Pushed: %d files\n", r.FilesPushed)
				fmt.Printf("  Skipped: %d files\n", r.FilesSkipped)
				for _, e := range r.Errors {
					fmt.Fprintf(os.Stderr, "  Error: %s\n", e)
				}
			}
		}

		backupManifest(db, cfg, rc)
		return nil
	},
}

func printStreamResult(r qsync.StreamResult) {
	fmt.Printf("\nDevice: %s\n", r.DeviceSerial)
	fmt.Printf("  Synced: %d files\n", r.FilesStreamed)
	fmt.Printf("  Skipped: %d files (already synced)\n", r.FilesSkipped)
	for _, e := range r.Errors {
		fmt.Fprintf(os.Stderr, "  Error: %s\n", e)
	}
}

func init() {
	syncCmd.Flags().StringVarP(&syncDevice, "device", "d", "", "Device serial (default: all)")
	syncCmd.Flags().BoolVar(&syncSkipLocal, "skip-local", false, "Don't keep local copies — sync straight to destinations")
	rootCmd.AddCommand(syncCmd)
}
