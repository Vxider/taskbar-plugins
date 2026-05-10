package helper

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/vxider/codex-buddy/uconsole/network-manager/internal/modemsysfs"
	"golang.org/x/sys/unix"
)

var (
	findATPortFunc  = findATPort
	findATPortsFunc = findATPorts
	firstDeviceFunc = modemsysfs.FirstDevice
	sendATFunc      = sendAT
	sleepFunc       = time.Sleep
	runFunc         = run
	writeNewIDFunc  = writeNewID
	ensureNodesFunc = ensureATDeviceNodes
	mknodFunc       = unix.Mknod
	chmodFunc       = os.Chmod
	devRoot         = "/dev"
	ttyClassRoot    = "/sys/class/tty"
)

func Run(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: --helper modem [bind|on|standby|off]")
	}
	if args[0] != "modem" {
		return fmt.Errorf("unknown helper target: %s", args[0])
	}
	switch strings.ToLower(strings.TrimSpace(args[1])) {
	case "bind":
		return ensureATDriver()
	}

	var cfun string
	switch strings.ToLower(strings.TrimSpace(args[1])) {
	case "on":
		cfun = "1"
	case "standby":
		cfun = "4"
	case "off":
		cfun = "0"
	default:
		return fmt.Errorf("unknown modem mode: %s", args[1])
	}

	port, err := ensureATPort()
	if err != nil {
		return err
	}

	// Keep ModemManager running. Stopping the service can disturb shared device
	// paths on uConsole-class hardware, including the built-in keyboard path.
	_, port, err = sendATAcrossCandidates(port, "AT", 3, 300*time.Millisecond)
	if err != nil {
		return err
	}
	_, port, err = sendATAcrossCandidates(port, "AT+CFUN="+cfun, 3, 300*time.Millisecond)
	if err != nil {
		return err
	}

	if cfun != "1" {
		fmt.Printf("modem %s via %s\n", args[1], port)
		return nil
	}

	readyPort, err := waitForATPortReady(port, 12*time.Second)
	if err != nil {
		return err
	}

	fmt.Printf("modem %s via %s\n", args[1], readyPort)
	return nil
}

func ensureATPort() (string, error) {
	if port := findATPortFunc(); port != "" {
		return port, nil
	}

	if err := ensureATDriver(); err != nil {
		return "", err
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if port := findATPortFunc(); port != "" {
			return port, nil
		}
		sleepFunc(200 * time.Millisecond)
	}
	return "", fmt.Errorf("timed out waiting for modem AT port")
}

func ensureATDriver() error {
	if err := runFunc("modprobe", "option"); err != nil {
		return err
	}
	if err := writeNewIDFunc(); err != nil && !errors.Is(err, unix.EEXIST) {
		return err
	}
	return ensureNodesFunc()
}

func sendATWithRetry(path, command string, attempts int, delay time.Duration) (string, error) {
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		response, err := sendATFunc(path, command)
		if err == nil {
			return response, nil
		}
		lastErr = err
		if attempt < attempts {
			sleepFunc(delay)
		}
	}

	return "", fmt.Errorf("%s failed after %d attempt(s): %w", command, attempts, lastErr)
}

func sendATAcrossCandidates(initialPath, command string, attempts int, delay time.Duration) (string, string, error) {
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		for _, candidate := range candidateATPorts(initialPath) {
			response, err := sendATFunc(candidate, command)
			if err == nil {
				return response, candidate, nil
			}
			lastErr = err
		}
		if attempt < attempts {
			sleepFunc(delay)
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no modem AT ports found")
	}
	return "", initialPath, fmt.Errorf("%s failed after %d attempt(s): %w", command, attempts, lastErr)
}

func waitForATPortReady(initialPath string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		for _, candidate := range candidateATPorts(initialPath) {
			if _, err := sendATFunc(candidate, "AT"); err == nil {
				return candidate, nil
			} else {
				lastErr = err
			}
		}
		sleepFunc(300 * time.Millisecond)
	}

	if lastErr != nil {
		return "", fmt.Errorf("timed out waiting for modem AT port to recover: %w", lastErr)
	}
	return "", fmt.Errorf("timed out waiting for modem AT port to recover")
}

