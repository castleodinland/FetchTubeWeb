package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
)

// generateIcon creates a 256x256 PNG icon and wraps it in an ICO file.
// Design: dark rounded-rect background, YouTube-red circle, white play triangle.

func main() {
	sizes := []int{256, 128, 64, 48, 32, 16}
	var pngDatas [][]byte

	for _, sz := range sizes {
		img := renderIcon(sz)
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			panic(err)
		}
		pngDatas = append(pngDatas, buf.Bytes())
	}

	// Write ICO file
	out, _ := os.Create("app_icon.ico")
	defer out.Close()

	// ICO header
	writeLE(out, uint16(0))      // reserved
	writeLE(out, uint16(1))      // type: ICO
	writeLE(out, uint16(len(sizes))) // count

	// Calculate offset for image data
	offset := 6 + 16*len(sizes) // header + directory entries

	// Directory entries
	for i, sz := range sizes {
		data := pngDatas[i]
		sz8 := sz
		if sz8 > 256 {
			sz8 = 0
		}
		writeLE(out, uint8(sz8))     // width (0 means 256)
		writeLE(out, uint8(sz8))     // height
		writeLE(out, uint8(0))       // palette colors
		writeLE(out, uint8(0))       // reserved
		writeLE(out, uint16(1))      // color planes
		writeLE(out, uint16(32))     // bits per pixel
		writeLE(out, uint32(len(data))) // image size
		writeLE(out, uint32(offset)) // offset to image data
		offset += len(data)
	}

	// Image data (PNG for each size)
	for _, data := range pngDatas {
		out.Write(data)
	}
}

func writeLE(f *os.File, v interface{}) {
	binary.Write(f, binary.LittleEndian, v)
}

func renderIcon(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// --- Background: dark navy ---
	bgColor := color.RGBA{R: 0x14, G: 0x14, B: 0x2B, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	// --- YouTube-red centered circle ---
	cx := size / 2
	cy := size / 2
	circleR := float64(size) * 0.31
	redColor := color.RGBA{R: 0xFF, G: 0x00, B: 0x00, A: 255}
	triSize := float64(size) * 0.14
	triOffsetX := float64(size) * 0.016

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - float64(cx)
			dy := float64(y) - float64(cy)
			dist := math.Sqrt(dx*dx + dy*dy)

			if dist <= circleR {
				// Anti-alias circle edge
				alpha := 1.0
				edgeWidth := 1.5
				if dist > circleR-edgeWidth {
					alpha = (circleR - dist) / edgeWidth
					if alpha < 0 {
						alpha = 0
					}
				}

				// Blend red circle
				orig := img.RGBAAt(x, y)
				img.SetRGBA(x, y, blendRGBA(orig, redColor, alpha))

				// White play triangle inside the circle
				triCX := float64(cx) + triOffsetX
				triCY := float64(cy)
				leftX := triCX - triSize*0.5
				rightX := triCX + triSize*0.7
				topY := triCY - triSize
				bottomY := triCY + triSize

				if pointInTriangle(float64(x), float64(y), leftX, topY, rightX, triCY, leftX, bottomY) {
					d := distToTriangle(float64(x), float64(y), leftX, topY, rightX, triCY, leftX, bottomY)
					triAlpha := 1.0
					if d < 1.0 {
						triAlpha = d
					}
					if triAlpha > 0 {
						orig2 := img.RGBAAt(x, y)
						white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
						img.SetRGBA(x, y, blendRGBA(orig2, white, triAlpha))
					}
				}
			}
		}
	}

	// --- Rounded corners ---
	cornerR := float64(size) * 0.18
	maskBg := color.RGBA{A: 0} // transparent

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			alpha := roundedCornerAlpha(x, y, size, cornerR)
			if alpha < 1.0 {
				c := img.RGBAAt(x, y)
				img.SetRGBA(x, y, color.RGBA{
					R: uint8(float64(c.R) * alpha),
					G: uint8(float64(c.G) * alpha),
					B: uint8(float64(c.B) * alpha),
					A: uint8(float64(c.A) * alpha),
				})
				_ = maskBg
			}
		}
	}

	return img
}

func roundedCornerAlpha(x, y, size int, r float64) float64 {
	// Top-left
	if float64(x) < r && float64(y) < r {
		dx := r - float64(x)
		dy := r - float64(y)
		dist := math.Sqrt(dx*dx+dy*dy) / r
		if dist > 1.0 {
			return 0
		}
		return 1.0 - smoothstep(0.8, 1.0, dist)
	}
	// Top-right
	if float64(size-1-x) < r && float64(y) < r {
		dx := r - float64(size-1-x)
		dy := r - float64(y)
		dist := math.Sqrt(dx*dx+dy*dy) / r
		if dist > 1.0 {
			return 0
		}
		return 1.0 - smoothstep(0.8, 1.0, dist)
	}
	// Bottom-left
	if float64(x) < r && float64(size-1-y) < r {
		dx := r - float64(x)
		dy := r - float64(size-1-y)
		dist := math.Sqrt(dx*dx+dy*dy) / r
		if dist > 1.0 {
			return 0
		}
		return 1.0 - smoothstep(0.8, 1.0, dist)
	}
	// Bottom-right
	if float64(size-1-x) < r && float64(size-1-y) < r {
		dx := r - float64(size-1-x)
		dy := r - float64(size-1-y)
		dist := math.Sqrt(dx*dx+dy*dy) / r
		if dist > 1.0 {
			return 0
		}
		return 1.0 - smoothstep(0.8, 1.0, dist)
	}
	return 1.0
}

func smoothstep(edge0, edge1, x float64) float64 {
	t := (x - edge0) / (edge1 - edge0)
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}

func pointInTriangle(px, py, x1, y1, x2, y2, x3, y3 float64) bool {
	d1 := (px-x2)*(y1-y2) - (x1-x2)*(py-y2)
	d2 := (px-x3)*(y2-y3) - (x2-x3)*(py-y3)
	d3 := (px-x1)*(y3-y1) - (x3-x1)*(py-y1)
	hasNeg := (d1 < 0) || (d2 < 0) || (d3 < 0)
	hasPos := (d1 > 0) || (d2 > 0) || (d3 > 0)
	return !(hasNeg && hasPos)
}

func distToTriangle(px, py, x1, y1, x2, y2, x3, y3 float64) float64 {
	d1 := distToSegment(px, py, x1, y1, x2, y2)
	d2 := distToSegment(px, py, x2, y2, x3, y3)
	d3 := distToSegment(px, py, x3, y3, x1, y1)
	return min(min(d1, d2), d3)
}

func distToSegment(px, py, x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	if dx == 0 && dy == 0 {
		return math.Sqrt((px-x1)*(px-x1) + (py-y1)*(py-y1))
	}
	t := ((px-x1)*dx + (py-y1)*dy) / (dx*dx + dy*dy)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	nx := x1 + t*dx
	ny := y1 + t*dy
	return math.Sqrt((px-nx)*(px-nx) + (py-ny)*(py-ny))
}

func blendRGBA(bg, fg color.RGBA, alpha float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(fg.R)*alpha + float64(bg.R)*(1-alpha)),
		G: uint8(float64(fg.G)*alpha + float64(bg.G)*(1-alpha)),
		B: uint8(float64(fg.B)*alpha + float64(bg.B)*(1-alpha)),
		A: 255,
	}
}
