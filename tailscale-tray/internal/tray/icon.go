package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

type iconDot struct {
	cx    float64
	cy    float64
	r     float64
	alpha uint8
}

var tailscaleDots = [...]iconDot{
	{cx: 72, cy: 72, r: 18, alpha: 0x66},
	{cx: 126, cy: 72, r: 18, alpha: 0x66},
	{cx: 180, cy: 72, r: 18, alpha: 0x66},
	{cx: 72, cy: 126, r: 18, alpha: 0xFF},
	{cx: 126, cy: 126, r: 18, alpha: 0xFF},
	{cx: 180, cy: 126, r: 18, alpha: 0xFF},
	{cx: 72, cy: 180, r: 18, alpha: 0x66},
	{cx: 126, cy: 180, r: 18, alpha: 0xFF},
	{cx: 180, cy: 180, r: 18, alpha: 0x66},
}

var tailscaleDotFill = color.NRGBA{R: 0xF3, G: 0xF4, B: 0xF6, A: 0xFF}
var tailscaleOfflineMark = color.NRGBA{R: 0xE5, G: 0x3E, B: 0x3E, A: 0xFF}

func trayIcon(online bool) []byte {
	// Match the source icon size used by network-manager so tray hosts scale
	// both plugins consistently, while keeping this mark close to the bounds.
	const size = 16
	const supersample = 8
	const iconViewBox = 252.0
	const canvasViewBox = 216.0
	const iconOffset = (canvasViewBox - iconViewBox) / 2
	const dotScale = 1.4
	const dotCenter = iconViewBox / 2

	hiSize := size * supersample
	img := image.NewNRGBA(image.Rect(0, 0, hiSize, hiSize))

	for y := 0; y < hiSize; y++ {
		vy := (float64(y) + 0.5) * canvasViewBox / float64(hiSize)
		for x := 0; x < hiSize; x++ {
			vx := (float64(x) + 0.5) * canvasViewBox / float64(hiSize)
			for _, dot := range tailscaleDots {
				scaledCX := dotCenter + (dot.cx-dotCenter)*dotScale
				scaledCY := dotCenter + (dot.cy-dotCenter)*dotScale
				scaledR := dot.r * dotScale
				dx := (vx - iconOffset) - scaledCX
				dy := (vy - iconOffset) - scaledCY
				if dx*dx+dy*dy <= scaledR*scaledR {
					img.SetNRGBA(x, y, color.NRGBA{
						R: tailscaleDotFill.R,
						G: tailscaleDotFill.G,
						B: tailscaleDotFill.B,
						A: dot.alpha,
					})
					break
				}
			}
		}
	}

	if !online {
		drawOfflineSlash(img, tailscaleOfflineMark)
	}

	outImg := image.NewNRGBA(image.Rect(0, 0, size, size))
	samplesPerPixel := supersample * supersample

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var sumRA, sumGA, sumBA, sumA int
			for sy := 0; sy < supersample; sy++ {
				for sx := 0; sx < supersample; sx++ {
					sample := img.NRGBAAt(x*supersample+sx, y*supersample+sy)
					a := int(sample.A)
					sumRA += int(sample.R) * a
					sumGA += int(sample.G) * a
					sumBA += int(sample.B) * a
					sumA += a
				}
			}

			alpha := uint8(sumA / samplesPerPixel)
			if alpha == 0 {
				continue
			}

			avgA := sumA / samplesPerPixel
			outImg.SetNRGBA(x, y, color.NRGBA{
				R: uint8((sumRA / samplesPerPixel) / avgA),
				G: uint8((sumGA / samplesPerPixel) / avgA),
				B: uint8((sumBA / samplesPerPixel) / avgA),
				A: alpha,
			})
		}
	}

	var out bytes.Buffer
	if err := png.Encode(&out, outImg); err != nil {
		return nil
	}
	return out.Bytes()
}

func drawOfflineSlash(img *image.NRGBA, fill color.NRGBA) {
	points := slashPoints(image.Rect(2, 2, 14, 14))
	for _, point := range points {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				x := point[0] + dx
				y := point[1] + dy
				if image.Pt(x, y).In(img.Bounds()) {
					img.SetNRGBA(x, y, fill)
				}
			}
		}
	}
}

func slashPoints(rect image.Rectangle) [][2]int {
	x0 := rect.Min.X
	y0 := rect.Max.Y - 1
	x1 := rect.Max.X - 1
	y1 := rect.Min.Y

	dx := x1 - x0
	dy := y1 - y0
	steps := int(math.Max(math.Abs(float64(dx)), math.Abs(float64(dy))))
	if steps == 0 {
		return [][2]int{{x0, y0}}
	}

	points := make([][2]int, 0, steps+1)
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x := int(math.Round(float64(x0) + t*float64(dx)))
		y := int(math.Round(float64(y0) + t*float64(dy)))
		points = append(points, [2]int{x, y})
	}
	return points
}
