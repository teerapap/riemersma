//
// main.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"os"

	"github.com/teerapap/riemersma"
)

// Command-line Parsing
var help bool
var ratio float64
var queueSize uint
var colorDepth uint
var inputFilePath string
var outputFilePath string

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "%s [options]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.BoolVar(&help, "help", false, "show help")
	flag.BoolVar(&help, "h", false, "show help")
	flag.Float64Var(&ratio, "ratio", 16.0, "weight ratio between youngest pixel and oldest pixel")
	flag.UintVar(&queueSize, "size", 16, " the number of most recent pixel quantization errors to remember")
	flag.UintVar(&colorDepth, "depth", 1, " grayscale color depth in number of bits. Possible values are 1, 2, 4, 8 bits.")
	flag.StringVar(&inputFilePath, "i", "-", "input image file. '-' means stdin")
	flag.StringVar(&outputFilePath, "o", "-", "output image file. '-' means stdout")
}

func main() {
	// Parse command-line
	flag.Parse()

	if help {
		flag.Usage()
		os.Exit(0)
	}
	if ratio <= 0 {
		panic("Ratio must be positive")
	}
	switch colorDepth {
	case 1, 2, 4, 8:
		break
	default:
		panic(fmt.Sprintf("Unsupported color depth: %d", colorDepth))
	}
	inputFile := os.Stdin
	if inputFilePath != "-" {
		f, err := os.Open(inputFilePath)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		inputFile = f
	}
	outputFile := os.Stdout
	if inputFilePath != "-" {
		f, err := os.Open(outputFilePath)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		outputFile = f
	}

	// load image file
	src, format, err := image.Decode(inputFile)
	if err != nil {
		panic(err)
	}

	// setup destination
	var dst draw.Image
	if colorDepth == 8 {
		dst = image.NewGray(src.Bounds())
	} else {
		numColor := int(math.Pow(2.0, float64(colorDepth)))
		pal := make(color.Palette, numColor)
		step := 0xff / (numColor - 1)
		for i := 0; i < numColor; i++ {
			pal[i] = color.Gray{Y: uint8(step * i)}
		}
		dst = image.NewPaletted(src.Bounds(), pal)
	}

	// dither src image as dst image
	riemersma := riemersma.NewOperation(int(queueSize), ratio)
	riemersma.Draw(dst, dst.Bounds(), src, src.Bounds().Min)

	// save to output file

	switch format {
	case "png":
		err := png.Encode(outputFile, dst)
		if err != nil {
			panic(err)
		}
	default:
		err := jpeg.Encode(outputFile, dst, nil)
		if err != nil {
			panic(err)
		}
	}
}
