package computer

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"danmo-work/core/domain"
)

// shrinkComputerImage downscales img so max(width,height) <= maxEdge and
// re-encodes as JPEG (q=82) to cut vision-token cost. DisplayWidth/Height retain
// the captured region's size in mouse-coordinate units, never the raw image size.
func shrinkComputerImage(img domain.ComputerImage, maxEdge int) domain.ComputerImage {
	setDisplayDefaults(&img)
	if maxEdge <= 0 || img.Width <= 0 || img.Height <= 0 {
		return img
	}
	long := img.Width
	if img.Height > long {
		long = img.Height
	}
	if long <= maxEdge {
		return img
	}

	src, err := decodeRaster(img.Data)
	if err != nil {
		return img
	}
	nw := img.Width * maxEdge / long
	nh := img.Height * maxEdge / long
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := resizeNearest(src, nw, nh)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 82}); err != nil {
		return img
	}
	img.Data = buf.Bytes()
	img.MimeType = "image/jpeg"
	img.Width = nw
	img.Height = nh
	return img
}

func decodeRaster(data []byte) (image.Image, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err == nil {
		return img, nil
	}
	return jpeg.Decode(bytes.NewReader(data))
}

func resizeNearest(src image.Image, nw, nh int) *image.RGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		sy := b.Min.Y + y*sh/nh
		for x := 0; x < nw; x++ {
			sx := b.Min.X + x*sw/nw
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

func setDisplayDefaults(img *domain.ComputerImage) {
	if img.DisplayWidth <= 0 {
		img.DisplayWidth = img.Width
	}
	if img.DisplayHeight <= 0 {
		img.DisplayHeight = img.Height
	}
}

func mapShotCoord(x, y, imgW, imgH, dispW, dispH int) (int, int) {
	if imgW <= 0 || imgH <= 0 || dispW <= 0 || dispH <= 0 {
		return x, y
	}
	return x * dispW / imgW, y * dispH / imgH
}

// mapImagePoint translates an image-relative point to the input coordinate
// space. The origin is already in screen units and must never be scaled.
func mapImagePoint(img domain.ComputerImage, x, y int) (int, int, error) {
	if img.Width <= 0 || img.Height <= 0 || img.DisplayWidth <= 0 || img.DisplayHeight <= 0 {
		return 0, 0, fmt.Errorf("no valid screenshot geometry; take a screenshot before acting")
	}
	if x < 0 || y < 0 || x >= img.Width || y >= img.Height {
		return 0, 0, fmt.Errorf("coordinate (%d,%d) is outside the %dx%d screenshot; re-read the image before acting", x, y, img.Width, img.Height)
	}
	dx, dy := mapShotCoord(x, y, img.Width, img.Height, img.DisplayWidth, img.DisplayHeight)
	return img.OriginX + dx, img.OriginY + dy, nil
}
