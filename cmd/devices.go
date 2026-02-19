package cmd

import (
	"fmt"

	"github.com/FluidXR/fetchquest/internal/adb"
	"github.com/FluidXR/fetchquest/internal/config"
	"github.com/FluidXR/fetchquest/internal/manifest"

	"github.com/spf13/cobra"
)

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List connected Quests and their sync status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		adbClient := adb.NewClient()
		devices, err := adbClient.Devices()
		if err != nil {
			return err
		}

		if len(devices) == 0 {
			fmt.Println("No devices connected.")
			return nil
		}

		db, err := manifest.Open(config.ConfigDir())
		if err != nil {
			return fmt.Errorf("open manifest: %w", err)
		}
		defer db.Close()

		numDests := len(cfg.Destinations)

		for _, d := range devices {
			nickname := ""
			if dc, ok := cfg.Devices[d.Serial]; ok && dc.Nickname != "" {
				nickname = fmt.Sprintf(" (%s)", dc.Nickname)
			}

			status := d.State
			if !d.IsOnline() {
				status = "OFFLINE"
			}

			fmt.Printf("%-20s %s  [%s] [%s]%s\n",
				d.Serial, d.Model, d.ConnType, status, nickname)

			if d.IsOnline() {
				stats, err := db.GetDeviceStats(d.Serial, numDests)
				if err == nil && stats.TotalFiles > 0 {
					fmt.Printf("  Files tracked: %d | Pulled: %d | Fully synced: %d\n",
						stats.TotalFiles, stats.PulledFiles, stats.SyncedFiles)
				}
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(devicesCmd)
}
