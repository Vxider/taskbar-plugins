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
)

func trayIcon(fill color.NRGBA, mode signalIconMode, bars int) []byte {
	const (
		width  = 16
		height = 16
	)

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	inactive := color.NRGBA{R: 0x7A, G: 0x7A, B: 0x7A, A: 0xFF}

	switch mode {
	case signalIconOff:
		drawOffMark(img, inactive)
	case signalIconStandby:
		drawSignalBars(img, 1, 14, 0, fill, inactive)
	default:
		drawSignalBars(img, 1, 14, bars, fill, inactive)
	}

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil
	}
	return out.Bytes()
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
