package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/vxider/codex-buddy/uconsole/network-manager/internal/helper"
	"github.com/vxider/codex-buddy/uconsole/network-manager/internal/tray"
	"golang.org/x/sys/unix"
)

var instanceLockFile *os.File

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--helper" {
		if err := helper.Run(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	replaceExisting := hasArg("--replace-existing")
	ensureSessionEnv()
	logger := newLogger()
	if !acquireSingleInstance(logger, replaceExisting) {
		return
	}
	logger.Printf("startup env: XDG_RUNTIME_DIR=%q DBUS_SESSION_BUS_ADDRESS=%q DISPLAY=%q WAYLAND_DISPLAY=%q",
		os.Getenv("XDG_RUNTIME_DIR"),
		os.Getenv("DBUS_SESSION_BUS_ADDRESS"),
		os.Getenv("DISPLAY"),
		os.Getenv("WAYLAND_DISPLAY"),
	)
	installSignalHandler(logger)
	waitForStatusNotifierWatcher(logger)
	logTrayDBusState(logger, os.Getpid())
	tray.Run(logger)
	releaseSingleInstanceLock()
}

func hasArg(want string) bool {
	for _, arg := range os.Args[1:] {
		if arg == want {
			return true
		}
	}
	return false
}

func ensureSessionEnv() {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = filepath.Join("/run/user", fmt.Sprintf("%d", os.Getuid()))
		_ = os.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	}
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" && runtimeDir != "" {
		_ = os.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(runtimeDir, "bus"))
	}
}

func newLogger() *log.Logger {
	writers := []io.Writer{os.Stdout}

	if cacheDir, err := os.UserCacheDir(); err == nil {
		logDir := filepath.Join(cacheDir, "network-manager-tray")
		if err := os.MkdirAll(logDir, 0o755); err == nil {
			logPath := filepath.Join(logDir, "network-manager-tray.log")
			if file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
				writers = append(writers, file)
			}
		}
	}

	writer := io.MultiWriter(writers...)
	log.SetOutput(writer)
	log.SetFlags(log.LstdFlags)
	return log.New(writer, "network-manager: ", log.LstdFlags|log.Lmsgprefix)
}

func acquireSingleInstance(logger *log.Logger, replaceExisting bool) bool {
	runtimeDir := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR"))
	if runtimeDir == "" {
		logger.Printf("single-instance disabled: XDG_RUNTIME_DIR is empty")
		return true
	}

	lockPath := filepath.Join(runtimeDir, "network-manager-tray.lock")
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		logger.Printf("single-instance disabled: open lock file: %v", err)
		return true
	}

	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if replaceExisting {
			pid := readLockedPID(file)
			if pid > 0 && pid != os.Getpid() {
				logger.Printf("replace-existing requested; terminating prior instance pid=%d", pid)
				if killErr := syscall.Kill(pid, syscall.SIGTERM); killErr != nil && killErr != syscall.ESRCH {
					logger.Printf("replace-existing: terminate pid=%d failed: %v", pid, killErr)
				}
			}
			if waitForLock(file, 3*time.Second) != nil && pid > 0 && pid != os.Getpid() {
				logger.Printf("replace-existing: prior instance pid=%d did not exit after SIGTERM; killing", pid)
				if killErr := syscall.Kill(pid, syscall.SIGKILL); killErr != nil && killErr != syscall.ESRCH {
					logger.Printf("replace-existing: kill pid=%d failed: %v", pid, killErr)
				}
			}
			if waitForLock(file, 2*time.Second) == nil {
				writeLockedPID(file)
				instanceLockFile = file
				return true
			}
			logger.Printf("replace-existing requested but prior instance did not release lock")
		}
		logger.Printf("another instance is already running; exiting")
		_ = file.Close()
		return false
	}

	writeLockedPID(file)
	instanceLockFile = file
	return true
}

func readLockedPID(file *os.File) int {
	if _, err := file.Seek(0, 0); err != nil {
		return 0
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return 0
	}
	for i := len(data) - 1; i >= 0; i-- {
		if data[i] < '0' || data[i] > '9' {
			continue
		}
		end := i + 1
		for i >= 0 && data[i] >= '0' && data[i] <= '9' {
			i--
		}
		pid, err := strconv.Atoi(string(data[i+1 : end]))
		if err == nil && pid > 0 {
			return pid
		}
	}
	return 0
}

