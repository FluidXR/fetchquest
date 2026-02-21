package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

type destPreset struct {
	name        string
	rcloneType  string
	description string
	remoteName  string
	extraSetup  func(reader *bufio.Reader, remoteName string) error
}

var destPresets = []destPreset{
	{
		name:        "Google Drive",
		rcloneType:  "drive",
		description: "Sync to your Google Drive account",
		remoteName:  "gdrive",
	},
	{
		name:        "Dropbox",
		rcloneType:  "dropbox",
		description: "Sync to your Dropbox account",
		remoteName:  "dropbox",
	},
	{
		name:        "SMB / NAS",
		rcloneType:  "smb",
		description: "Sync to a network drive (NAS, Windows share, etc.)",
		remoteName:  "nas",
		extraSetup: func(reader *bufio.Reader, remoteName string) error {
			fmt.Print("  Server IP or hostname: ")
			host, _ := reader.ReadString('\n')
			host = strings.TrimSpace(host)
			if host == "" {
				return fmt.Errorf("host is required")
			}

			fmt.Print("  Username: ")
			user, _ := reader.ReadString('\n')
			user = strings.TrimSpace(user)

			// Create the remote with host and user
			args := []string{"config", "create", remoteName, "smb", "host", host}
			if user != "" {
				args = append(args, "user", user)
			}
			out, err := exec.Command("rclone", args...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("rclone config create: %w\n%s", err, out)
			}

			if user != "" {
				fmt.Print("  Password: ")
				pass, _ := reader.ReadString('\n')
				pass = strings.TrimSpace(pass)
				if pass != "" {
					out, err := exec.Command("rclone", "config", "password", remoteName, "pass", pass).CombinedOutput()
					if err != nil {
						return fmt.Errorf("rclone config password: %w\n%s", err, out)
					}
				}
			}
			return nil
		},
	},
	{
		name:        "Amazon S3",
		rcloneType:  "s3",
		description: "Sync to an S3-compatible bucket",
		remoteName:  "s3",
	},
}

// browseRemoteFolder lets the user interactively navigate folders on an rclone remote.
func browseRemoteFolder(reader *bufio.Reader, remoteName, startPath string) (string, error) {
	currentPath := startPath

	for {
		remote := remoteName + ":" + currentPath

		// List directories at current path
		out, err := exec.Command("rclone", "lsd", remote).CombinedOutput()
		dirs := []string{}
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				// rclone lsd format: "          -1 2025-01-01 00:00:00        -1 dirname"
				// The directory name is the last field
				parts := strings.Fields(line)
				if len(parts) >= 5 {
					dirName := strings.Join(parts[4:], " ")
					dirs = append(dirs, dirName)
				}
			}
		}

		// Display current location and options
		if currentPath == "" {
			fmt.Printf("\n  📂 / (root)\n")
		} else {
			fmt.Printf("\n  📂 /%s\n", currentPath)
		}
		fmt.Println()

		optNum := 1
		fmt.Printf("  %d. ✅ Use this folder\n", optNum)
		optNum++
		fmt.Printf("  %d. 📁 Create new folder here\n", optNum)
		optNum++

		if currentPath != "" {
			fmt.Printf("  %d. ⬆️  Go up\n", optNum)
			optNum++
		}

		dirStartIdx := optNum
		for i, d := range dirs {
			fmt.Printf("  %d. 📂 %s\n", dirStartIdx+i, d)
		}

		fmt.Print("\nChoice: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		choice := 0
		fmt.Sscanf(input, "%d", &choice)

		switch {
		case choice == 1:
			// Use this folder
			if currentPath == "" {
				// Don't sync to root, suggest FetchQuest
				fmt.Print("  Folder name to create (default: FetchQuest): ")
				name, _ := reader.ReadString('\n')
				name = strings.TrimSpace(name)
				if name == "" {
					name = "FetchQuest"
				}
				finalPath := name
				remoteFinal := remoteName + ":" + finalPath
				fmt.Printf("  Creating %s...\n", remoteFinal)
				mkOut, err := exec.Command("rclone", "mkdir", remoteFinal).CombinedOutput()
				if err != nil {
					return "", fmt.Errorf("failed to create folder: %w\n%s", err, mkOut)
				}
				return finalPath, nil
			}
			return currentPath, nil

		case choice == 2:
			// Create new folder
			fmt.Print("  New folder name: ")
			name, _ := reader.ReadString('\n')
			name = strings.TrimSpace(name)
			if name == "" {
				fmt.Println("  Skipped.")
				continue
			}
			newPath := name
			if currentPath != "" {
				newPath = currentPath + "/" + name
			}
			remoteFinal := remoteName + ":" + newPath
			fmt.Printf("  Creating %s...\n", remoteFinal)
			mkOut, err := exec.Command("rclone", "mkdir", remoteFinal).CombinedOutput()
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Failed to create folder: %v\n%s", err, mkOut)
				continue
			}
			return newPath, nil

		case currentPath != "" && choice == 3:
			// Go up
			idx := strings.LastIndex(currentPath, "/")
			if idx < 0 {
				currentPath = ""
			} else {
				currentPath = currentPath[:idx]
			}

		default:
			// Navigate into a directory
			adjustedChoice := choice - dirStartIdx
			if adjustedChoice >= 0 && adjustedChoice < len(dirs) {
				if currentPath == "" {
					currentPath = dirs[adjustedChoice]
				} else {
					currentPath = currentPath + "/" + dirs[adjustedChoice]
				}
			} else {
				fmt.Println("  Invalid choice, try again.")
			}
		}
	}
}

