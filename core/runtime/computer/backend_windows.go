//go:build windows

package computer

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/png" // register PNG decoder for image.DecodeConfig
	"os"
	"strconv"
	"strings"

	"danmo-work/core/domain"
)

func newBackend() backend { return &windowsBackend{} }

// windowsBackend drives Windows via PowerShell, using Win32 P/Invoke
// (user32.dll) for window enumeration, focus, and input, and System.Drawing for
// screenshots. PowerShell is invoked fresh per action so Add-Type definitions do
// not collide across calls.
type windowsBackend struct {
	shell string
}

func (b *windowsBackend) ps() string {
	if b.shell != "" {
		return b.shell
	}
	if hasBinary("pwsh") {
		b.shell = "pwsh"
	} else {
		b.shell = "powershell"
	}
	return b.shell
}

func (b *windowsBackend) run(ctx context.Context, script string) (string, error) {
	return runCmd(ctx, nil, b.ps(), "-NoProfile", "-NonInteractive", "-Command", script)
}

func (b *windowsBackend) probe(_ domain.ConfigComputerSection) domain.ComputerStatus {
	st := domain.ComputerStatus{Platform: "windows", Backend: "windows"}
	if !hasBinary("powershell") && !hasBinary("pwsh") {
		st.DegradedReason = "PowerShell not found"
		return st
	}
	st.Available = true
	return st
}

const winUser32 = `
Add-Type @"
using System;
using System.Runtime.InteropServices;
using System.Text;
public class DQWin {
  [DllImport("user32.dll")] public static extern bool EnumWindows(EnumWindowsProc lpEnumFunc, IntPtr lParam);
  public delegate bool EnumWindowsProc(IntPtr hWnd, IntPtr lParam);
  [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern int GetWindowText(IntPtr hWnd, StringBuilder text, int count);
  [DllImport("user32.dll")] public static extern int GetWindowTextLength(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT lpRect);
  [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint pid);
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
  [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
  [DllImport("user32.dll")] public static extern void mouse_event(uint dwFlags, uint dx, uint dy, uint dwData, IntPtr dwExtraInfo);
  [DllImport("user32.dll")] public static extern bool GetCursorPos(out POINT p);
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int Left, Top, Right, Bottom; }
  [StructLayout(LayoutKind.Sequential)] public struct POINT { public int X, Y; }
}
"@
`

func (b *windowsBackend) listWindows(ctx context.Context, query string) ([]domain.WindowInfo, error) {
	script := winUser32 + `
$tab = [char]9
$sb = New-Object System.Text.StringBuilder
$cb = {
  param($h,$l)
  if ([DQWin]::IsWindowVisible($h)) {
    $len = [DQWin]::GetWindowTextLength($h)
    if ($len -gt 0) {
      $t = New-Object System.Text.StringBuilder ($len+1)
      [void][DQWin]::GetWindowText($h,$t,$t.Capacity)
      $r = New-Object DQWin+RECT
      [void][DQWin]::GetWindowRect($h,[ref]$r)
      $procId = 0
      [void][DQWin]::GetWindowThreadProcessId($h,[ref]$procId)
      $fields = @($h.ToInt64(),$procId,$r.Left,$r.Top,($r.Right-$r.Left),($r.Bottom-$r.Top),$t.ToString())
      [void]$sb.AppendLine(($fields -join $tab))
    }
  }
  return $true
}
[void][DQWin]::EnumWindows($cb,[IntPtr]::Zero)
$sb.ToString()
`
	out, err := b.run(ctx, script)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var wins []domain.WindowInfo
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(parts) < 7 {
			continue
		}
		info := domain.WindowInfo{ID: parts[0], Title: parts[6]}
		info.PID, _ = strconv.Atoi(parts[1])
		info.X, _ = strconv.Atoi(parts[2])
		info.Y, _ = strconv.Atoi(parts[3])
		info.Width, _ = strconv.Atoi(parts[4])
		info.Height, _ = strconv.Atoi(parts[5])
		if q != "" && !strings.Contains(strings.ToLower(info.Title), q) {
			continue
		}
		wins = append(wins, info)
	}
	return wins, nil
}