func writeLockedPID(file *os.File) {
	if err := file.Truncate(0); err != nil {
		return
	}
	if _, err := file.Seek(0, 0); err != nil {
		return
	}
	_, _ = file.WriteString(fmt.Sprintf("%d\n", os.Getpid()))
}

func waitForLock(file *os.File, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return syscall.EWOULDBLOCK
		}
		time.Sleep(150 * time.Millisecond)
	}
}

func installSignalHandler(logger *log.Logger) {
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-signals
		if logger != nil {
			logger.Printf("received signal %s; shutting down", sig)
		}
		releaseSingleInstanceLock()
		os.Exit(0)
	}()
}

func releaseSingleInstanceLock() {
	if instanceLockFile == nil {
		return
	}
	_ = unix.Flock(int(instanceLockFile.Fd()), unix.LOCK_UN)
	_ = instanceLockFile.Close()
	instanceLockFile = nil
}

func waitForStatusNotifierWatcher(logger *log.Logger) {
	for {
		ready, err := statusNotifierWatcherReady()
		if ready {
			return
		}
		if err != nil {
			logger.Printf("status notifier watcher not ready: %v", err)
		} else {
			logger.Printf("status notifier watcher not ready")
		}

		select {
		case <-time.After(time.Second):
		}
	}
}

func statusNotifierWatcherReady() (bool, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false, err
	}

	var names []string
	call := conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0)
	if call.Err != nil {
		return false, call.Err
	}
	if err := call.Store(&names); err != nil {
		return false, err
	}
	for _, name := range names {
		if name == "org.kde.StatusNotifierWatcher" {
			return true, nil
		}
	}
	return false, nil
}

func logTrayDBusState(logger *log.Logger, pid int) {
	expectedName := fmt.Sprintf("org.kde.StatusNotifierItem-%d-1", pid)
	for _, delay := range []time.Duration{time.Second, 4 * time.Second} {
		go func(delay time.Duration) {
			time.Sleep(delay)
			logTrayDBusSnapshot(logger, expectedName, delay)
		}(delay)
	}
}

func logTrayDBusSnapshot(logger *log.Logger, expectedName string, delay time.Duration) {
	conn, err := dbus.SessionBus()
	if err != nil {
		logger.Printf("tray dbus check after %s: session bus connect failed: %v", delay, err)
		return
	}

	var names []string
	call := conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0)
	if call.Err != nil {
		logger.Printf("tray dbus check after %s: ListNames failed: %v", delay, call.Err)
		return
	}
	if err := call.Store(&names); err != nil {
		logger.Printf("tray dbus check after %s: ListNames decode failed: %v", delay, err)
		return
	}

	hasWatcher := false
	hasExpectedItem := false
	for _, name := range names {
		if name == "org.kde.StatusNotifierWatcher" {
			hasWatcher = true
		}
		if name == expectedName {
			hasExpectedItem = true
		}
	}

	logger.Printf("tray dbus check after %s: watcher=%t expected_item=%t expected_name=%q", delay, hasWatcher, hasExpectedItem, expectedName)
	if !hasWatcher {
		return
	}

	var registered dbus.Variant
	call = conn.Object("org.kde.StatusNotifierWatcher", "/StatusNotifierWatcher").
		Call("org.freedesktop.DBus.Properties.Get", 0,
			"org.kde.StatusNotifierWatcher", "RegisteredStatusNotifierItems")
	if call.Err != nil {
		logger.Printf("tray dbus check after %s: watcher property read failed: %v", delay, call.Err)
		return
	}
	if err := call.Store(&registered); err != nil {
		logger.Printf("tray dbus check after %s: watcher property decode failed: %v", delay, err)
		return
	}

	items, ok := registered.Value().([]string)
	if !ok {
		logger.Printf("tray dbus check after %s: watcher items unexpected type %T", delay, registered.Value())
		return
	}
	logger.Printf("tray dbus check after %s: registered items=%v", delay, items)
}
