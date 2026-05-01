package helper

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestSendATWithRetry(t *testing.T) {
	origSend := sendATFunc
	origSleep := sleepFunc
	t.Cleanup(func() {
		sendATFunc = origSend
		sleepFunc = origSleep
	})

	attempts := 0
	sendATFunc = func(path, command string) (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("temporary failure")
		}
		return "OK", nil
	}
	sleepFunc = func(time.Duration) {}

	response, err := sendATWithRetry("/dev/ttyUSB2", "AT", 3, time.Millisecond)
	if err != nil {
		t.Fatalf("sendATWithRetry() error = %v", err)
	}
	if response != "OK" {
		t.Fatalf("sendATWithRetry() response = %q, want %q", response, "OK")
	}
	if attempts != 3 {
		t.Fatalf("sendATWithRetry() attempts = %d, want %d", attempts, 3)
	}
}

func TestWaitForATPortReadyTracksReenumeratedPort(t *testing.T) {
	origFind := findATPortFunc
	origFindAll := findATPortsFunc
	origSend := sendATFunc
	origSleep := sleepFunc
	t.Cleanup(func() {
		findATPortFunc = origFind
		findATPortsFunc = origFindAll
		sendATFunc = origSend
		sleepFunc = origSleep
	})

	findCalls := 0
	findATPortFunc = func() string {
		findCalls++
		if findCalls < 2 {
			return ""
		}
		return "/dev/ttyUSB3"
	}
	findATPortsFunc = func() []string {
		return []string{"/dev/ttyUSB3"}
	}

	sendATFunc = func(path, command string) (string, error) {
		switch path {
		case "/dev/ttyUSB2":
			return "", errors.New("old port gone")
		case "/dev/ttyUSB3":
			return "OK", nil
		default:
			return "", errors.New("unexpected port")
		}
	}
	sleepFunc = func(time.Duration) {}

	port, err := waitForATPortReady("/dev/ttyUSB2", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("waitForATPortReady() error = %v", err)
	}
	if port != "/dev/ttyUSB3" {
		t.Fatalf("waitForATPortReady() port = %q, want %q", port, "/dev/ttyUSB3")
	}
}

func TestSendATAcrossCandidatesFallsBackToAnotherPort(t *testing.T) {
	origFind := findATPortFunc
	origFindAll := findATPortsFunc
	origSend := sendATFunc
	origSleep := sleepFunc
	t.Cleanup(func() {
		findATPortFunc = origFind
		findATPortsFunc = origFindAll
		sendATFunc = origSend
		sleepFunc = origSleep
	})

	findATPortFunc = func() string {
		return "/dev/ttyUSB2"
	}
	findATPortsFunc = func() []string {
		return []string{"/dev/ttyUSB2", "/dev/ttyUSB3"}
	}
	sendATFunc = func(path, command string) (string, error) {
		switch path {
		case "/dev/ttyUSB2":
			return "", errors.New("wrong serial function")
		case "/dev/ttyUSB3":
			return "OK", nil
		default:
			return "", errors.New("unexpected port")
		}
	}
	sleepFunc = func(time.Duration) {}

	response, port, err := sendATAcrossCandidates("/dev/ttyUSB2", "AT", 2, time.Millisecond)
	if err != nil {
		t.Fatalf("sendATAcrossCandidates() error = %v", err)
	}
	if response != "OK" {
		t.Fatalf("sendATAcrossCandidates() response = %q, want %q", response, "OK")
	}
	if port != "/dev/ttyUSB3" {
		t.Fatalf("sendATAcrossCandidates() port = %q, want %q", port, "/dev/ttyUSB3")
	}
}

