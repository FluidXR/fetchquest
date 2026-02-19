package cmd

import (
	"fmt"
	"os"

	"github.com/FluidXR/fetchquest/internal/config"
	"github.com/FluidXR/fetchquest/internal/manifest"
	"github.com/FluidXR/fetchquest/internal/rclone"
	"github.com/FluidXR/fetchquest/internal/sync"

	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Upload local media to rclone destination(s)",
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
		pusher := &sync.Pusher{
			Rclone:   rc,
			Manifest: db,
			Config:   cfg,
		}

		fmt.Println("Pushing media to all destinations...")
		results, err := pusher.PushAll()
		if err != nil {
			return err
		}
		for _, r := range results {
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
	rootCmd.AddCommand(pushCmd)
}
