package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

// The "Kesik" mark: three rounded slices. Their horizontal offsets encode
// the state, so the menu bar icon works as a single-colour template image.

type trayState int

const (
	trayOff trayState = iota
	trayOn
	trayScan0
	trayScan1
	trayScan2
	trayWarn
)

// slice is one bar in a 24x24 design grid.
type slice struct {
	x, y, w, h float64
	alpha      float64
}

func glyph(state trayState) []slice {
	const h = 5.0
	switch state {
	case trayOff:
		return []slice{{4.5, 3, 15, h, .55}, {4.5, 9.5, 15, h, .55}, {4.5, 16, 15, h, .55}}
	case trayScan0:
		return []slice{{2, 3, 15, h, 1}, {5, 9.5, 15, h, .6}, {8, 16, 15, h, .3}}
	case trayScan1:
		return []slice{{8, 3, 15, h, .3}, {2, 9.5, 15, h, 1}, {5, 16, 15, h, .6}}
	case trayScan2:
		return []slice{{5, 3, 15, h, .6}, {8, 9.5, 15, h, .3}, {2, 16, 15, h, 1}}
	case trayWarn:
		return []slice{
			{5, 3, 15, h, 1},
			{8, 9.5, 6, h, 1},
			{17, 9.5, 6, h, 1},
			{2, 16, 15, h, 1},
		}
	default: // on
		return []slice{{5, 3, 15, h, 1}, {8, 9.5, 15, h, 1}, {2, 16, 15, h, 1}}
	}
}

// renderGlyph draws the mark into a size×size RGBA image with the given
// padding (in design units) and colour. Edges are anti-aliased by
// supersampling.
func renderGlyph(state trayState, size int, pad float64, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	scale := float64(size) / (24 + 2*pad)
	const ss = 4
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var cov float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := (float64(px)+(float64(sx)+.5)/ss)/scale - pad
					y := (float64(py)+(float64(sy)+.5)/ss)/scale - pad
					for _, s := range glyph(state) {
						if inRoundRect(x, y, s.x, s.y, s.w, s.h, 1.2) {
							cov += s.alpha
							break
						}
					}
				}
			}
			a := cov / (ss * ss)
			if a <= 0 {
				continue
			}
			img.SetRGBA(px, py, color.RGBA{
				R: uint8(
					float64(c.R) * a,
				),
				G: uint8(float64(c.G) * a),
				B: uint8(float64(c.B) * a),
				A: uint8(255 * a),
			})
		}
	}
	return img
}

func inRoundRect(x, y, rx, ry, rw, rh, r float64) bool {
	if x < rx || y < ry || x > rx+rw || y > ry+rh {
		return false
	}
	cx := math.Max(rx+r, math.Min(x, rx+rw-r))
	cy := math.Max(ry+r, math.Min(y, ry+rh-r))
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= r*r
}

func pngBytes(img image.Image) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// trayIcons are pre-rendered template images (black + alpha), 44 px so they
// stay sharp on Retina menu bars.
var trayIcons = func() map[trayState][]byte {
	m := map[trayState][]byte{}
	for _, s := range []trayState{trayOff, trayOn, trayScan0, trayScan1, trayScan2, trayWarn} {
		m[s] = pngBytes(renderGlyph(s, 44, 2, color.RGBA{0, 0, 0, 255}))
	}
	return m
}()

// renderAppIcon draws direction A of the app icon: graphite squircle with
// three offset signal-green slices.
func renderAppIcon(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	bg := color.RGBA{0x14, 0x16, 0x1B, 0xFF}
	fg := color.RGBA{0xC8, 0xF2, 0x5A, 0xFF}
	// macOS icon grid: the tile is ~80% of the canvas.
	margin := float64(size) * 0.1
	tile := float64(size) - 2*margin
	radius := tile * 0.225
	marks := glyph(trayOn)
	const ss = 3
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var tA, gA float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := float64(px) + (float64(sx)+.5)/ss
					y := float64(py) + (float64(sy)+.5)/ss
					if !inRoundRect(x, y, margin, margin, tile, tile, radius) {
						continue
					}
					tA++
					gx := (x-margin)/tile*24*1.55 - 24*0.275
					gy := (y-margin)/tile*24*1.55 - 24*0.275
					for _, s := range marks {
						if inRoundRect(gx, gy, s.x, s.y, s.w, s.h, 1.2) {
							gA++
							break
						}
					}
				}
			}
			if tA == 0 {
				continue
			}
			ta, ga := tA/(ss*ss), gA/(ss*ss)
			mix := func(a, b uint8) uint8 {
				if ta == 0 {
					return 0
				}
				return uint8((float64(a)*(ta-ga) + float64(b)*ga) / ta)
			}
			img.SetRGBA(px, py, color.RGBA{
				R: uint8(
					float64(mix(bg.R, fg.R)) * ta,
				),
				G: uint8(float64(mix(bg.G, fg.G)) * ta),
				B: uint8(float64(mix(bg.B, fg.B)) * ta),
				A: uint8(255 * ta),
			})
		}
	}
	return img
}
