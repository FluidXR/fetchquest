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

var syncDevice string

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Pull media from Quest(s) then push to destination(s)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		db, err := manifest.Open(config.ConfigDir())
		if err != nil {
			return fmt.Errorf("open manifest: %w", err)
		}
		defer db.Close()

		// Pull
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

		// Push
		if len(cfg.Destinations) == 0 {
			fmt.Println("\nNo destinations configured — skipping push.")
			return nil
		}

		rc := rclone.NewClient()
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

		backupManifest(db, cfg, rc)
		return nil
	},
}

func init() {
	syncCmd.Flags().StringVarP(&syncDevice, "device", "d", "", "Device serial (default: all)")
	rootCmd.AddCommand(syncCmd)
}
