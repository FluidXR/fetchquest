package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version of FetchQuest.
const Version = "0.4.0"

var rootCmd = &cobra.Command{
	Use:     "fetchquest",
	Short:   "Sync media from Meta Quest headsets to cloud/NAS",
	Version: Version,
	Long: `FetchQuest pulls videos, screenshots, and photos from Meta Quest headsets
via ADB, stores them locally, then syncs them to cloud/NAS destinations via rclone.`,
}

// requireDeps returns a PersistentPreRunE that checks for external dependencies
// and prompts to nickname any new devices.
func requireDeps() func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := checkDeps(); err != nil {
			return err
		}
		checkNewDevices()
		return nil
	}
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
