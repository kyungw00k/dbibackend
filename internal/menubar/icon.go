package menubar

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

func generateIcon(c color.Color) []byte {
	const size = 22
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := x - size/2
			dy := y - size/2
			if dx*dx+dy*dy <= (size/2-1)*(size/2-1) {
				img.Set(x, y, c)
			}
		}
	}

	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

var (
	iconDisconnected = generateIcon(color.RGBA{R: 180, G: 180, B: 180, A: 255})
	iconWaiting      = generateIcon(color.RGBA{R: 255, G: 180, B: 0, A: 255})
	iconConnected    = generateIcon(color.RGBA{R: 76, G: 175, B: 80, A: 255})
	iconTransferring = generateIcon(color.RGBA{R: 33, G: 150, B: 243, A: 255})
)
