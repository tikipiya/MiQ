package gallery

import (
	"image"
	"math"
)

func BlockDifference(left, right image.Image, columns, rows int) float64 {
	bounds := left.Bounds()
	total := 0.0
	for row := 0; row < rows; row++ {
		for column := 0; column < columns; column++ {
			x0 := bounds.Min.X + column*bounds.Dx()/columns
			x1 := bounds.Min.X + (column+1)*bounds.Dx()/columns
			y0 := bounds.Min.Y + row*bounds.Dy()/rows
			y1 := bounds.Min.Y + (row+1)*bounds.Dy()/rows
			lr, lg, lb := blockAverage(left, x0, y0, x1, y1)
			rr, rg, rb := blockAverage(right, x0, y0, x1, y1)
			total += math.Abs(lr-rr) + math.Abs(lg-rg) + math.Abs(lb-rb)
		}
	}
	return total / float64(columns*rows*3)
}

func blockAverage(img image.Image, x0, y0, x1, y1 int) (float64, float64, float64) {
	r, g, b, count := 0.0, 0.0, 0.0, 0.0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			r += float64(cr) / 65535
			g += float64(cg) / 65535
			b += float64(cb) / 65535
			count++
		}
	}
	if count == 0 {
		return 0, 0, 0
	}
	return r / count, g / count, b / count
}
