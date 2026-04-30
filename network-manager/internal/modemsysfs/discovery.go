package modemsysfs

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	VendorID  = "2ecc"
	ProductID = "3012"
)

type Device struct {
	SysfsPath        string
	ATTTY            string
	ATTTYs           []string
	NetworkInterface string
}

func FirstDevice() (Device, bool) {
	devices := scan("/sys/bus/usb/devices", "/sys/class/tty", "/sys/class/net")
	if len(devices) == 0 {
		return Device{}, false
	}
	return devices[0], true
}

func scan(usbDevicesRoot, ttyClassRoot, netClassRoot string) []Device {
	entries, err := os.ReadDir(usbDevicesRoot)
	if err != nil {
		return nil
	}

	var devices []Device
	for _, entry := range entries {
		name := entry.Name()
		if strings.Contains(name, ":") {
			continue
		}

		devicePath := filepath.Join(usbDevicesRoot, name)
		realPath, ok := matchingDevicePath(devicePath)
		if !ok {
			continue
		}

		devices = append(devices, Device{
			SysfsPath:        realPath,
			ATTTYs:           findATTTYs(realPath, ttyClassRoot),
			NetworkInterface: findNetworkInterface(realPath, netClassRoot),
		})
		if len(devices[len(devices)-1].ATTTYs) > 0 {
			devices[len(devices)-1].ATTTY = devices[len(devices)-1].ATTTYs[0]
		}
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].SysfsPath < devices[j].SysfsPath
	})
	return devices
}

func matchingDevicePath(devicePath string) (string, bool) {
	vendor, err := os.ReadFile(filepath.Join(devicePath, "idVendor"))
	if err != nil || strings.TrimSpace(string(vendor)) != VendorID {
		return "", false
	}

	product, err := os.ReadFile(filepath.Join(devicePath, "idProduct"))
	if err != nil || strings.TrimSpace(string(product)) != ProductID {
		return "", false
	}

	realPath, err := filepath.EvalSymlinks(devicePath)
	if err != nil {
		return "", false
	}
	return realPath, true
}

func findATTTYs(devicePath, ttyClassRoot string) []string {
	matches := ttyMatches(ttyClassRoot)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(matches))
	var ttys []string
	for _, match := range matches {
		resolved, err := filepath.EvalSymlinks(match)
		if err != nil {
			continue
		}
		if !belongsToDevice(resolved, devicePath) {
			continue
		}

		name := filepath.Base(match)
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		ttys = append(ttys, name)
	}
	return ttys
}

func ttyMatches(ttyClassRoot string) []string {
	patterns := []string{"ttyUSB*", "ttyACM*"}
	var matches []string
	for _, pattern := range patterns {
		found, err := filepath.Glob(filepath.Join(ttyClassRoot, pattern))
		if err != nil {
			continue
		}
		matches = append(matches, found...)
	}
	sort.Strings(matches)
	return matches
}

func findNetworkInterface(devicePath, netClassRoot string) string {
	matches, err := filepath.Glob(filepath.Join(netClassRoot, "*"))
	if err != nil {
		return ""
	}

	sort.Strings(matches)
	for _, match := range matches {
		resolved, err := filepath.EvalSymlinks(match)
		if err != nil {
			continue
		}
		if belongsToDevice(resolved, devicePath) {
			return filepath.Base(match)
		}
	}
	return ""
}

func belongsToDevice(resolvedPath, devicePath string) bool {
	resolvedPath = filepath.Clean(resolvedPath)
	devicePath = filepath.Clean(devicePath)
	if resolvedPath == devicePath {
		return true
	}
	if strings.HasPrefix(resolvedPath, devicePath+string(filepath.Separator)) {
		return true
	}
	return strings.HasPrefix(resolvedPath, devicePath+":")
}
