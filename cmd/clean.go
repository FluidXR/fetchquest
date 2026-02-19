package cmd

import (
	"bufio"
	"fmt"
	"os"
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
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Delete already-synced media from Quest(s)",
	Long: `Removes files from Quest that have been confirmed synced to ALL configured destinations.
Shows a dry-run summary first unless --confirm is passed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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
			entries, err := db.GetFullySyncedFiles(serial, destNames)
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

func init() {
	cleanCmd.Flags().StringVarP(&cleanDevice, "device", "d", "", "Device serial (default: all)")
	cleanCmd.Flags().BoolVar(&cleanConfirm, "confirm", false, "Skip confirmation prompt")
	cleanCmd.Flags().BoolVar(&cleanDryRun, "dry-run", false, "Show what would be deleted without deleting")
	rootCmd.AddCommand(cleanCmd)
}
