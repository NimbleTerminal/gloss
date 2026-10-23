package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"

	"nimbleterminal.dev/ntmaps/tile"
	"nimbleterminal.dev/ntmaps/tile/pmtiles"
)

// islands is a made-up world as a raster PMTiles archive, zooms 0 to 3:
// seas and islands from a few waves over latitude and longitude, banded by
// height, with a graticule every 30 degrees. It has no real geography, so
// it owes no map data's license.
func islands() []byte {
	palette := color.Palette{
		color.NRGBA{164, 199, 226, 255}, // deep sea
		color.NRGBA{190, 218, 238, 255}, // shallows
		color.NRGBA{214, 226, 180, 255}, // lowland
		color.NRGBA{180, 204, 150, 255}, // hills
		color.NRGBA{160, 148, 120, 255}, // mountains
		color.NRGBA{120, 140, 165, 255}, // graticule
	}
	height := func(lat, lng float64) float64 {
		r := math.Pi / 180
		return math.Sin(lng*r*3)*math.Cos(lat*r*4) + 0.6*math.Sin(lng*r*7+1)*math.Sin(lat*r*5+2) + 0.3*math.Cos(lng*r*13-lat*r*11)
	}
	tiles := map[tile.ID][]byte{}
	for z := 0; z <= 3; z++ {
		n := 1 << z
		for ty := 0; ty < n; ty++ {
			for tx := 0; tx < n; tx++ {
				img := image.NewPaletted(image.Rect(0, 0, 256, 256), palette)
				world := float64(256 * n)
				for y := 0; y < 256; y++ {
					lat := 180 / math.Pi * math.Atan(math.Sinh(math.Pi*(1-2*(float64(ty*256+y)+0.5)/world)))
					for x := 0; x < 256; x++ {
						lng := (float64(tx*256+x)+0.5)/world*360 - 180
						h := height(lat, lng)
						i := uint8(0)
						switch {
						case h > 1.1:
							i = 4
						case h > 0.75:
							i = 3
						case h > 0.45:
							i = 2
						case h > 0.3:
							i = 1
						}
						// One pixel either side of each 30th degree, at any zoom.
						step := 360 / world
						if math.Abs(math.Remainder(lng, 30)) < step || math.Abs(math.Remainder(lat, 30)) < step*math.Cos(lat*math.Pi/180) {
							i = 5
						}
						img.SetColorIndex(x, y, i)
					}
				}
				var b bytes.Buffer
				if err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&b, img); err != nil {
					panic(err)
				}
				tiles[tile.ID{Z: z, X: tx, Y: ty}] = b.Bytes()
			}
		}
	}
	var out bytes.Buffer
	if err := pmtiles.Write(&out, tiles, pmtiles.WriteOptions{Name: "Islands", Attribution: "gloss examples"}); err != nil {
		panic(err)
	}
	return out.Bytes()
}
