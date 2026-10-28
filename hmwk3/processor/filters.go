package main

import (
	"image"
	"image/color"

	"github.com/disintegration/imaging"
)

func ApplyNegative(img image.Image) image.Image {
	bounds := img.Bounds()
	out := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			out.Set(x, y, color.RGBA{
				R: 255 - uint8(r>>8),
				G: 255 - uint8(g>>8),
				B: 255 - uint8(b>>8),
				A: uint8(a >> 8),
			})
		}
	}
	return out
}

func ApplyFlipX(img image.Image) image.Image {
	bounds := img.Bounds()
	out := image.NewRGBA(bounds)

	width := bounds.Max.X
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)
			flippedX := width - 1 - (x - bounds.Min.X)
			out.Set(flippedX, y, c)
		}
	}
	return out
}

func ApplyBlur(img image.Image) image.Image {
	return imaging.Blur(img, 2.0)
}

func ApplySharpen(img image.Image) image.Image {
	return imaging.Sharpen(img, 2.0)
}