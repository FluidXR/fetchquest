package adb

// ConnectionType indicates how a device is connected.
type ConnectionType string

const (
	USB     ConnectionType = "usb"
	WiFi    ConnectionType = "wifi"
	Unknown ConnectionType = "unknown"
)

// Device represents a connected ADB device.
type Device struct {
	Serial     string
	State      string // "device", "offline", "unauthorized", etc.
	ConnType   ConnectionType
	Model      string
	Product    string
	TransportID string
}

// IsOnline returns true if the device is in "device" state (ready).
func (d Device) IsOnline() bool {
	return d.State == "device"
}
