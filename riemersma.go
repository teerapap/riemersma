//
// riemersma.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package riemersma

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

// Riemersma is a [draw.Drawer] (similar to [draw.FloydSteinberg]) which does Riemersma dithering to src image and draws result on dst image
var Riemersma = Drawer{}

type Drawer struct{}

func (dr Drawer) Draw(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point) {
	op := NewOperation(16, 16)
	op.Draw(dst, r, src, sp)
}

// Riemersma Dither Operation. It is not resuable.
type Op struct {
	Ratio   float64   // weight ratio between youngest pixel and oldest pixel
	Weights []float64 // pre-calculated weights

	errors errorList // most recent quantization errors
	x, y   int       // current dithering pixel
}

// Create new Riemersma dither operation with specific queueSize and ratio
// queueSize is the number of most recent pixel quantization errors to remember
// ratio is weight ratio between youngest pixel and oldest pixel
func NewOperation(queueSize int, ratio float64) *Op {
	return &Op{
		Ratio:   ratio,
		Weights: initWeights(queueSize, ratio),
		errors:  newErrorList(queueSize),
	}
}

func initWeights(size int, ratio float64) []float64 {
	weights := make([]float64, size)
	m := math.Exp(math.Log(ratio) / float64(size-1))

	v := 1.0
	for i := 0; i < size; i++ {
		weights[i] = math.Round(v)
		v = v * m
	}

	return weights
}

func (rs *Op) Draw(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point) {
	image := NewImage(dst, r, src, sp)
	rs.Dither(image)
}

func (rs *Op) Dither(image Image) {
	/* determine the required order of the Hilbert curve */
	imgSize := image.Size()
	sideLength := max(imgSize.X, imgSize.Y)
	level := log2(sideLength)
	if (1 << level) < sideLength {
		level += 1
	}

	if level > 0 {
		rs.hilbertLevel(level, dirUP, image)
	}
	rs.move(dirNONE, image)
}

func (rs *Op) hilbertLevel(level int, dir hilbertDirection, image Image) {
	if level == 1 {
		switch dir {
		case dirLEFT:
			rs.move(dirRIGHT, image)
			rs.move(dirDOWN, image)
			rs.move(dirLEFT, image)
		case dirRIGHT:
			rs.move(dirLEFT, image)
			rs.move(dirUP, image)
			rs.move(dirRIGHT, image)
		case dirUP:
			rs.move(dirDOWN, image)
			rs.move(dirRIGHT, image)
			rs.move(dirUP, image)
		case dirDOWN:
			rs.move(dirUP, image)
			rs.move(dirLEFT, image)
			rs.move(dirDOWN, image)
		}
	} else {
		switch dir {
		case dirLEFT:
			rs.hilbertLevel(level-1, dirUP, image)
			rs.move(dirRIGHT, image)
			rs.hilbertLevel(level-1, dirLEFT, image)
			rs.move(dirDOWN, image)
			rs.hilbertLevel(level-1, dirLEFT, image)
			rs.move(dirLEFT, image)
			rs.hilbertLevel(level-1, dirDOWN, image)
		case dirRIGHT:
			rs.hilbertLevel(level-1, dirDOWN, image)
			rs.move(dirLEFT, image)
			rs.hilbertLevel(level-1, dirRIGHT, image)
			rs.move(dirUP, image)
			rs.hilbertLevel(level-1, dirRIGHT, image)
			rs.move(dirRIGHT, image)
			rs.hilbertLevel(level-1, dirUP, image)
		case dirUP:
			rs.hilbertLevel(level-1, dirLEFT, image)
			rs.move(dirDOWN, image)
			rs.hilbertLevel(level-1, dirUP, image)
			rs.move(dirRIGHT, image)
			rs.hilbertLevel(level-1, dirUP, image)
			rs.move(dirUP, image)
			rs.hilbertLevel(level-1, dirRIGHT, image)
		case dirDOWN:
			rs.hilbertLevel(level-1, dirRIGHT, image)
			rs.move(dirUP, image)
			rs.hilbertLevel(level-1, dirDOWN, image)
			rs.move(dirLEFT, image)
			rs.hilbertLevel(level-1, dirDOWN, image)
			rs.move(dirDOWN, image)
			rs.hilbertLevel(level-1, dirLEFT, image)
		}
	}
}

