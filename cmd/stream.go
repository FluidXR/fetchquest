package cmd

import (
	"fmt"
	"os"

	"github.com/FluidXR/fetchquest/internal/adb"
	"github.com/FluidXR/fetchquest/internal/config"
	"github.com/FluidXR/fetchquest/internal/manifest"
	"github.com/FluidXR/fetchquest/internal/rclone"
	"github.com/FluidXR/fetchquest/internal/sync"

	"github.com/spf13/cobra"
)

var streamDevice string

var streamCmd = &cobra.Command{
	Use:               "stream",
	Short:             "One-file-at-a-time: pull -> push -> delete local copy",
	PersistentPreRunE: requireDeps(),
	Long: `Streaming mode for machines with limited disk space.
Pulls one file at a time from Quest, uploads to all destinations,
then deletes the local copy before moving to the next file.`,
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
		streamer := &sync.Streamer{
			ADB:      adb.NewClient(),
			Rclone:   rc,
			Manifest: db,
			Config:   cfg,
		}

		if streamDevice != "" {
			fmt.Printf("Streaming media from device %s...\n", streamDevice)
			result, err := streamer.StreamDevice(streamDevice)
			if err != nil {
				return err
			}
			printStreamResult(result)
			backupManifest(db, cfg, rc)
			return nil
		}

		fmt.Println("Streaming media from all connected devices...")
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

		backupManifest(db, cfg, rc)
		return nil
	},
}

func printStreamResult(r sync.StreamResult) {
	fmt.Printf("\nDevice: %s\n", r.DeviceSerial)
	fmt.Printf("  Streamed: %d files\n", r.FilesStreamed)
	fmt.Printf("  Skipped: %d files (already synced)\n", r.FilesSkipped)
	for _, e := range r.Errors {
		fmt.Fprintf(os.Stderr, "  Error: %s\n", e)
	}
}

func init() {
	streamCmd.Flags().StringVarP(&streamDevice, "device", "d", "", "Device serial (default: all)")
	rootCmd.AddCommand(streamCmd)
}