func (b *windowsBackend) windowBounds(ctx context.Context, id string) (domain.WindowInfo, error) {
	wins, err := b.listWindows(ctx, "")
	if err != nil {
		return domain.WindowInfo{}, err
	}
	for _, w := range wins {
		if w.ID == id {
			return w, nil
		}
	}
	return domain.WindowInfo{}, fmt.Errorf("window %q not found", id)
}

func (b *windowsBackend) focusWindow(ctx context.Context, id string) error {
	hwnd, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid window id %q", id)
	}
	script := winUser32 + fmt.Sprintf(`
$h = [IntPtr]%d
[void][DQWin]::ShowWindow($h, 9)
[void][DQWin]::SetForegroundWindow($h)
`, hwnd)
	_, err = b.run(ctx, script)
	return err
}

func (b *windowsBackend) screenshot(ctx context.Context, windowID string) (domain.ComputerImage, error) {
	origin := image.Point{}
	rect := ""
	if windowID != "" {
		info, err := b.windowBounds(ctx, windowID)
		if err != nil {
			return domain.ComputerImage{}, err
		}
		origin = image.Point{X: info.X, Y: info.Y}
		rect = fmt.Sprintf("%d %d %d %d", info.X, info.Y, info.Width, info.Height)
	}
	tmp, err := os.CreateTemp("", "dq-computer-*.png")
	if err != nil {
		return domain.ComputerImage{}, err
	}
	path := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(path)

	var script string
	if rect == "" {
		script = fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms,System.Drawing
$b = [System.Windows.Forms.SystemInformation]::VirtualScreen
$bmp = New-Object System.Drawing.Bitmap $b.Width, $b.Height
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.CopyFromScreen($b.Left, $b.Top, 0, 0, $bmp.Size)
$bmp.Save(%q, [System.Drawing.Imaging.ImageFormat]::Png)
`, path)
	} else {
		var x, y, w, h int
		fmt.Sscanf(rect, "%d %d %d %d", &x, &y, &w, &h)
		script = fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms,System.Drawing
$bmp = New-Object System.Drawing.Bitmap %d, %d
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.CopyFromScreen(%d, %d, 0, 0, $bmp.Size)
$bmp.Save(%q, [System.Drawing.Imaging.ImageFormat]::Png)
`, w, h, x, y, path)
	}
	if _, err := b.run(ctx, script); err != nil {
		return domain.ComputerImage{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.ComputerImage{}, err
	}
	w, h := 0, 0
	if cfg, _, decErr := image.DecodeConfig(bytes.NewReader(data)); decErr == nil {
		w, h = cfg.Width, cfg.Height
	}
	return domain.ComputerImage{MimeType: "image/png", Data: data, Width: w, Height: h, OriginX: origin.X, OriginY: origin.Y}, nil
}

func (b *windowsBackend) mouseMove(ctx context.Context, x, y int) error {
	script := winUser32 + fmt.Sprintf("[void][DQWin]::SetCursorPos(%d,%d)", x, y)
	_, err := b.run(ctx, script)
	return err
}

func (b *windowsBackend) mouseClick(ctx context.Context, button domain.MouseButton, x, y, clicks int) error {
	if x >= 0 && y >= 0 {
		if err := b.mouseMove(ctx, x, y); err != nil {
			return err
		}
	}
	down, up := "0x0002", "0x0004" // left
	switch button {
	case domain.MouseRight:
		down, up = "0x0008", "0x0010"
	case domain.MouseMiddle:
		down, up = "0x0020", "0x0040"
	}
	script := winUser32
	for i := 0; i < clicks; i++ {
		script += fmt.Sprintf("\n[DQWin]::mouse_event(%s,0,0,0,[IntPtr]::Zero)\n[DQWin]::mouse_event(%s,0,0,0,[IntPtr]::Zero)", down, up)
	}
	_, err := b.run(ctx, script)
	return err
}

