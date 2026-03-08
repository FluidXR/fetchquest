package rclone

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Client wraps rclone command-line calls.
type Client struct {
	bin string // path to rclone binary
}

// NewClient creates a new rclone client. If rclonePath is empty, "rclone" is used (found via PATH).
func NewClient(rclonePath ...string) *Client {
	bin := "rclone"
	if len(rclonePath) > 0 && rclonePath[0] != "" {
		bin = rclonePath[0]
	} else {
		bin = findBin("rclone")
	}
	return &Client{bin: bin}
}

// findBin returns the full path to a binary, checking PATH first then
// common Homebrew locations (useful inside macOS .app bundles where PATH
// is minimal).
func findBin(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	for _, dir := range []string{
		"/opt/homebrew/bin",
		"/usr/local/bin",
	} {
		p := dir + "/" + name
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return name
}

// Bin returns the rclone binary path.
func (c *Client) Bin() string { return c.bin }

// Copy uploads a local file to an rclone remote destination.
// dest should be like "gdrive:QuestMedia/device123/Videos/"
func (c *Client) Copy(localPath, dest string) error {
	out, err := newCmd(c.bin, "copyto", localPath, dest).CombinedOutput()
	if err != nil {
		return fmt.Errorf("rclone copyto %s -> %s: %w\n%s", localPath, dest, err, out)
	}
	return nil
}

// CopyFrom downloads a remote file to a local path.
func (c *Client) CopyFrom(remoteSrc, localDest string) error {
	out, err := newCmd(c.bin, "copyto", remoteSrc, localDest).CombinedOutput()
	if err != nil {
		return fmt.Errorf("rclone copyto %s -> %s: %w\n%s", remoteSrc, localDest, err, out)
	}
	return nil
}

// excludeFlags returns rclone flags to skip macOS/Windows junk files.
var excludeFlags = []string{
	"--exclude", ".DS_Store",
	"--exclude", "._*",
	"--exclude", "Thumbs.db",
	"--exclude", "desktop.ini",
}

// CopyDir uploads a local directory to an rclone remote destination.
func (c *Client) CopyDir(localDir, dest string) error {
	args := append([]string{"copy"}, excludeFlags...)
	args = append(args, localDir, dest)
	out, err := newCmd(c.bin, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("rclone copy %s -> %s: %w\n%s", localDir, dest, err, out)
	}
	return nil
}

// Check verifies a file exists at the destination.
func (c *Client) Check(localPath, dest string) error {
	out, err := newCmd(c.bin, "check", "--one-way",
		"--include", localPath, ".", dest,
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("rclone check: %w\n%s", err, out)
	}
	return nil
}

// IsReachable checks if a remote destination is reachable with a short timeout.
func (c *Client) IsReachable(remote string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := newCmdContext(ctx, c.bin, "lsd", remote, "--max-depth", "0").Run()
	return err == nil
}

// ListRemotes returns configured rclone remotes.
func (c *Client) ListRemotes() ([]string, error) {
	out, err := newCmd(c.bin, "listremotes").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("rclone listremotes: %w\n%s", err, out)
	}
	var remotes []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			remotes = append(remotes, line)
		}
	}
	return remotes, nil
}