const localDestSentinel = "__local__"

func interactiveAddDest(reader *bufio.Reader) (string, string, error) {
	fmt.Println("\nWhat type of destination?")
	fmt.Printf("  1. Local folder — Save to a directory on this computer\n")
	for i, p := range destPresets {
		fmt.Printf("  %d. %s — %s\n", i+2, p.name, p.description)
	}
	fmt.Printf("  %d. Other (I'll provide the rclone remote string)\n", len(destPresets)+2)
	fmt.Print("\nChoice: ")

	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	// Local folder option
	if choice == "1" {
		fmt.Print("  Folder path (default: ~/FetchQuest): ")
		path, _ := reader.ReadString('\n')
		path = strings.TrimSpace(path)
		if path == "" {
			path = "~/FetchQuest"
		}
		return localDestSentinel, path, nil
	}

	// Everything below here needs rclone
	if _, err := exec.LookPath("rclone"); err != nil {
		return "", "", fmt.Errorf("rclone is not installed — install it first (https://rclone.org/install/)")
	}

	// "Other" option
	if choice == fmt.Sprintf("%d", len(destPresets)+2) {
		fmt.Print("Destination name: ")
		name, _ := reader.ReadString('\n')
		name = strings.TrimSpace(name)
		if name == "" {
			return "", "", fmt.Errorf("name is required")
		}
		fmt.Print("Rclone remote (e.g. myremote:path/to/folder): ")
		remote, _ := reader.ReadString('\n')
		remote = strings.TrimSpace(remote)
		if remote == "" {
			return "", "", fmt.Errorf("remote is required")
		}
		return name, remote, nil
	}

	// Parse preset choice (offset by 1 because local folder is option 1)
	idx := 0
	fmt.Sscanf(choice, "%d", &idx)
	idx -= 2 // adjust for local folder being option 1
	if idx < 0 || idx >= len(destPresets) {
		return "", "", fmt.Errorf("invalid choice")
	}
	preset := destPresets[idx]

	remoteName := preset.remoteName
	fmt.Printf("\nSetting up %s...\n", preset.name)

	if preset.extraSetup != nil {
		// Custom setup (e.g., SMB needs host/user/pass)
		if err := preset.extraSetup(reader, remoteName); err != nil {
			return "", "", err
		}
	} else {
		// Standard OAuth-based setup — run rclone config create
		fmt.Println("This will open your browser to authorize access.")
		fmt.Print("Press Enter to continue...")
		reader.ReadString('\n')

		create := exec.Command("rclone", "config", "create", remoteName, preset.rcloneType)
		create.Stdout = os.Stdout
		create.Stderr = os.Stderr
		create.Stdin = os.Stdin
		if err := create.Run(); err != nil {
			return "", "", fmt.Errorf("rclone config create failed: %w", err)
		}
	}

	// Choose folder
	fmt.Printf("\nHow do you want to pick the folder on %s?\n", preset.name)
	fmt.Println("  1. Browse folders interactively")
	fmt.Println("  2. Type/paste a path")
	fmt.Print("\nChoice: ")
	folderChoice, _ := reader.ReadString('\n')
	folderChoice = strings.TrimSpace(folderChoice)

	var folder string
	if folderChoice == "2" {
		fmt.Printf("  Path or link (e.g. Documents/FetchQuest or a Google Drive URL): ")
		path, _ := reader.ReadString('\n')
		path = strings.TrimSpace(path)

		// Detect pasted URLs and extract the path/ID
		driveURLPattern := regexp.MustCompile(`drive\.google\.com/drive/.*folders/([a-zA-Z0-9_-]+)`)
		dropboxURLPattern := regexp.MustCompile(`dropbox\.com/home(.*)`)

		if m := driveURLPattern.FindStringSubmatch(path); m != nil {
			// Google Drive: extract folder ID and set as root
			folderID := m[1]
			fmt.Printf("  Detected Google Drive folder ID: %s\n", folderID)
			fmt.Printf("  Setting root_folder_id on rclone remote %q...\n", remoteName)
			out, err := exec.Command("rclone", "config", "update", remoteName, "root_folder_id", folderID).CombinedOutput()
			if err != nil {
				return "", "", fmt.Errorf("failed to set root_folder_id: %w\n%s", err, out)
			}
			fmt.Printf("  Create a FetchQuest subfolder there? [Y/n] ")
			yn, _ := reader.ReadString('\n')
			yn = strings.TrimSpace(strings.ToLower(yn))
			if yn != "n" && yn != "no" {
				path = "FetchQuest"
			} else {
				path = ""
			}
		} else if m := dropboxURLPattern.FindStringSubmatch(path); m != nil {
			// Dropbox: extract path from URL
			extracted := strings.TrimPrefix(m[1], "/")
			if extracted != "" {
				fmt.Printf("  Detected Dropbox path: %s\n", extracted)
				path = extracted
			} else {
				path = ""
			}
			fmt.Printf("  Create a FetchQuest subfolder there? [Y/n] ")
			yn, _ := reader.ReadString('\n')
			yn = strings.TrimSpace(strings.ToLower(yn))
			if yn != "n" && yn != "no" {
				if path == "" {
					path = "FetchQuest"
				} else {
					path = path + "/FetchQuest"
				}
			}
		} else if strings.Contains(path, "://") {
			// Generic URL — warn the user
			return "", "", fmt.Errorf("unrecognized URL format — please provide a folder path instead (e.g. Documents/FetchQuest)")
		} else if path == "" {
			path = "FetchQuest"
			fmt.Printf("  Using default: %s\n", path)
		} else {
			fmt.Printf("  Create a FetchQuest subfolder there? [Y/n] ")
			yn, _ := reader.ReadString('\n')
			yn = strings.TrimSpace(strings.ToLower(yn))
			if yn != "n" && yn != "no" {
				path = path + "/FetchQuest"
			}
		}

		if path != "" {
			// Ensure folder exists
			remoteFull := remoteName + ":" + path
			fmt.Printf("  Creating %s...\n", remoteFull)
			mkOut, mkErr := exec.Command("rclone", "mkdir", remoteFull).CombinedOutput()
			if mkErr != nil {
				return "", "", fmt.Errorf("failed to create folder: %w\n%s", mkErr, mkOut)
			}
		}
		folder = path
	} else {
		var err error
		folder, err = browseRemoteFolder(reader, remoteName, "")
		if err != nil {
			return "", "", err
		}
	}

	rcloneRemote := fmt.Sprintf("%s:%s", remoteName, folder)

	// Use preset name as the destination name
	destName := strings.ToLower(strings.ReplaceAll(preset.name, " / ", "-"))
	destName = strings.ReplaceAll(destName, " ", "-")

	return destName, rcloneRemote, nil
}

var configAddDestCmd = &cobra.Command{
	Use:   "add-dest [name] [rclone_remote]",
	Short: "Add a sync destination (interactive if no args given)",
	Long: `Add a destination for syncing Quest media.

Run without arguments for an interactive setup wizard:
  fetchquest config add-dest

Or provide name and rclone remote directly:
  fetchquest config add-dest my-nas "nas:share/FetchQuest"`,
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var name, remote string

		if len(args) == 2 {
			// Direct args — check rclone is available
			if _, err := exec.LookPath("rclone"); err != nil {
				return fmt.Errorf("rclone is not installed — install it first (https://rclone.org/install/)")
			}
			name = args[0]
			remote = args[1]
		} else if len(args) == 0 {
			reader := bufio.NewReader(os.Stdin)
			var err error
			name, remote, err = interactiveAddDest(reader)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("provide both name and remote, or no arguments for interactive setup")
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		// Local folder — set sync_dir instead of adding a destination
		if name == localDestSentinel {
			cfg.SyncDir = remote
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("\nLocal sync directory set to: %s\n", remote)
			return nil
		}

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
		fmt.Printf("\nAdded destination: %s -> %s\n", name, remote)
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
