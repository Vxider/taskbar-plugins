package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestTrayIconProducesPNG(t *testing.T) {
	icon := trayIcon(true)
	if len(icon) == 0 {
		t.Fatal("trayIcon returned empty data")
	}
	if _, err := png.Decode(bytes.NewReader(icon)); err != nil {
		t.Fatalf("trayIcon did not return a valid PNG: %v", err)
	}
}

func TestTrayIconOfflineDrawsSlash(t *testing.T) {
	icon := trayIcon(false)
	img, err := png.Decode(bytes.NewReader(icon))
	if err != nil {
		t.Fatalf("decode tray icon: %v", err)
	}

	if !hasOpaquePixel(img, image.Rect(3, 3, 13, 13)) {
		t.Fatal("offline icon did not draw its slash")
	}
}

func TestTrayIconOnlineDoesNotDrawSlash(t *testing.T) {
	icon := trayIcon(true)
	img, err := png.Decode(bytes.NewReader(icon))
	if err != nil {
		t.Fatalf("decode tray icon: %v", err)
	}

	got := color.NRGBAModel.Convert(img.At(5, 11)).(color.NRGBA)
	if got == tailscaleOfflineMark {
		t.Fatal("online icon unexpectedly drew the offline slash")
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