func (b *windowsBackend) mouseDrag(ctx context.Context, x0, y0, x1, y1 int) error {
	script := winUser32 + fmt.Sprintf(`
[void][DQWin]::SetCursorPos(%d,%d)
[DQWin]::mouse_event(0x0002,0,0,0,[IntPtr]::Zero)
[void][DQWin]::SetCursorPos(%d,%d)
[DQWin]::mouse_event(0x0004,0,0,0,[IntPtr]::Zero)
`, x0, y0, x1, y1)
	_, err := b.run(ctx, script)
	return err
}

func (b *windowsBackend) scroll(ctx context.Context, x, y int, dir domain.ScrollDirection, amount int) error {
	if x >= 0 && y >= 0 {
		if err := b.mouseMove(ctx, x, y); err != nil {
			return err
		}
	}
	delta := -120 * amount // down
	flag := "0x0800"       // WHEEL
	switch dir {
	case domain.ScrollUp:
		delta = 120 * amount
	case domain.ScrollLeft:
		flag = "0x01000" // HWHEEL
		delta = -120 * amount
	case domain.ScrollRight:
		flag = "0x01000"
		delta = 120 * amount
	}
	script := winUser32 + fmt.Sprintf("[DQWin]::mouse_event(%s,0,0,%d,[IntPtr]::Zero)", flag, delta)
	_, err := b.run(ctx, script)
	return err
}

func (b *windowsBackend) typeText(ctx context.Context, text string) error {
	script := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
[System.Windows.Forms.SendKeys]::SendWait(%s)
`, psQuoteSendKeys(text))
	_, err := b.run(ctx, script)
	return err
}

func (b *windowsBackend) key(ctx context.Context, key string) error {
	seq := sendKeysChord(key)
	script := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
[System.Windows.Forms.SendKeys]::SendWait(%s)
`, psQuoteSendKeys(seq))
	_, err := b.run(ctx, script)
	return err
}

// psQuoteSendKeys single-quotes a string for PowerShell (doubling quotes).
func psQuoteSendKeys(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// sendKeysChord maps "ctrl+s" / "Return" to SendKeys syntax (^s, {ENTER}).
func sendKeysChord(key string) string {
	parts := strings.Split(key, "+")
	base := parts[len(parts)-1]
	prefix := ""
	for _, m := range parts[:len(parts)-1] {
		switch strings.ToLower(m) {
		case "ctrl", "control":
			prefix += "^"
		case "alt", "option":
			prefix += "%"
		case "shift":
			prefix += "+"
		}
	}
	special := map[string]string{
		"return": "{ENTER}", "enter": "{ENTER}", "tab": "{TAB}", "space": " ",
		"escape": "{ESC}", "esc": "{ESC}", "delete": "{DEL}", "backspace": "{BACKSPACE}",
		"up": "{UP}", "down": "{DOWN}", "left": "{LEFT}", "right": "{RIGHT}", "home": "{HOME}", "end": "{END}",
	}
	if s, ok := special[strings.ToLower(base)]; ok {
		return prefix + s
	}
	return prefix + base
}

func (b *windowsBackend) cursorPosition(ctx context.Context) (int, int, error) {
	script := winUser32 + `
$p = New-Object DQWin+POINT
[void][DQWin]::GetCursorPos([ref]$p)
"$($p.X),$($p.Y)"
`
	out, err := b.run(ctx, script)
	if err != nil {
		return 0, 0, err
	}
	xStr, yStr, ok := strings.Cut(strings.TrimSpace(out), ",")
	if !ok {
		return 0, 0, fmt.Errorf("unexpected cursor output %q", out)
	}
	x, _ := strconv.Atoi(strings.TrimSpace(xStr))
	y, _ := strconv.Atoi(strings.TrimSpace(yStr))
	return x, y, nil
}
