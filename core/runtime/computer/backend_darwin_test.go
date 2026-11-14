//go:build darwin

package computer

import (
	"image"
	"strings"
	"testing"

	"danmo-work/core/domain"
)

func TestDarwinWindowParsingUsesStableWindowID(t *testing.T) {
	wins := parseDarwinWindows("123\t77\tCursor\t-2560\t-839\t2560\t1440\tEditor", "")
	if len(wins) != 1 || wins[0].ID != "123" || wins[0].X != -2560 {
		t.Fatalf("windows=%+v", wins)
	}
}

func TestDarwinMultiDisplayStitchCoordinates(t *testing.T) {
	main := darwinCapture{display: darwinDisplay{x: 0, y: 0, width: 1728, height: 1117}, img: domain.ComputerImage{Width: 3456, Height: 2234, DisplayWidth: 1728, DisplayHeight: 1117}, data: image.NewRGBA(image.Rect(0, 0, 3456, 2234))}
	secondary := darwinCapture{display: darwinDisplay{x: -2560, y: -839, width: 2560, height: 1440}, img: domain.ComputerImage{Width: 5120, Height: 2880, DisplayWidth: 2560, DisplayHeight: 1440}, data: image.NewRGBA(image.Rect(0, 0, 5120, 2880))}
	_, w, h, screenW, screenH, x, y := stitchDisplays([]darwinCapture{main, secondary})
	if w != 8576 || h != 3912 || screenW != 4288 || screenH != 1956 || x != -2560 || y != -839 {
		t.Fatalf("geometry: image=%dx%d screen=%dx%d origin=%d,%d", w, h, screenW, screenH, x, y)
	}
	img := domain.ComputerImage{Width: w, Height: h, DisplayWidth: screenW, DisplayHeight: screenH, OriginX: x, OriginY: y}
	mappedX, mappedY, err := mapImagePoint(img, 2560*2, 839*2)
	if err != nil {
		t.Fatal(err)
	}
	if mappedX != 0 || mappedY != 0 {
		t.Fatalf("main-screen origin mapped to (%d,%d)", mappedX, mappedY)
	}
}

func TestDarwinKeyScriptPlusAndSpecialKeys(t *testing.T) {
	script, err := darwinKeyScript("shift++")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, "shift down") {
		t.Fatalf("script=%q", script)
	}
	for _, key := range []string{"F12", "Home", "End", "PageUp", "PageDown"} {
		if _, err := darwinKeyScript(key); err != nil {
			t.Errorf("%s: %v", key, err)
		}
	}
}
