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
	atTTYPath := filepath.Join(root, "devices", "pci0000:00", "0000:00:14.0", "usb1", "1-9:1.6", "ttyUSB7")
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
	if len(device.ATTTYs) != 1 || device.ATTTYs[0] != "ttyUSB7" {
		t.Fatalf("device.ATTTYs = %#v, want []string{\"ttyUSB7\"}", device.ATTTYs)
	}
	if device.NetworkInterface != "wwan0" {
		t.Fatalf("device.NetworkInterface = %q, want %q", device.NetworkInterface, "wwan0")
	}
}

func TestScanFindsAllSerialCandidatesForModem(t *testing.T) {
	root := t.TempDir()
	usbDevicesRoot := filepath.Join(root, "sys", "bus", "usb", "devices")
	ttyClassRoot := filepath.Join(root, "sys", "class", "tty")
	netClassRoot := filepath.Join(root, "sys", "class", "net")

	deviceRealPath := filepath.Join(root, "devices", "usb1", "1-5")
	ttyUSBPath := filepath.Join(root, "devices", "usb1", "1-5:1.3", "ttyUSB2")
	ttyACMPath := filepath.Join(root, "devices", "usb1", "1-5:1.7", "ttyACM0")

	mustMkdirAll(t, usbDevicesRoot)
	mustMkdirAll(t, ttyClassRoot)
	mustMkdirAll(t, netClassRoot)
	mustMkdirAll(t, ttyUSBPath)
	mustMkdirAll(t, ttyACMPath)

	mustWriteFile(t, filepath.Join(deviceRealPath, "idVendor"), VendorID+"\n")
	mustWriteFile(t, filepath.Join(deviceRealPath, "idProduct"), ProductID+"\n")
	mustSymlink(t, deviceRealPath, filepath.Join(usbDevicesRoot, "1-5"))
	mustSymlink(t, ttyUSBPath, filepath.Join(ttyClassRoot, "ttyUSB2"))
	mustSymlink(t, ttyACMPath, filepath.Join(ttyClassRoot, "ttyACM0"))

	devices := scan(usbDevicesRoot, ttyClassRoot, netClassRoot)
	if len(devices) != 1 {
		t.Fatalf("scan() returned %d devices, want 1", len(devices))
	}

	device := devices[0]
	if len(device.ATTTYs) != 2 {
		t.Fatalf("len(device.ATTTYs) = %d, want 2", len(device.ATTTYs))
	}
	if device.ATTTYs[0] != "ttyACM0" || device.ATTTYs[1] != "ttyUSB2" {
		t.Fatalf("device.ATTTYs = %#v, want []string{\"ttyACM0\", \"ttyUSB2\"}", device.ATTTYs)
	}
	if device.ATTTY != "ttyACM0" {
		t.Fatalf("device.ATTTY = %q, want %q", device.ATTTY, "ttyACM0")
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

func TestBelongsToDeviceMatchesInterfaceDescendants(t *testing.T) {
	devicePath := "/sys/devices/platform/axi/1000480000.usb/usb1/1-1/1-1.3"

	if !belongsToDevice(devicePath+"/1-1.3:1.0/net/eth1", devicePath) {
		t.Fatalf("belongsToDevice() = false, want true for network interface descendant")
	}
	if !belongsToDevice(devicePath+"/1-1.3:1.2/ttyUSB2/tty/ttyUSB2", devicePath) {
		t.Fatalf("belongsToDevice() = false, want true for tty descendant")
	}
	if belongsToDevice("/sys/devices/platform/axi/1001100000.mmc/mmc_host/mmc1/mmc1:0001/mmc1:0001:1/net/wlan0", devicePath) {
		t.Fatalf("belongsToDevice() = true, want false for unrelated device")
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
