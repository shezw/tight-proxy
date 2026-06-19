package icon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/png"
)

func LightningCirclePNG() []byte {
	return lightningCirclePNG(64, color.NRGBA{R: 30, G: 107, B: 255, A: 255}, color.NRGBA{R: 255, G: 205, B: 64, A: 255})
}

func LightningCircleTemplatePNG() []byte {
	black := color.NRGBA{A: 255}
	return lightningCirclePNG(32, black, black)
}

func LightningCircleICO() []byte {
	sizes := []int{16, 24, 32, 48, 64}
	images := make([][]byte, 0, len(sizes))
	for _, size := range sizes {
		images = append(images, lightningCirclePNG(size, color.NRGBA{R: 30, G: 107, B: 255, A: 255}, color.NRGBA{R: 255, G: 205, B: 64, A: 255}))
	}

	const headerSize = 6
	const entrySize = 16
	offset := headerSize + entrySize*len(images)
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, uint16(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(len(images)))
	for index, data := range images {
		size := sizes[index]
		if size >= 256 {
			buf.WriteByte(0)
		} else {
			buf.WriteByte(byte(size))
		}
		if size >= 256 {
			buf.WriteByte(0)
		} else {
			buf.WriteByte(byte(size))
		}
		buf.WriteByte(0)
		buf.WriteByte(0)
		_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
		_ = binary.Write(&buf, binary.LittleEndian, uint16(32))
		_ = binary.Write(&buf, binary.LittleEndian, uint32(len(data)))
		_ = binary.Write(&buf, binary.LittleEndian, uint32(offset))
		offset += len(data)
	}
	for _, data := range images {
		buf.Write(data)
	}
	return buf.Bytes()
}

func lightningCirclePNG(size int, ring, boltColor color.NRGBA) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), image.Transparent, image.Point{}, draw.Src)
	cx, cy := size/2, size/2
	outerRadius := size*7/16 + 1
	innerRadius := size*11/32 + 1
	outer := outerRadius * outerRadius
	inner := innerRadius * innerRadius
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := x-cx, y-cy
			d := dx*dx + dy*dy
			if d <= outer && d >= inner {
				img.SetNRGBA(x, y, ring)
			}
		}
	}
	bolt := scalePoints(size, []image.Point{
		{35, 8}, {18, 35}, {30, 35},
		{25, 56}, {46, 27}, {34, 27},
	})
	fillPolygon(img, bolt, boltColor)
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func scalePoints(size int, pts []image.Point) []image.Point {
	scaled := make([]image.Point, 0, len(pts))
	for _, pt := range pts {
		scaled = append(scaled, image.Point{
			X: pt.X * size / 64,
			Y: pt.Y * size / 64,
		})
	}
	return scaled
}

func fillPolygon(img *image.NRGBA, pts []image.Point, c color.NRGBA) {
	if len(pts) < 3 {
		return
	}
	minY, maxY := pts[0].Y, pts[0].Y
	for _, p := range pts {
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	for y := minY; y <= maxY; y++ {
		var nodes []int
		j := len(pts) - 1
		for i := range pts {
			if (pts[i].Y < y && pts[j].Y >= y) || (pts[j].Y < y && pts[i].Y >= y) {
				x := pts[i].X + (y-pts[i].Y)*(pts[j].X-pts[i].X)/(pts[j].Y-pts[i].Y)
				nodes = append(nodes, x)
			}
			j = i
		}
		for i := 0; i+1 < len(nodes); i += 2 {
			if nodes[i] > nodes[i+1] {
				nodes[i], nodes[i+1] = nodes[i+1], nodes[i]
			}
			for x := nodes[i]; x < nodes[i+1]; x++ {
				img.SetNRGBA(x, y, c)
			}
		}
	}
}
