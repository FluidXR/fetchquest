package cmd

import (
	"fmt"
	"os"

	"github.com/FluidXR/fetchquest/internal/adb"
	"github.com/FluidXR/fetchquest/internal/config"
	"github.com/FluidXR/fetchquest/internal/manifest"
	"github.com/FluidXR/fetchquest/internal/sync"

	"github.com/spf13/cobra"
)

var pullDevice string

var pullCmd = &cobra.Command{
	Use:               "pull",
	Short:             "Pull media from Quest(s) to local sync directory",
	PersistentPreRunE: requireDeps(),
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

		puller := &sync.Puller{
			ADB:      adb.NewClient(),
			Manifest: db,
			Config:   cfg,
		}

		if pullDevice != "" {
			fmt.Printf("Pulling media from device %s...\n", pullDevice)
			result, err := puller.PullDevice(pullDevice)
			if err != nil {
				return err
			}
			printPullResult(result)
			return nil
		}

		fmt.Println("Pulling media from all connected devices...")
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
		return nil
	},
}

func printPullResult(r sync.PullResult) {
	fmt.Printf("\nDevice: %s\n", r.DeviceSerial)
	fmt.Printf("  Pulled: %d files\n", r.FilesPulled)
	fmt.Printf("  Skipped: %d files (already synced)\n", r.FilesSkipped)
	for _, e := range r.Errors {
		fmt.Fprintf(os.Stderr, "  Error: %s\n", e)
	}
}

func init() {
	pullCmd.Flags().StringVarP(&pullDevice, "device", "d", "", "Device serial to pull from (default: all)")
	rootCmd.AddCommand(pullCmd)
}
