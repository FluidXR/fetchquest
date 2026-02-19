package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FluidXR/fetchquest/internal/config"
	"github.com/FluidXR/fetchquest/internal/rclone"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage fetchquest configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		fmt.Printf("Config file: %s\n\n", config.ConfigPath())
		fmt.Printf("Sync directory: %s\n", cfg.SyncDir)
		fmt.Printf("Media paths:\n")
		for _, p := range cfg.MediaPaths {
			fmt.Printf("  - %s\n", p)
		}
		fmt.Printf("\nDestinations:\n")
		if len(cfg.Destinations) == 0 {
			fmt.Println("  (none configured)")
		}
		for _, d := range cfg.Destinations {
			fmt.Printf("  - %s: %s\n", d.Name, d.RcloneRemote)
		}
		fmt.Printf("\nDevices:\n")
		if len(cfg.Devices) == 0 {
			fmt.Println("  (none configured)")
		}
		for serial, dc := range cfg.Devices {
			fmt.Printf("  - %s", serial)
			if dc.Nickname != "" {
				fmt.Printf(" (%s)", dc.Nickname)
			}
			if dc.WiFiIP != "" {
				fmt.Printf(" [wifi: %s]", dc.WiFiIP)
			}
			fmt.Println()
		}
		return nil
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create default config file",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.DefaultConfig()
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("Config created at %s\n", config.ConfigPath())
		return nil
	},
}

var configNicknameCmd = &cobra.Command{
	Use:   "nickname <serial> <name>",
	Short: "Set a nickname for a device",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		serial := args[0]
		name := args[1]

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		dc := cfg.Devices[serial]
		dc.Nickname = name
		cfg.Devices[serial] = dc
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("Set nickname for %s: %s\n", serial, name)
		return nil
	},
}

var configAddDestCmd = &cobra.Command{
	Use:   "add-dest <name> <rclone_remote>",
	Short: "Add an rclone destination",
	Long:  `Example: fetchquest config add-dest google-drive "gdrive:QuestMedia"`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		remote := args[1]

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		// Check for duplicate
		for _, d := range cfg.Destinations {
			if d.Name == name {
				return fmt.Errorf("destination %q already exists", name)
			}
		}
		cfg.Destinations = append(cfg.Destinations, config.Destination{
			Name:         name,
			RcloneRemote: remote,
		})
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("Added destination: %s -> %s\n", name, remote)
		return nil
	},
}

var configRemoveDestCmd = &cobra.Command{
	Use:   "remove-dest <name>",
	Short: "Remove an rclone destination",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		found := false
		var remaining []config.Destination
		for _, d := range cfg.Destinations {
			if d.Name == name {
				found = true
				continue
			}
			remaining = append(remaining, d)
		}
		if !found {
			return fmt.Errorf("destination %q not found", name)
		}
		cfg.Destinations = remaining
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("Removed destination: %s\n", name)
		return nil
	},
}

var configSetWiFiCmd = &cobra.Command{
	Use:   "set-wifi <serial> <ip>",
	Short: "Set WiFi IP for a device (for wireless ADB)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		serial := args[0]
		ip := args[1]

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		dc := cfg.Devices[serial]
		dc.WiFiIP = ip
		cfg.Devices[serial] = dc
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("Set WiFi IP for %s: %s\n", serial, ip)
		return nil
	},
}

var configRestoreCmd = &cobra.Command{
	Use:   "restore [destination-name]",
	Short: "Restore manifest DB from a backup on a destination",
	Long: `Downloads the manifest DB backup from a configured rclone destination.
If no destination is specified, tries each one until a backup is found.

Example: fetchquest config restore my-nas`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if len(cfg.Destinations) == 0 {
			return fmt.Errorf("no destinations configured — add one with 'fetchquest config add-dest' first")
		}

		rc := rclone.NewClient()
		configDir := config.ConfigDir()
		localDB := filepath.Join(configDir, "manifest.db")

		// Check if local manifest already exists
		if _, err := os.Stat(localDB); err == nil {
			fmt.Printf("Warning: local manifest already exists at %s\n", localDB)
			fmt.Print("Overwrite? [y/N] ")
			var answer string
			fmt.Scanln(&answer)
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		// Determine which destinations to try
		var dests []config.Destination
		if len(args) > 0 {
			name := args[0]
			for _, d := range cfg.Destinations {
				if d.Name == name {
					dests = append(dests, d)
					break
				}
			}
			if len(dests) == 0 {
				return fmt.Errorf("destination %q not found in config", name)
			}
		} else {
			dests = cfg.Destinations
		}

		if err := os.MkdirAll(configDir, 0o755); err != nil {
			return fmt.Errorf("create config dir: %w", err)
		}

		for _, dest := range dests {
			remote := dest.RcloneRemote
			if !strings.HasSuffix(remote, "/") {
				remote += "/"
			}
			remote += ".fetchquest/manifest.db"

			fmt.Printf("Trying %s (%s)...\n", dest.Name, remote)
			if err := rc.CopyFrom(remote, localDB); err != nil {
				fmt.Printf("  Not found or failed: %v\n", err)
				continue
			}
			fmt.Printf("Manifest restored from %s to %s\n", dest.Name, localDB)
			return nil
		}

		return fmt.Errorf("no manifest backup found on any destination")
	},
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configNicknameCmd)
	configCmd.AddCommand(configAddDestCmd)
	configCmd.AddCommand(configRemoveDestCmd)
	configCmd.AddCommand(configSetWiFiCmd)
	configCmd.AddCommand(configRestoreCmd)
	rootCmd.AddCommand(configCmd)
}
