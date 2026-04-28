package modemsysfs

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	VendorID          = "2ecc"
	ProductID         = "3012"
	atInterfaceSuffix = "1.2"
)

type Device struct {
	SysfsPath        string
	ATTTY            string
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
			ATTTY:            findATTTY(realPath, ttyClassRoot),
			NetworkInterface: findNetworkInterface(realPath, netClassRoot),
		})
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

func findATTTY(devicePath, ttyClassRoot string) string {
	matches, err := filepath.Glob(filepath.Join(ttyClassRoot, "ttyUSB*"))
	if err != nil {
		return ""
	}

	sort.Strings(matches)
	interfacePrefix := devicePath + ":" + atInterfaceSuffix + string(os.PathSeparator)
	for _, match := range matches {
		resolved, err := filepath.EvalSymlinks(match)
		if err != nil {
			continue
		}
		if strings.HasPrefix(resolved, interfacePrefix) {
			return filepath.Base(match)
		}
	}
	return ""
}

func findNetworkInterface(devicePath, netClassRoot string) string {
	matches, err := filepath.Glob(filepath.Join(netClassRoot, "*"))
	if err != nil {
		return ""
	}

	sort.Strings(matches)
	interfacePrefix := devicePath + ":"
	for _, match := range matches {
		resolved, err := filepath.EvalSymlinks(match)
		if err != nil {
			continue
		}
		if strings.HasPrefix(resolved, interfacePrefix) {
			return filepath.Base(match)
		}
	}
	return ""
}
