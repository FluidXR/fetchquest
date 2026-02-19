package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "fetchquest",
	Short: "Sync media from Meta Quest headsets to cloud/NAS",
	Long: `FetchQuest pulls videos, screenshots, and photos from Meta Quest headsets
via ADB, stores them locally, then syncs them to cloud/NAS destinations via rclone.`,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
