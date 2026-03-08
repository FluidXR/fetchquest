package adb

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// FileInfo represents a file on the Quest filesystem.
type FileInfo struct {
	Path  string
	Size  int64
	MTime time.Time
}

// Client wraps ADB command-line calls.
type Client struct {
	bin string // path to adb binary
}

// NewClient creates a new ADB client. If adbPath is empty, "adb" is used (found via PATH).
func NewClient(adbPath ...string) *Client {
	bin := "adb"
	if len(adbPath) > 0 && adbPath[0] != "" {
		bin = adbPath[0]
	} else {
		bin = findBin("adb")
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
	return name // fall back to bare name
}

// Bin returns the adb binary path.
func (c *Client) Bin() string { return c.bin }

// Devices returns all connected ADB devices.
func (c *Client) Devices() ([]Device, error) {
	out, err := newCmd(c.bin, "devices", "-l").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("adb devices: %w\n%s", err, out)
	}
	return parseDeviceList(string(out)), nil
}

// Connect connects to a wireless ADB device.
func (c *Client) Connect(ip string, port int) error {
	addr := fmt.Sprintf("%s:%d", ip, port)
	out, err := newCmd(c.bin, "connect", addr).CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb connect %s: %w\n%s", addr, err, out)
	}
	output := string(out)
	if strings.Contains(output, "connected") {
		return nil
	}
	return fmt.Errorf("adb connect %s: %s", addr, strings.TrimSpace(output))
}

// ListFiles lists files in a directory on the device, non-recursively.
func (c *Client) ListFiles(serial, remotePath string) ([]FileInfo, error) {
	// Use `ls -la` to get file info
	out, err := newCmd(c.bin, "-s", serial, "shell",
		fmt.Sprintf("find %s -maxdepth 1 -type f -exec stat -c '%%s %%Y %%n' {} +", remotePath),
	).CombinedOutput()
	if err != nil {
		// Directory might not exist
		if strings.Contains(string(out), "No such file") {
			return nil, nil
		}
		return nil, fmt.Errorf("adb shell find %s: %w\n%s", remotePath, err, out)
	}
	return parseStatOutput(string(out)), nil
}

// ListFilesRecursive lists all files recursively under a directory.
func (c *Client) ListFilesRecursive(serial, remotePath string) ([]FileInfo, error) {
	return c.ListFilesRecursiveCtx(context.Background(), serial, remotePath)
}

// ListFilesRecursiveCtx is like ListFilesRecursive but accepts a context for cancellation.
func (c *Client) ListFilesRecursiveCtx(ctx context.Context, serial, remotePath string) ([]FileInfo, error) {
	out, err := newCmdContext(ctx, c.bin, "-s", serial, "shell",
		fmt.Sprintf("find %s -type f -exec stat -c '%%s %%Y %%n' {} +", remotePath),
	).CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if strings.Contains(string(out), "No such file") {
			return nil, nil
		}
		return nil, fmt.Errorf("adb shell find %s: %w\n%s", remotePath, err, out)
	}
	return parseStatOutput(string(out)), nil
}

// Pull copies a file from the device to the local filesystem.
func (c *Client) Pull(serial, remotePath, localPath string) error {
	out, err := newCmd(c.bin, "-s", serial, "pull", remotePath, localPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb pull %s: %w\n%s", remotePath, err, out)
	}
	return nil
}

// Remove deletes a file on the device.
func (c *Client) Remove(serial, remotePath string) error {
	out, err := newCmd(c.bin, "-s", serial, "shell", "rm", remotePath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb rm %s: %w\n%s", remotePath, err, out)
	}
	return nil
}

// parseDeviceList parses `adb devices -l` output.
func parseDeviceList(output string) []Device {
	var devices []Device
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "List of") || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		d := Device{
			Serial: fields[0],
			State:  fields[1],
		}
		// Determine connection type
		if strings.Contains(d.Serial, ":") {
			d.ConnType = WiFi
		} else {
			d.ConnType = USB
		}
		// Parse key:value pairs
		for _, f := range fields[2:] {
			parts := strings.SplitN(f, ":", 2)
			if len(parts) != 2 {
				continue
			}
			switch parts[0] {
			case "model":
				d.Model = parts[1]
			case "product":
				d.Product = parts[1]
			case "transport_id":
				d.TransportID = parts[1]
			}
		}
		devices = append(devices, d)
	}
	return devices
}

// parseStatOutput parses output from `stat -c '%s %Y %n'`.
func parseStatOutput(output string) []FileInfo {
	var files []FileInfo
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Format: <size> <mtime_epoch> <full_path>
		parts := strings.SplitN(line, " ", 3)
		if len(parts) != 3 {
			continue
		}
		size, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			continue
		}
		epoch, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}
		path := parts[2]
		// Normalize path
		path = filepath.ToSlash(strings.TrimRight(path, "\r"))
		files = append(files, FileInfo{
			Path:  path,
			Size:  size,
			MTime: time.Unix(epoch, 0),
		})
	}
	return files
}
