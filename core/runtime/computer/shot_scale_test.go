package computer

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"danmo-work/core/domain"
)

func TestShrinkComputerImageDownscales(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1280, 800))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	in := domain.ComputerImage{MimeType: "image/png", Data: buf.Bytes(), Width: 1280, Height: 800}
	out := shrinkComputerImage(in, 1024)
	if out.Width != 1024 || out.Height != 640 {
		t.Fatalf("got %dx%d", out.Width, out.Height)
	}
	if out.DisplayWidth != 1280 || out.DisplayHeight != 800 {
		t.Fatalf("display %dx%d", out.DisplayWidth, out.DisplayHeight)
	}
	if out.MimeType != "image/jpeg" {
		t.Fatalf("mime=%s", out.MimeType)
	}
	x, y := mapShotCoord(512, 320, out.Width, out.Height, out.DisplayWidth, out.DisplayHeight)
	if x != 640 || y != 400 {
		t.Fatalf("mapped %d,%d", x, y)
	}
}

func TestShrinkComputerImageNoopWhenSmall(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 80))
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	in := domain.ComputerImage{MimeType: "image/png", Data: buf.Bytes(), Width: 100, Height: 80}
	out := shrinkComputerImage(in, 1024)
	if out.Width != 100 || out.MimeType != "image/png" {
		t.Fatalf("%+v", out)
	}
}

func TestMapShotCoordIdentity(t *testing.T) {
	x, y := mapShotCoord(10, 20, 100, 100, 100, 100)
	if x != 10 || y != 20 {
		t.Fatal(x, y)
	}
}
