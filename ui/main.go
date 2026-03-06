package main

import (
	"context"
	"embed"
	"log"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func init() {
	// macOS .app bundles get a minimal PATH (/usr/bin:/bin:/usr/sbin:/sbin).
	// Add common install locations so exec.LookPath finds brew/macports binaries.
	extraPaths := []string{
		"/opt/homebrew/bin",
		"/opt/homebrew/sbin",
		"/usr/local/bin",
		"/usr/local/sbin",
		os.ExpandEnv("$HOME/bin"),
		os.ExpandEnv("$HOME/.local/bin"),
	}
	current := os.Getenv("PATH")
	for _, p := range extraPaths {
		if !strings.Contains(current, p) {
			current += ":" + p
		}
	}
	os.Setenv("PATH", current)
}

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:    "FetchQuest",
		Width:    1280,
		Height:   660,
		MinWidth: 640,
		MinHeight: 400,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Mac: &mac.Options{
			About: &mac.AboutInfo{
				Title:   "FetchQuest",
				Message: "Sync your Quest recordings everywhere.",
				Icon:    appIcon,
			},
		},
		OnStartup: func(ctx context.Context) {
			app.SetContext(ctx)
		},
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