func TestRunStandbyOnlySendsATCommands(t *testing.T) {
	origFind := findATPortFunc
	origFindAll := findATPortsFunc
	origSend := sendATFunc
	origSleep := sleepFunc
	t.Cleanup(func() {
		findATPortFunc = origFind
		findATPortsFunc = origFindAll
		sendATFunc = origSend
		sleepFunc = origSleep
	})

	findATPortFunc = func() string {
		return "/dev/ttyUSB2"
	}
	findATPortsFunc = func() []string {
		return []string{"/dev/ttyUSB2"}
	}
	sleepFunc = func(time.Duration) {}

	var commands []string
	sendATFunc = func(path, command string) (string, error) {
		if path != "/dev/ttyUSB2" {
			t.Fatalf("sendAT path = %q, want /dev/ttyUSB2", path)
		}
		commands = append(commands, command)
		return "OK", nil
	}

	if err := Run([]string{"modem", "standby"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	want := []string{"AT", "AT+CFUN=4"}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
}

func TestRunBindOnlyEnsuresATDriver(t *testing.T) {
	origRun := runFunc
	origWrite := writeNewIDFunc
	origEnsureNodes := ensureNodesFunc
	t.Cleanup(func() {
		runFunc = origRun
		writeNewIDFunc = origWrite
		ensureNodesFunc = origEnsureNodes
	})

	var commands [][]string
	runFunc = func(name string, args ...string) error {
		commands = append(commands, append([]string{name}, args...))
		return nil
	}
	writeNewIDFunc = func() error {
		commands = append(commands, []string{"writeNewID"})
		return nil
	}
	ensureNodesFunc = func() error {
		commands = append(commands, []string{"ensureNodes"})
		return nil
	}

	if err := Run([]string{"modem", "bind"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	want := [][]string{
		{"modprobe", "option"},
		{"writeNewID"},
		{"ensureNodes"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
}

func TestParseDeviceMajorMinor(t *testing.T) {
	major, minor, err := parseDeviceMajorMinor("188:2\n")
	if err != nil {
		t.Fatalf("parseDeviceMajorMinor() error = %v", err)
	}
	if major != 188 || minor != 2 {
		t.Fatalf("parseDeviceMajorMinor() = (%d, %d), want (188, 2)", major, minor)
	}
}

func TestEnsureTTYDeviceNodeCreatesMissingNode(t *testing.T) {
	origMknod := mknodFunc
	origChmod := chmodFunc
	origChown := chownFunc
	origDevRoot := devRoot
	origTTYClassRoot := ttyClassRoot
	origPKEXECUID, hadPKEXECUID := os.LookupEnv("PKEXEC_UID")
	t.Cleanup(func() {
		mknodFunc = origMknod
		chmodFunc = origChmod
		chownFunc = origChown
		devRoot = origDevRoot
		ttyClassRoot = origTTYClassRoot
		if hadPKEXECUID {
			_ = os.Setenv("PKEXEC_UID", origPKEXECUID)
		} else {
			_ = os.Unsetenv("PKEXEC_UID")
		}
	})

	root := t.TempDir()
	devRoot = filepath.Join(root, "dev")
	ttyClassRoot = filepath.Join(root, "sys", "class", "tty")
	mustMkdirAll(t, devRoot)
	mustMkdirAll(t, filepath.Join(ttyClassRoot, "ttyUSB2"))
	mustWriteFile(t, filepath.Join(ttyClassRoot, "ttyUSB2", "dev"), "188:2\n")
	_ = os.Setenv("PKEXEC_UID", "1000")

	var gotPath string
	var gotMode uint32
	var gotDev int
	var gotUID int
	var chmodMode os.FileMode
	mknodFunc = func(path string, mode uint32, dev int) error {
		gotPath = path
		gotMode = mode
		gotDev = dev
		return nil
	}
	chownFunc = func(path string, uid, gid int) error {
		if path != filepath.Join(devRoot, "ttyUSB2") {
			t.Fatalf("chown path = %q, want %q", path, filepath.Join(devRoot, "ttyUSB2"))
		}
		gotUID = uid
		if gid != -1 {
			t.Fatalf("chown gid = %d, want -1", gid)
		}
		return nil
	}
	chmodFunc = func(path string, mode os.FileMode) error {
		if path != filepath.Join(devRoot, "ttyUSB2") {
			t.Fatalf("chmod path = %q, want %q", path, filepath.Join(devRoot, "ttyUSB2"))
		}
		chmodMode = mode
		return nil
	}

	if err := ensureTTYDeviceNode("ttyUSB2"); err != nil {
		t.Fatalf("ensureTTYDeviceNode() error = %v", err)
	}
	if gotPath != filepath.Join(devRoot, "ttyUSB2") {
		t.Fatalf("mknod path = %q, want %q", gotPath, filepath.Join(devRoot, "ttyUSB2"))
	}
	if gotMode != unix.S_IFCHR|0o600 {
		t.Fatalf("mknod mode = %#o, want %#o", gotMode, unix.S_IFCHR|0o600)
	}
	if gotDev != int(unix.Mkdev(188, 2)) {
		t.Fatalf("mknod dev = %d, want %d", gotDev, int(unix.Mkdev(188, 2)))
	}
	if gotUID != 1000 {
		t.Fatalf("chown uid = %d, want 1000", gotUID)
	}
	if chmodMode != 0o600 {
		t.Fatalf("chmod mode = %#o, want 0600", chmodMode)
	}
}

func TestEnsureTTYDeviceNodeRepairsExistingNodePermissions(t *testing.T) {
	origMknod := mknodFunc
	origChmod := chmodFunc
	origChown := chownFunc
	origDevRoot := devRoot
	origTTYClassRoot := ttyClassRoot
	origPKEXECUID, hadPKEXECUID := os.LookupEnv("PKEXEC_UID")
	t.Cleanup(func() {
		mknodFunc = origMknod
		chmodFunc = origChmod
		chownFunc = origChown
		devRoot = origDevRoot
		ttyClassRoot = origTTYClassRoot
		if hadPKEXECUID {
			_ = os.Setenv("PKEXEC_UID", origPKEXECUID)
		} else {
			_ = os.Unsetenv("PKEXEC_UID")
		}
	})

	root := t.TempDir()
	devRoot = filepath.Join(root, "dev")
	ttyClassRoot = filepath.Join(root, "sys", "class", "tty")
	mustMkdirAll(t, devRoot)
	mustWriteFile(t, filepath.Join(devRoot, "ttyUSB2"), "")
	mustWriteFile(t, filepath.Join(ttyClassRoot, "ttyUSB2", "dev"), "188:2\n")
	_ = os.Setenv("PKEXEC_UID", "1000")

	mknodFunc = func(path string, mode uint32, dev int) error {
		t.Fatalf("mknod should not be called for existing node")
		return nil
	}
	chownCalled := false
	chownFunc = func(path string, uid, gid int) error {
		chownCalled = true
		return nil
	}
	chmodCalled := false
	chmodFunc = func(path string, mode os.FileMode) error {
		chmodCalled = true
		return nil
	}

	if err := ensureTTYDeviceNode("ttyUSB2"); err != nil {
		t.Fatalf("ensureTTYDeviceNode() error = %v", err)
	}
	if !chownCalled {
		t.Fatalf("chown was not called")
	}
	if !chmodCalled {
		t.Fatalf("chmod was not called")
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
