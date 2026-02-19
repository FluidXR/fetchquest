package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/FluidXR/fetchquest/internal/adb"
	"github.com/FluidXR/fetchquest/internal/config"
)

type dependency struct {
	name       string
	binary     string
	installCmd map[string]string // GOOS -> install command
}

var dependencies = []dependency{
	{
		name:   "ADB (Android Debug Bridge)",
		binary: "adb",
		installCmd: map[string]string{
			"darwin":  "brew install android-platform-tools",
			"linux":   "sudo apt install android-tools-adb",
			"windows": "winget install Google.PlatformTools",
		},
	},
	{
		name:   "rclone",
		binary: "rclone",
		installCmd: map[string]string{
			"darwin":  "brew install rclone",
			"linux":   "curl https://rclone.org/install.sh | sudo bash",
			"windows": "winget install Rclone.Rclone",
		},
	},
}

// checkDeps verifies that required external tools are installed.
// Returns nil if all deps are present or user declines to install.
func checkDeps() error {
	var missing []dependency
	for _, dep := range dependencies {
		if _, err := exec.LookPath(dep.binary); err != nil {
			missing = append(missing, dep)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	fmt.Println("FetchQuest requires the following tools that are not installed:")
	fmt.Println()
	for _, dep := range missing {
		fmt.Printf("  - %s (%s)\n", dep.name, dep.binary)
	}
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	for _, dep := range missing {
		cmd, ok := dep.installCmd[runtime.GOOS]
		if !ok {
			fmt.Printf("Please install %s manually and try again.\n", dep.name)
			continue
		}

		fmt.Printf("Install %s with: %s\n", dep.name, cmd)
		fmt.Print("Run now? [Y/n] ")
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))

		if answer != "" && answer != "y" && answer != "yes" {
			fmt.Printf("Skipped. Install %s manually before using fetchquest.\n", dep.name)
			continue
		}

		fmt.Printf("Running: %s\n", cmd)
		parts := strings.Fields(cmd)
		install := exec.Command(parts[0], parts[1:]...)
		install.Stdout = os.Stdout
		install.Stderr = os.Stderr
		install.Stdin = os.Stdin
		if err := install.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to install %s: %v\n", dep.name, err)
			fmt.Fprintf(os.Stderr, "Please install it manually and try again.\n")
		} else {
			fmt.Printf("%s installed successfully.\n\n", dep.name)
		}
	}

	// Re-check after install attempts
	for _, dep := range missing {
		if _, err := exec.LookPath(dep.binary); err != nil {
			return fmt.Errorf("%s is required but not installed", dep.binary)
		}
	}
	return nil
}

// checkNewDevices prompts the user to nickname any newly discovered devices.
func checkNewDevices() {
	cfg, err := config.Load()
	if err != nil {
		return
	}

	client := adb.NewClient()
	devices, err := client.Devices()
	if err != nil {
		return
	}

	reader := bufio.NewReader(os.Stdin)
	changed := false

	for _, d := range devices {
		if !d.IsOnline() {
			continue
		}
		if _, known := cfg.Devices[d.Serial]; known {
			continue
		}

		model := d.Model
		if model == "" {
			model = "unknown model"
		}
		fmt.Printf("\nNew device detected: %s (%s)\n", d.Serial, model)
		fmt.Print("Give it a nickname (or press Enter to skip): ")
		name, _ := reader.ReadString('\n')
		name = strings.TrimSpace(name)

		if cfg.Devices == nil {
			cfg.Devices = make(map[string]config.DeviceConfig)
		}
		dc := cfg.Devices[d.Serial]
		if name != "" {
			dc.Nickname = name
		}
		cfg.Devices[d.Serial] = dc
		changed = true
	}

	if changed {
		if err := config.Save(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save config: %v\n", err)
		}
	}
}
