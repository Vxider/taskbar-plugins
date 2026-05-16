package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

type signalIconMode int

const (
	signalIconBars signalIconMode = iota
	signalIconStandby
	signalIconOff
	signalIconRadioDisabled
	signalIconUnavailable
)

func trayIcon(fill color.NRGBA, mode signalIconMode, bars int) []byte {
	const (
		width  = 16
		height = 16
	)
	_ = fill

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	active := signalActiveColor()
	inactive := signalInactiveColor()

	switch mode {
	case signalIconOff:
		drawOffMark(img, inactive)
	case signalIconRadioDisabled:
		drawSignalBars(img, 1, 14, 0, active, inactive)
		drawSlashMark(img, signalRadioDisabledColor())
	case signalIconStandby:
		drawSignalBars(img, 1, 14, 0, active, inactive)
	case signalIconUnavailable:
		drawSignalBars(img, 1, 14, 0, active, inactive)
		drawQuestionMark(img, signalWarningColor())
	default:
		drawSignalBars(img, 1, 14, bars, active, inactive)
	}

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil
	}
	return out.Bytes()
}

func signalActiveColor() color.NRGBA {
	return color.NRGBA{R: 0x2B, G: 0x84, B: 0xC6, A: 0xFF}
}

func signalInactiveColor() color.NRGBA {
	return color.NRGBA{R: 0x7A, G: 0x7A, B: 0x7A, A: 0xFF}
}

func signalWarningColor() color.NRGBA {
	return color.NRGBA{R: 0xD6, G: 0x32, B: 0x32, A: 0xFF}
}

func signalRadioDisabledColor() color.NRGBA {
	return color.NRGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0xFF}
}

func drawSignalBars(img *image.NRGBA, x0, baseline, bars int, active, inactive color.NRGBA) {
	if bars < 0 {
		bars = 0
	}
	if bars > 4 {
		bars = 4
	}

	heights := []int{4, 7, 10, 13}
	const (
		barWidth = 3
		gap      = 1
	)
	for i, height := range heights {
		fill := inactive
		if i < bars {
			fill = active
		}

		left := x0 + i*(barWidth+gap)
		top := baseline - height + 1
		for x := left; x < left+barWidth; x++ {
			for y := top; y <= baseline; y++ {
				img.SetNRGBA(x, y, fill)
			}
		}
	}
}

func drawSlashMark(img *image.NRGBA, fill color.NRGBA) {
	for x, y := 3, 13; x <= 13; x, y = x+1, y-1 {
		img.SetNRGBA(x, y, fill)
		if x+1 < img.Bounds().Max.X {
			img.SetNRGBA(x+1, y, fill)
		}
	}
}

func drawOffMark(img *image.NRGBA, fill color.NRGBA) {
	points := [][2]int{
		{4, 4}, {5, 5}, {6, 6}, {7, 7}, {8, 8}, {9, 9}, {10, 10},
		{10, 4}, {9, 5}, {8, 6}, {7, 7}, {6, 8}, {5, 9}, {4, 10},
		{5, 4}, {9, 4}, {4, 5}, {10, 5}, {4, 9}, {10, 9}, {5, 10}, {9, 10},
	}
	for _, point := range points {
		img.SetNRGBA(point[0], point[1], fill)
	}
}

func drawQuestionMark(img *image.NRGBA, fill color.NRGBA) {
	points := [][2]int{
		{5, 1}, {6, 1}, {7, 1}, {8, 1}, {9, 1},
		{4, 2}, {5, 2}, {9, 2}, {10, 2},
		{4, 3}, {10, 3},
		{9, 4}, {10, 4},
		{8, 5}, {9, 5},
		{7, 6}, {8, 6},
		{6, 7}, {7, 7},
		{6, 8}, {7, 8},
		{6, 11}, {7, 11},
		{6, 12}, {7, 12},
	}
	for _, point := range points {
		img.SetNRGBA(point[0], point[1], fill)
	}
}