func (rs *Op) move(dir hilbertDirection, image Image) {
	size := image.Size()
	numChannels := image.ColorNumChannels()

	/* dither the current pixel */
	if rs.x >= 0 && rs.x < size.X && rs.y >= 0 && rs.y < size.Y {
		newError := image.DitherPixel(rs.x, rs.y, rs.AccumulatedError(numChannels))
		rs.errors.Rotate(newError)
	}

	/* move to the next pixel */
	switch dir {
	case dirLEFT:
		rs.x--
	case dirRIGHT:
		rs.x++
	case dirUP:
		rs.y--
	case dirDOWN:
		rs.y++
	}
}

func (rs *Op) AccumulatedError(numChannel int) ColorError {
	acc := make(ColorError, numChannel)

	for i := 0; i < rs.errors.Size(); i++ {
		err := rs.errors.Get(i)
		w := rs.Weights[i]
		for j := 0; j < numChannel; j++ {
			if err == nil { // error is zero
				continue
			}
			acc[j] += err[j] * w
		}
	}
	for j := 0; j < numChannel; j++ {
		acc[j] /= rs.Ratio
	}
	return acc
}

type Image interface {
	Size() image.Point                                      // image size
	ColorNumChannels() int                                  // number of color channels
	DitherPixel(x int, y int, accErr ColorError) ColorError // Dither pixel with accumulated error
}

type anyImage struct {
	dst         draw.Image
	dp          image.Point
	src         image.Image
	sp          image.Point
	size        image.Point
	numChannels int
}

func NewImage(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point) Image {
	srcSize := src.Bounds().Max.Sub(sp)
	imgSize := image.Pt(min(srcSize.X, r.Dx()), min(srcSize.Y, r.Dy()))
	return anyImage{
		dst:         dst,
		dp:          r.Min,
		src:         src,
		sp:          sp,
		size:        imgSize,
		numChannels: 4,
	}
}

func (img anyImage) Size() image.Point {
	return img.size
}

func (img anyImage) ColorNumChannels() int {
	return img.numChannels
}

func (img anyImage) DitherPixel(x int, y int, accErr ColorError) ColorError {
	// Convert src color to  non-alpha-premultiplied 64-bit color
	sc := color.NRGBA64Model.Convert(img.src.At(img.sp.X+x, img.sp.Y+y)).(color.NRGBA64)

	// Adjust src color with accummulated quantization errors
	nc := color.NRGBA64{
		R: clamp(int32(sc.R) + int32(math.Round(accErr[0]))),
		G: clamp(int32(sc.G) + int32(math.Round(accErr[1]))),
		B: clamp(int32(sc.B) + int32(math.Round(accErr[2]))),
		A: clamp(int32(sc.A) + int32(math.Round(accErr[3]))),
	}

	// Set new color to destination. The color will be quantized.
	img.dst.Set(img.dp.X+x, img.dp.Y+y, nc)

	// Convert src color to  non-alpha-premultiplied 64-bit color
	dc := color.NRGBA64Model.Convert(img.dst.At(img.dp.X+x, img.dp.Y+y)).(color.NRGBA64)

	return ColorError{
		float64(sc.R) - float64(dc.R),
		float64(sc.G) - float64(dc.G),
		float64(sc.B) - float64(dc.B),
		float64(sc.A) - float64(dc.A),
	}
}

func clamp(i int32) uint16 {
	if i < 0 {
		return 0
	}
	if i > 0xffff {
		return 0xffff
	}
	return uint16(i)
}

func log2(value int) int {
	result := 0
	for value > 1 {
		value = value >> 1
		result += 1
	}
	return result
}

type hilbertDirection int

const (
	dirNONE = iota
	dirUP
	dirLEFT
	dirDOWN
	dirRIGHT
)

// color quantization errors for each channel
type ColorError []float64

type errorList struct {
	err  []ColorError
	head int
}

func newErrorList(size int) errorList {
	return errorList{
		err:  make([]ColorError, size),
		head: 0,
	}
}

func (el errorList) Get(i int) ColorError {
	return el.err[(el.head+i)%len(el.err)]
}

func (el errorList) Size() int {
	return len(el.err)
}

func (el *errorList) Rotate(val ColorError) {
	el.err[el.head] = val
	el.head += 1
	if el.head >= len(el.err) {
		el.head = 0
	}
}
