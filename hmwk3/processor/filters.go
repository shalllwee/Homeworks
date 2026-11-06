package main

import (
	"encoding/json"
	"image"
	"image/color"

	"github.com/disintegration/imaging"
)

type FilterParams struct {
	Sigma float64 `json:"sigma"`
}

func parseParam(raw json.RawMessage) float64 {
	var p FilterParams
	if err := json.Unmarshal(raw, &p); err == nil && p.Sigma > 0 {
		return p.Sigma
	}
	return 2.0
}



func ApplyNegative(img image.Image) image.Image {
	bounds := img.Bounds()
	out := image.NewNRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			out.SetNRGBA(x, y, color.NRGBA{
				R: 255 - c.R,
				G: 255 - c.G,
				B: 255 - c.B,
				A: c.A,
			})
		}
	}
	return out
}

func ApplyFlipX(img image.Image) image.Image {
	bounds := img.Bounds()
	out := image.NewNRGBA(bounds)

	minY := bounds.Min.Y
	maxY := bounds.Max.Y

	for y := minY; y < maxY; y++ {
		flippedY := maxY - 1 - (y - minY)
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			out.Set(x, flippedY, img.At(x, y))
		}
	}
	return out
}

func ApplyBlur(img image.Image, rawParams json.RawMessage) image.Image {
	val := parseParam(rawParams)
	return imaging.Blur(img, val)
}

func ApplySharpen(img image.Image, rawParams json.RawMessage) image.Image {
	val := parseParam(rawParams)
	return imaging.Sharpen(img, val)
}