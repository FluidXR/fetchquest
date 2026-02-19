package rclone

import (
	"fmt"
	"os/exec"
	"strings"
)

// Client wraps rclone command-line calls.
type Client struct{}

// NewClient creates a new rclone client.
func NewClient() *Client {
	return &Client{}
}

// Copy uploads a local file to an rclone remote destination.
// dest should be like "gdrive:QuestMedia/device123/Videos/"
func (c *Client) Copy(localPath, dest string) error {
	out, err := exec.Command("rclone", "copyto", localPath, dest).CombinedOutput()
	if err != nil {
		return fmt.Errorf("rclone copyto %s -> %s: %w\n%s", localPath, dest, err, out)
	}
	return nil
}

// CopyFrom downloads a remote file to a local path.
func (c *Client) CopyFrom(remoteSrc, localDest string) error {
	out, err := exec.Command("rclone", "copyto", remoteSrc, localDest).CombinedOutput()
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
	out, err := exec.Command("rclone", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("rclone copy %s -> %s: %w\n%s", localDir, dest, err, out)
	}
	return nil
}

// Check verifies a file exists at the destination.
func (c *Client) Check(localPath, dest string) error {
	out, err := exec.Command("rclone", "check", "--one-way",
		"--include", localPath, ".", dest,
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("rclone check: %w\n%s", err, out)
	}
	return nil
}

// ListRemotes returns configured rclone remotes.
func (c *Client) ListRemotes() ([]string, error) {
	out, err := exec.Command("rclone", "listremotes").CombinedOutput()
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