func candidateATPorts(initialPath string) []string {
	seen := make(map[string]struct{}, 4)
	var ports []string

	for _, path := range append([]string{initialPath, findATPortFunc()}, findATPortsFunc()...) {
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		ports = append(ports, path)
	}

	return ports
}

func writeNewID() error {
	path := "/sys/bus/usb-serial/drivers/option1/new_id"
	return os.WriteFile(path, []byte(modemsysfs.VendorID+" "+modemsysfs.ProductID), 0o644)
}

func findATPort() string {
	ports := findATPorts()
	if len(ports) == 0 {
		return ""
	}
	return ports[0]
}

func findATPorts() []string {
	device, ok := firstDeviceFunc()
	if !ok {
		return nil
	}

	ttys := device.ATTTYs
	if len(ttys) == 0 && device.ATTTY != "" {
		ttys = []string{device.ATTTY}
	}

	ports := make([]string, 0, len(ttys))
	for _, tty := range ttys {
		if tty == "" {
			continue
		}
		ports = append(ports, "/dev/"+tty)
	}
	return ports
}

func ensureATDeviceNodes() error {
	device, ok := firstDeviceFunc()
	if !ok {
		return nil
	}

	var lastErr error
	for _, tty := range device.ATTTYs {
		if strings.TrimSpace(tty) == "" {
			continue
		}
		if err := ensureTTYDeviceNode(tty); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func ensureTTYDeviceNode(tty string) error {
	devPath := devRoot + "/" + tty

	raw, err := os.ReadFile(ttyClassRoot + "/" + tty + "/dev")
	if err != nil {
		return err
	}
	major, minor, err := parseDeviceMajorMinor(string(raw))
	if err != nil {
		return err
	}

	dev := int(unix.Mkdev(uint32(major), uint32(minor)))
	if _, err := os.Stat(devPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := mknodFunc(devPath, unix.S_IFCHR|0o660, dev); err != nil && !errors.Is(err, unix.EEXIST) && !errors.Is(err, os.ErrExist) {
			return err
		}
	}
	return chmodFunc(devPath, 0o660)
}

func parseDeviceMajorMinor(raw string) (int, int, error) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected device number %q", strings.TrimSpace(raw))
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return major, minor, nil
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func sendAT(path, command string) (string, error) {
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer unix.Close(fd)

	tios, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return "", fmt.Errorf("termios get: %w", err)
	}

	tios.Iflag = 0
	tios.Oflag = 0
	tios.Cflag = unix.CS8 | unix.CREAD | unix.CLOCAL
	tios.Lflag = 0
	tios.Cc[unix.VMIN] = 0
	tios.Cc[unix.VTIME] = 10
	tios.Ispeed = unix.B115200
	tios.Ospeed = unix.B115200
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, tios); err != nil {
		return "", fmt.Errorf("termios set: %w", err)
	}

	if err := unix.IoctlSetInt(fd, unix.TCFLSH, unix.TCIOFLUSH); err != nil {
		return "", fmt.Errorf("flush tty: %w", err)
	}

	if _, err := unix.Write(fd, []byte(command+"\r")); err != nil {
		return "", fmt.Errorf("write %s: %w", command, err)
	}

	deadline := time.Now().Add(8 * time.Second)
	var response strings.Builder
	buf := make([]byte, 512)
	for time.Now().Before(deadline) {
		pollFds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		_, err := unix.Poll(pollFds, 250)
		if err != nil && err != unix.EINTR {
			return response.String(), fmt.Errorf("poll tty: %w", err)
		}
		if pollFds[0].Revents&unix.POLLIN == 0 {
			continue
		}
		n, err := unix.Read(fd, buf)
		if err != nil {
			if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
				continue
			}
			return response.String(), fmt.Errorf("read tty: %w", err)
		}
		if n == 0 {
			continue
		}
		response.Write(buf[:n])
		text := response.String()
		if strings.Contains(text, "\r\nOK\r\n") {
			return text, nil
		}
		if strings.Contains(text, "ERROR") || strings.Contains(text, "+CME ERROR") {
			return text, fmt.Errorf("%s failed: %s", command, compact(text))
		}
	}
	return response.String(), fmt.Errorf("%s timed out", command)
}

func compact(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}
