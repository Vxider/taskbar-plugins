package modemctl

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vxider/codex-buddy/uconsole/network-manager/internal/modemsysfs"
	"golang.org/x/sys/unix"
)

type State struct {
	Installed          bool
	Available          bool
	HardwarePresent    bool
	ATReady            bool
	ModemManagerActive bool
	Error              string
	Manufacturer       string
	Model              string
	Revision           string
	PrimaryPort        string
	NetworkPort        string
	ModemState         string
	PowerState         string
	RegistrationState  string
	PacketServiceState string
	SignalQuality      string
	WiFiConnected      bool
	WiFiDevice         string
	WiFiConnection     string
	WiFiState          string
	AltNetConnected    bool
	AltNetDevice       string
	AltNetType         string
	AltNetConnection   string
	AltNetState        string
}

var (
	runCommandFunc = runCommand
	sleepFunc      = time.Sleep
)

func Load(ctx context.Context) State {
	state := State{}

	loadModemManagerState(ctx, &state)
	loadSysfsFallback(&state)

	if _, err := exec.LookPath("nmcli"); err == nil {
		loadNMCLIState(ctx, &state)
	}

	if _, err := exec.LookPath("mmcli"); err != nil {
		return state
	}
	state.Installed = true

	listRaw, err := run(ctx, "mmcli", "-L")
	if err != nil {
		state.Error = err.Error()
		return state
	}

	modemID := parseFirstModemID(string(listRaw))
	if modemID == "" {
		if strings.Contains(string(listRaw), "No modems were found") {
			if state.HardwarePresent {
				if bindErr := ensureATDriver(ctx); bindErr == nil {
					loadSysfsFallback(&state)
					if retryRaw, retryErr := run(ctx, "mmcli", "-L"); retryErr == nil {
						listRaw = retryRaw
						modemID = parseFirstModemID(string(listRaw))
					}
				}
			}
			if modemID == "" && state.HardwarePresent {
				if state.Manufacturer == "" {
					state.Manufacturer = "CMCC"
				}
				if state.Model == "" {
					state.Model = "ML307C/ML307B"
				}
				if state.ModemState == "" {
					state.ModemState = "present"
				}
				loadATSignalQuality(&state)
				return state
			}
			state.Error = "mmcli reports no modems"
			return state
		}
		state.Error = "mmcli -L returned no modem id: " + strings.Join(strings.Fields(string(listRaw)), " ")
		return state
	}

	raw, err := run(ctx, "mmcli", "-m", modemID, "-K")
	if err != nil {
		state.Error = err.Error()
		return state
	}

	state.Available = true
	values := parseKeyValue(raw)
	state.Manufacturer = values["modem.generic.manufacturer"]
	state.Model = values["modem.generic.model"]
	state.Revision = values["modem.generic.revision"]
	state.PrimaryPort = values["modem.generic.primary-port"]
	state.ModemState = values["modem.generic.state"]
	state.PowerState = values["modem.generic.power-state"]
	state.RegistrationState = values["modem.3gpp.registration-state"]
	state.PacketServiceState = values["modem.3gpp.packet-service-state"]
	state.SignalQuality = values["modem.generic.signal-quality.value"]

	for key, value := range values {
		if !strings.HasPrefix(key, "modem.generic.ports.value[") {
			continue
		}
		if strings.Contains(value, "(net)") && state.NetworkPort == "" {
			state.NetworkPort = strings.TrimSpace(strings.TrimSuffix(value, "(net)"))
		}
	}

	loadATSignalQuality(&state)
	return state
}

func ensureATDriver(ctx context.Context) error {
	device, ok := modemsysfs.FirstDevice()
	if !ok || len(device.ATTTYs) > 0 {
		return nil
	}

	if err := runCommandFunc(ctx, "modprobe", "option"); err != nil {
		if helperErr := runBindHelper(ctx); helperErr != nil {
			return err
		}
	} else if err := writeOptionNewID(); err != nil && !strings.Contains(err.Error(), "File exists") {
		if helperErr := runBindHelper(ctx); helperErr != nil {
			return err
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if device, ok := modemsysfs.FirstDevice(); ok && len(device.ATTTYs) > 0 {
			return nil
		}
		sleepFunc(150 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for modem AT port")
}

func writeOptionNewID() error {
	return os.WriteFile(
		"/sys/bus/usb-serial/drivers/option1/new_id",
		[]byte(modemsysfs.VendorID+" "+modemsysfs.ProductID),
		0o644,
	)
}

func runBindHelper(ctx context.Context) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return runCommandFunc(ctx, "pkexec", exe, "--helper", "modem", "bind")
}

func loadATSignalQuality(state *State) {
	if strings.TrimSpace(state.SignalQuality) != "" {
		return
	}

	var candidates []string
	if state.PrimaryPort != "" {
		candidates = append(candidates, "/dev/"+state.PrimaryPort)
	}
	if device, ok := modemsysfs.FirstDevice(); ok {
		for _, tty := range device.ATTTYs {
			if strings.TrimSpace(tty) != "" {
				candidates = append(candidates, "/dev/"+tty)
			}
		}
	}

	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if _, ok := seen[candidate]; ok || candidate == "/dev/" {
			continue
		}
		seen[candidate] = struct{}{}

		response, err := sendAT(candidate, "AT+CSQ", 1200*time.Millisecond)
		if err != nil {
			continue
		}
		if quality, ok := signalQualityFromCSQ(response); ok {
			state.SignalQuality = quality
			return
		}
	}
}

func signalQualityFromCSQ(response string) (string, bool) {
	match := regexp.MustCompile(`\+CSQ:\s*(\d+)\s*,`).FindStringSubmatch(response)
	if len(match) != 2 {
		return "", false
	}

	rssi, err := strconv.Atoi(match[1])
	if err != nil || rssi < 0 || rssi == 99 {
		return "", false
	}
	if rssi > 31 {
		rssi = 31
	}

	quality := rssi * 100 / 31
	return strconv.Itoa(quality), true
}

func sendAT(path, command string, timeout time.Duration) (string, error) {
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

	deadline := time.Now().Add(timeout)
	var response strings.Builder
	buf := make([]byte, 512)
	for time.Now().Before(deadline) {
		pollFds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		_, err := unix.Poll(pollFds, 100)
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
			return text, fmt.Errorf("%s failed: %s", command, strings.Join(strings.Fields(strings.TrimSpace(text)), " "))
		}
	}
	return response.String(), fmt.Errorf("%s timed out", command)
}

