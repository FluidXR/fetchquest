package adb

import (
	"bufio"
	"fmt"
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
type Client struct{}

// NewClient creates a new ADB client.
func NewClient() *Client {
	return &Client{}
}

// Devices returns all connected ADB devices.
func (c *Client) Devices() ([]Device, error) {
	out, err := exec.Command("adb", "devices", "-l").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("adb devices: %w\n%s", err, out)
	}
	return parseDeviceList(string(out)), nil
}

// Connect connects to a wireless ADB device.
func (c *Client) Connect(ip string, port int) error {
	addr := fmt.Sprintf("%s:%d", ip, port)
	out, err := exec.Command("adb", "connect", addr).CombinedOutput()
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
	out, err := exec.Command("adb", "-s", serial, "shell",
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
	out, err := exec.Command("adb", "-s", serial, "shell",
		fmt.Sprintf("find %s -type f -exec stat -c '%%s %%Y %%n' {} +", remotePath),
	).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "No such file") {
			return nil, nil
		}
		return nil, fmt.Errorf("adb shell find %s: %w\n%s", remotePath, err, out)
	}
	return parseStatOutput(string(out)), nil
}

// Pull copies a file from the device to the local filesystem.
func (c *Client) Pull(serial, remotePath, localPath string) error {
	out, err := exec.Command("adb", "-s", serial, "pull", remotePath, localPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb pull %s: %w\n%s", remotePath, err, out)
	}
	return nil
}

// Remove deletes a file on the device.
func (c *Client) Remove(serial, remotePath string) error {
	out, err := exec.Command("adb", "-s", serial, "shell", "rm", remotePath).CombinedOutput()
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
