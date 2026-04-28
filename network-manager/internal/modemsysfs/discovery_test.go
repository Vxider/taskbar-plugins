package modemsysfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanFindsModemOnArbitraryUSBPath(t *testing.T) {
	root := t.TempDir()
	usbDevicesRoot := filepath.Join(root, "sys", "bus", "usb", "devices")
	ttyClassRoot := filepath.Join(root, "sys", "class", "tty")
	netClassRoot := filepath.Join(root, "sys", "class", "net")

	deviceRealPath := filepath.Join(root, "devices", "pci0000:00", "0000:00:14.0", "usb1", "1-9")
	atTTYPath := filepath.Join(root, "devices", "pci0000:00", "0000:00:14.0", "usb1", "1-9:1.2", "ttyUSB7")
	netPath := filepath.Join(root, "devices", "pci0000:00", "0000:00:14.0", "usb1", "1-9:1.4", "net", "wwan0")

	mustMkdirAll(t, usbDevicesRoot)
	mustMkdirAll(t, ttyClassRoot)
	mustMkdirAll(t, netClassRoot)
	mustMkdirAll(t, atTTYPath)
	mustMkdirAll(t, netPath)

	mustWriteFile(t, filepath.Join(deviceRealPath, "idVendor"), VendorID+"\n")
	mustWriteFile(t, filepath.Join(deviceRealPath, "idProduct"), ProductID+"\n")

	deviceLinkPath := filepath.Join(usbDevicesRoot, "1-9")
	mustSymlink(t, deviceRealPath, deviceLinkPath)
	mustSymlink(t, atTTYPath, filepath.Join(ttyClassRoot, "ttyUSB7"))
	mustSymlink(t, netPath, filepath.Join(netClassRoot, "wwan0"))

	devices := scan(usbDevicesRoot, ttyClassRoot, netClassRoot)
	if len(devices) != 1 {
		t.Fatalf("scan() returned %d devices, want 1", len(devices))
	}

	device := devices[0]
	if device.SysfsPath != deviceRealPath {
		t.Fatalf("device.SysfsPath = %q, want %q", device.SysfsPath, deviceRealPath)
	}
	if device.ATTTY != "ttyUSB7" {
		t.Fatalf("device.ATTTY = %q, want %q", device.ATTTY, "ttyUSB7")
	}
	if device.NetworkInterface != "wwan0" {
		t.Fatalf("device.NetworkInterface = %q, want %q", device.NetworkInterface, "wwan0")
	}
}

func TestScanIgnoresNonMatchingDevices(t *testing.T) {
	root := t.TempDir()
	usbDevicesRoot := filepath.Join(root, "sys", "bus", "usb", "devices")
	ttyClassRoot := filepath.Join(root, "sys", "class", "tty")
	netClassRoot := filepath.Join(root, "sys", "class", "net")

	otherRealPath := filepath.Join(root, "devices", "usb1", "1-3")
	mustMkdirAll(t, usbDevicesRoot)
	mustMkdirAll(t, ttyClassRoot)
	mustMkdirAll(t, netClassRoot)
	mustWriteFile(t, filepath.Join(otherRealPath, "idVendor"), "ffff\n")
	mustWriteFile(t, filepath.Join(otherRealPath, "idProduct"), "0001\n")
	mustSymlink(t, otherRealPath, filepath.Join(usbDevicesRoot, "1-3"))

	if devices := scan(usbDevicesRoot, ttyClassRoot, netClassRoot); len(devices) != 0 {
		t.Fatalf("scan() returned %d devices, want 0", len(devices))
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	mustMkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func mustSymlink(t *testing.T, target, path string) {
	t.Helper()
	mustMkdirAll(t, filepath.Dir(path))
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("Symlink(%q, %q): %v", target, path, err)
	}
}