func LiveSummary(state State) string {
	if state.Error != "" {
		return "unavailable"
	}
	switch {
	case !state.Installed:
		return "not installed"
	case state.HardwarePresent && state.NetworkPort != "" && !state.Available:
		return "present"
	case state.HardwarePresent && state.ATReady && !state.ModemManagerActive:
		return "present"
	case state.HardwarePresent && state.ATReady && state.ModemManagerActive && !state.Available:
		return "driver ready"
	case !state.Available:
		return "not found"
	case strings.EqualFold(state.PacketServiceState, "attached"):
		return "online"
	case strings.EqualFold(state.RegistrationState, "home"), strings.EqualFold(state.RegistrationState, "roaming"):
		return "registered"
	case strings.EqualFold(state.ModemState, "disabled"):
		return "disabled"
	default:
		return "detached"
	}
}

func DesiredTarget(mode string, wifiConnected bool) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "standby":
		return "standby"
	case "off":
		return "off"
	case "auto":
		if wifiConnected {
			return "standby"
		}
		return "on"
	default:
		return "on"
	}
}

func loadNMCLIState(ctx context.Context, state *State) {
	raw, err := run(ctx, "nmcli", "-t", "-f", "DEVICE,TYPE,STATE,CONNECTION", "device", "status")
	if err != nil {
		return
	}

	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 4 {
			continue
		}
		device := parts[0]
		deviceType := parts[1]
		deviceState := parts[2]
		connection := strings.Join(parts[3:], ":")
		if deviceType != "wifi" {
			if !state.AltNetConnected && alternativeNetworkType(deviceType) && networkStateConnected(deviceState) && !isModemNetworkDevice(*state, device) {
				state.AltNetConnected = true
				state.AltNetDevice = device
				state.AltNetType = deviceType
				state.AltNetConnection = connection
				state.AltNetState = deviceState
			}
			continue
		}
		state.WiFiDevice = device
		state.WiFiConnection = connection
		state.WiFiState = deviceState
		if wifiStateConnected(deviceState) {
			state.WiFiConnected = true
			state.AltNetConnected = true
			state.AltNetDevice = device
			state.AltNetConnection = connection
			state.AltNetType = deviceType
			state.AltNetState = deviceState
			return
		}
	}
}

func wifiStateConnected(deviceState string) bool {
	return networkStateConnected(deviceState)
}

func networkStateConnected(deviceState string) bool {
	state := strings.ToLower(strings.TrimSpace(deviceState))
	if state == "connected" || strings.HasPrefix(state, "connected ") || strings.HasPrefix(state, "connected(") {
		return true
	}

	trimmed := strings.TrimSpace(deviceState)
	return trimmed == "已连接" || strings.HasPrefix(trimmed, "已连接 ")
}

func alternativeNetworkType(deviceType string) bool {
	switch strings.ToLower(strings.TrimSpace(deviceType)) {
	case "wifi", "ethernet":
		return true
	default:
		return false
	}
}

func isModemNetworkDevice(state State, device string) bool {
	return strings.EqualFold(strings.TrimSpace(state.NetworkPort), strings.TrimSpace(device))
}

func loadModemManagerState(ctx context.Context, state *State) {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return
	}
	cmd := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", "ModemManager")
	if err := cmd.Run(); err == nil {
		state.ModemManagerActive = true
	}
}

func loadSysfsFallback(state *State) {
	device, ok := modemsysfs.FirstDevice()
	if !ok {
		return
	}

	state.HardwarePresent = true
	if device.ATTTY != "" {
		state.ATReady = true
		if state.PrimaryPort == "" {
			state.PrimaryPort = device.ATTTY
		}
	}
	if state.NetworkPort == "" {
		state.NetworkPort = device.NetworkInterface
	}
}

func parseKeyValue(raw []byte) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		values[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return values
}

var modemPathPattern = regexp.MustCompile(`/Modem/([0-9]+)`)

func parseFirstModemID(text string) string {
	match := modemPathPattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), message)
	}
	return stdout.Bytes(), nil
}

func runCommand(ctx context.Context, name string, args ...string) error {
	_, err := run(ctx, name, args...)
	return err
}
