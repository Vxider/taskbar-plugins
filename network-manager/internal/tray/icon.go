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
	case signalIconStandby:
		drawSignalBars(img, 1, 14, 0, active, inactive)
	case signalIconUnavailable:
		drawUnavailableMark(img, inactive)
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

func drawUnavailableMark(img *image.NRGBA, fill color.NRGBA) {
	points := [][2]int{
		{3, 4}, {4, 4}, {5, 4}, {6, 4},
		{6, 5}, {6, 6},
		{5, 7}, {4, 8}, {4, 9},
		{4, 12},
		{9, 4}, {9, 5}, {9, 6}, {9, 7}, {9, 8}, {9, 9}, {9, 10}, {9, 11}, {9, 12},
		{12, 4}, {12, 5}, {12, 6}, {12, 7}, {12, 8}, {12, 9}, {12, 10}, {12, 11}, {12, 12},
		{10, 4}, {11, 4}, {10, 8}, {11, 8}, {10, 12}, {11, 12},
	}
	for _, point := range points {
		img.SetNRGBA(point[0], point[1], fill)
	}
}
