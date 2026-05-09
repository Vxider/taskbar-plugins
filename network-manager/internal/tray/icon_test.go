package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/vxider/codex-buddy/uconsole/network-manager/internal/configstate"
	"github.com/vxider/codex-buddy/uconsole/network-manager/internal/modemctl"
)

func TestSignalBarsFromQuality(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   int
		wantOK bool
	}{
		{name: "empty", input: "", want: 0, wantOK: false},
		{name: "zero", input: "0", want: 0, wantOK: true},
		{name: "low", input: "8", want: 1, wantOK: true},
		{name: "oneQuarter", input: "25", want: 2, wantOK: true},
		{name: "mid", input: "49", want: 2, wantOK: true},
		{name: "strong", input: "74", want: 3, wantOK: true},
		{name: "full", input: "75", want: 4, wantOK: true},
		{name: "clamped", input: "120", want: 4, wantOK: true},
		{name: "percentSuffix", input: "50%", want: 3, wantOK: true},
		{name: "invalid", input: "bad", want: 0, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := signalBarsFromQuality(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("signalBarsFromQuality(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("signalBarsFromQuality(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestTrayIconProducesPNG(t *testing.T) {
	icon := trayIcon(color.NRGBA{R: 0x2D, G: 0x9A, B: 0x5F, A: 0xFF}, signalIconBars, 3)
	if len(icon) == 0 {
		t.Fatal("trayIcon returned empty data")
	}
	if _, err := png.Decode(bytes.NewReader(icon)); err != nil {
		t.Fatalf("trayIcon did not return a valid PNG: %v", err)
	}
}

func TestTraySignalIcon(t *testing.T) {
	tests := []struct {
		name     string
		state    modemctl.State
		config   configstate.State
		writes   bool
		wantMode signalIconMode
		wantBars int
	}{
		{
			name:     "off",
			config:   configstate.State{ModemMode: configstate.ModeOff},
			writes:   true,
			wantMode: signalIconOff,
		},
		{
			name:     "standby",
			config:   configstate.State{ModemMode: configstate.ModeStandby},
			writes:   true,
			wantMode: signalIconStandby,
		},
		{
			name:     "onWithSignal",
			state:    modemctl.State{SignalQuality: "74"},
			config:   configstate.State{ModemMode: configstate.ModeOn},
			writes:   true,
			wantMode: signalIconBars,
			wantBars: 3,
		},
		{
			name:     "onWithoutSignal",
			config:   configstate.State{ModemMode: configstate.ModeOn},
			writes:   true,
			wantMode: signalIconUnavailable,
		},
		{
			name: "onWithModemError",
			state: modemctl.State{
				Installed: true,
				Error:     "mmcli reports no modems",
			},
			config:   configstate.State{ModemMode: configstate.ModeOn},
			writes:   true,
			wantMode: signalIconUnavailable,
		},
		{
			name: "readOnlyAutoStandbyWithSignal",
			state: modemctl.State{
				AltNetConnected: true,
				SignalQuality:   "74",
			},
			config:   configstate.State{ModemMode: configstate.ModeAuto},
			writes:   false,
			wantMode: signalIconBars,
			wantBars: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMode, gotBars := traySignalIcon(tt.state, tt.config, tt.writes)
			if gotMode != tt.wantMode || gotBars != tt.wantBars {
				t.Fatalf("traySignalIcon() = (%v, %d), want (%v, %d)", gotMode, gotBars, tt.wantMode, tt.wantBars)
			}
		})
	}
}

func TestTrayIconOffHasNoSignalBarsAtLeft(t *testing.T) {
	icon := trayIcon(color.NRGBA{R: 0x2D, G: 0x9A, B: 0x5F, A: 0xFF}, signalIconOff, 4)
	img, err := png.Decode(bytes.NewReader(icon))
	if err != nil {
		t.Fatalf("decode tray icon: %v", err)
	}

	if hasOpaquePixel(img, image.Rect(1, 4, 3, 14)) {
		t.Fatal("off icon unexpectedly drew signal bars in the left-most bar area")
	}
}

func TestTrayIconUsesGrayForInactiveBars(t *testing.T) {
	icon := trayIcon(color.NRGBA{R: 0x2B, G: 0x84, B: 0xC6, A: 0xFF}, signalIconBars, 1)
	img, err := png.Decode(bytes.NewReader(icon))
	if err != nil {
		t.Fatalf("decode tray icon: %v", err)
	}

	got := color.NRGBAModel.Convert(img.At(5, 14)).(color.NRGBA)
	want := signalInactiveColor()
	if got != want {
		t.Fatalf("inactive bar color = %#v, want %#v", got, want)
	}
}

func TestTrayIconUsesBlueForActiveBars(t *testing.T) {
	icon := trayIcon(color.NRGBA{R: 0xC7, G: 0x83, B: 0x19, A: 0xFF}, signalIconBars, 2)
	img, err := png.Decode(bytes.NewReader(icon))
	if err != nil {
		t.Fatalf("decode tray icon: %v", err)
	}

	got := color.NRGBAModel.Convert(img.At(1, 14)).(color.NRGBA)
	want := signalActiveColor()
	if got != want {
		t.Fatalf("active bar color = %#v, want %#v", got, want)
	}
}

func TestTrayIconUnavailableShowsGrayBarsWithRedQuestionMark(t *testing.T) {
	icon := trayIcon(color.NRGBA{R: 0x2D, G: 0x9A, B: 0x5F, A: 0xFF}, signalIconUnavailable, 4)
	img, err := png.Decode(bytes.NewReader(icon))
	if err != nil {
		t.Fatalf("decode tray icon: %v", err)
	}

	if got := color.NRGBAModel.Convert(img.At(1, 14)).(color.NRGBA); got != signalInactiveColor() {
		t.Fatalf("unavailable first bar color = %#v, want %#v", got, signalInactiveColor())
	}
	if got := color.NRGBAModel.Convert(img.At(7, 11)).(color.NRGBA); got != signalWarningColor() {
		t.Fatalf("unavailable question mark color = %#v, want %#v", got, signalWarningColor())
	}
}

func hasOpaquePixel(img image.Image, rect image.Rectangle) bool {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a != 0 {
				return true
			}
		}
	}
	return false
}
