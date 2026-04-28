package helper

import (
	"errors"
	"testing"
	"time"
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
	origSend := sendATFunc
	origSleep := sleepFunc
	t.Cleanup(func() {
		findATPortFunc = origFind
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
