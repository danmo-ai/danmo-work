package computer

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"

	"danmo-work/core/domain"
)

// shrinkComputerImage downscales img so max(width,height) <= maxEdge and
// re-encodes as JPEG (q=82) to cut vision-token cost. DisplayWidth/Height keep
// the pre-scale screen size for click coordinate mapping. maxEdge <= 0 skips.
func shrinkComputerImage(img domain.ComputerImage, maxEdge int) domain.ComputerImage {
	if maxEdge <= 0 || img.Width <= 0 || img.Height <= 0 {
		if img.DisplayWidth == 0 {
			img.DisplayWidth = img.Width
		}
		if img.DisplayHeight == 0 {
			img.DisplayHeight = img.Height
		}
		return img
	}
	if img.DisplayWidth == 0 {
		img.DisplayWidth = img.Width
	}
	if img.DisplayHeight == 0 {
		img.DisplayHeight = img.Height
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

func mapShotCoord(x, y, imgW, imgH, dispW, dispH int) (int, int) {
	if imgW <= 0 || imgH <= 0 || dispW <= 0 || dispH <= 0 || (imgW == dispW && imgH == dispH) {
		return x, y
	}
	return x * dispW / imgW, y * dispH / imgH
}
