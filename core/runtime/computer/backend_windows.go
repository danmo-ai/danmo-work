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
	return runCmd(ctx, nil, b.ps(), "-NoProfile", "-NonInteractive", "-Command", "[Console]::OutputEncoding = [System.Text.Encoding]::UTF8\n$OutputEncoding = [System.Text.Encoding]::UTF8\n"+script)
}

func (b *windowsBackend) runWin(ctx context.Context, script string) (string, error) {
	return b.run(ctx, winUser32+"\n[void][DQWin]::SetProcessDPIAware()\n"+script)
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
Add-Type -AssemblyName System.Windows.Forms
Add-Type @"
using System;
using System.Runtime.InteropServices;
using System.Text;
public class DQWin {
  [DllImport("user32.dll")] public static extern bool SetProcessDPIAware();
  [DllImport("user32.dll")] public static extern bool AttachThreadInput(uint from, uint to, bool attach);
  [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
  [DllImport("user32.dll")] public static extern bool BringWindowToTop(IntPtr hWnd);
  [DllImport("kernel32.dll")] public static extern uint GetCurrentThreadId();
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
  [DllImport("user32.dll")] public static extern void mouse_event(uint dwFlags, uint dx, uint dy, int dwData, IntPtr dwExtraInfo);
  [DllImport("user32.dll")] public static extern bool GetCursorPos(out POINT p);
  [DllImport("user32.dll")] public static extern void keybd_event(byte vk, byte scan, uint flags, UIntPtr extra);
  [DllImport("user32.dll")] public static extern uint SendInput(uint count, INPUT[] inputs, int size);
  [StructLayout(LayoutKind.Sequential)] public struct INPUT { public uint type; public INPUTUNION data; }
  [StructLayout(LayoutKind.Explicit)] public struct INPUTUNION { [FieldOffset(0)] public KEYBDINPUT keyboard; }
  [StructLayout(LayoutKind.Sequential)] public struct KEYBDINPUT { public ushort vk; public ushort scan; public uint flags; public uint time; public IntPtr extra; }
  public static uint TypeUnicode(string text) {
    var inputs = new INPUT[text.Length * 2];
    for (int i = 0; i < text.Length; i++) {
      inputs[i*2].type = 1;
      inputs[i*2].data.keyboard = new KEYBDINPUT { scan = text[i], flags = 0x0004 };
      inputs[i*2+1].type = 1;
      inputs[i*2+1].data.keyboard = new KEYBDINPUT { scan = text[i], flags = 0x0004 | 0x0002 };
    }
    return SendInput((uint)inputs.Length, inputs, Marshal.SizeOf(typeof(INPUT)));
  }
  public static void KeyDown(byte vk) { keybd_event(vk, 0, 0, UIntPtr.Zero); }
  public static void KeyUp(byte vk) { keybd_event(vk, 0, 0x0002, UIntPtr.Zero); }
  [DllImport("user32.dll")] public static extern short VkKeyScan(char ch);
  public static void ChordKey(byte vk, bool down) { if (down) KeyDown(vk); else KeyUp(vk); }
  public static void Chord(string chord) {
    string[] parts = chord.Split('\t');
    if (parts.Length != 2) throw new ArgumentException("expected key and modifiers");
    var modifiers = new System.Collections.Generic.List<byte>();
    foreach (string mod in parts[1].Split(',')) {
      if (mod == "shift") modifiers.Add(0x10);
      else if (mod == "ctrl") modifiers.Add(0x11);
      else if (mod == "alt") modifiers.Add(0x12);
      else if (mod == "meta") modifiers.Add(0x5B);
    }
    var keys = new System.Collections.Generic.List<byte>();
    foreach (string name in parts[0].ToLowerInvariant().Split('+')) {
      byte vk = 0;
      switch (name) {
        case "return": case "enter": vk = 0x0D; break;
        case "tab": vk = 0x09; break; case "space": vk = 0x20; break;
        case "escape": case "esc": vk = 0x1B; break;
        case "delete": vk = 0x2E; break; case "backspace": vk = 0x08; break;
        case "up": vk = 0x26; break; case "down": vk = 0x28; break;
        case "left": vk = 0x25; break; case "right": vk = 0x27; break;
        case "home": vk = 0x24; break; case "end": vk = 0x23; break;
        case "pageup": vk = 0x21; break; case "pagedown": vk = 0x22; break;
        default:
          if (name.Length == 2 && name[0] == 'f' && name[1] >= '1' && name[1] <= '9') vk = (byte)(0x70 + name[1] - '1');
          else if (name.Length == 3 && name.StartsWith("f1") && name[2] <= '2') vk = (byte)(0x70 + 9 + name[2] - '0');
          else if (name.Length == 1) { short scan = VkKeyScan(name[0]); if (scan == -1) throw new ArgumentException("unknown key " + name); vk = (byte)(scan & 0xff); }
          else throw new ArgumentException("unknown key " + name);
          break;
      }
      keys.Add(vk);
    }
    foreach (byte vk in modifiers) KeyDown(vk);
    foreach (byte vk in keys) KeyDown(vk);
    System.Threading.Thread.Sleep(35);
    keys.Reverse(); foreach (byte vk in keys) KeyUp(vk);
    modifiers.Reverse(); foreach (byte vk in modifiers) KeyUp(vk);
  }
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int Left, Top, Right, Bottom; }
  [StructLayout(LayoutKind.Sequential)] public struct POINT { public int X, Y; }
}
"@
`

func (b *windowsBackend) listWindows(ctx context.Context, query string) ([]domain.WindowInfo, error) {
	script := `
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
	out, err := b.runWin(ctx, script)
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
	script := fmt.Sprintf(`
$h = [IntPtr]%d
[void][DQWin]::ShowWindow($h, 9)
$foreground = [DQWin]::GetForegroundWindow()
$currentThread = [DQWin]::GetCurrentThreadId()
$targetPid = [UInt32]0
$foregroundPid = [UInt32]0
$targetThread = [DQWin]::GetWindowThreadProcessId($h, [ref]$targetPid)
$foregroundThread = [DQWin]::GetWindowThreadProcessId($foreground, [ref]$foregroundPid)
[void][DQWin]::AttachThreadInput($currentThread, $foregroundThread, $true)
[void][DQWin]::BringWindowToTop($h)
[void][DQWin]::SetForegroundWindow($h)
[void][DQWin]::AttachThreadInput($currentThread, $foregroundThread, $false)
`, hwnd)
	_, err = b.runWin(ctx, script)
	return err
}

func (b *windowsBackend) screenshot(ctx context.Context, windowID string, zoom *domain.ScreenRect) (domain.ComputerImage, error) {
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
	if zoom != nil {
		origin = image.Point{X: zoom.X, Y: zoom.Y}
		rect = fmt.Sprintf("%d %d %d %d", zoom.X, zoom.Y, zoom.Width, zoom.Height)
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
	if _, err := b.runWin(ctx, script); err != nil {
		return domain.ComputerImage{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.ComputerImage{}, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return domain.ComputerImage{}, fmt.Errorf("decode screenshot: %w", err)
	}
	return domain.ComputerImage{MimeType: "image/png", Data: data, Width: cfg.Width, Height: cfg.Height, OriginX: origin.X, OriginY: origin.Y, DisplayWidth: cfg.Width, DisplayHeight: cfg.Height}, nil
}

func (b *windowsBackend) mouseMove(ctx context.Context, x, y int, modifiers []domain.KeyModifier) error {
	script := winModifiers(modifiers, true) + fmt.Sprintf("\n[void][DQWin]::SetCursorPos(%d,%d)\n", x, y) + winModifiers(modifiers, false)
	_, err := b.runWin(ctx, script)
	return err
}

func (b *windowsBackend) mouseClick(ctx context.Context, button domain.MouseButton, x, y, clicks int, modifiers []domain.KeyModifier) error {
	if x >= 0 && y >= 0 {
		if err := b.mouseMove(ctx, x, y, nil); err != nil {
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
	script := winModifiers(modifiers, true)
	for i := 0; i < clicks; i++ {
		script += fmt.Sprintf("\n[DQWin]::mouse_event(%s,0,0,0,[IntPtr]::Zero)\n[DQWin]::mouse_event(%s,0,0,0,[IntPtr]::Zero)\nStart-Sleep -Milliseconds 45", down, up)
	}
	script += "\n" + winModifiers(modifiers, false)
	_, err := b.runWin(ctx, script)
	return err
}

func (b *windowsBackend) mouseDrag(ctx context.Context, x0, y0, x1, y1 int) error {
	var script strings.Builder
	fmt.Fprintf(&script, "[void][DQWin]::SetCursorPos(%d,%d)\n[DQWin]::mouse_event(0x0002,0,0,0,[IntPtr]::Zero)\n", x0, y0)
	for i := 1; i <= 12; i++ {
		x := x0 + (x1-x0)*i/12
		y := y0 + (y1-y0)*i/12
		fmt.Fprintf(&script, "[void][DQWin]::SetCursorPos(%d,%d)\nStart-Sleep -Milliseconds 12\n", x, y)
	}
	script.WriteString("[DQWin]::mouse_event(0x0004,0,0,0,[IntPtr]::Zero)")
	_, err := b.runWin(ctx, script.String())
	return err
}

func (b *windowsBackend) mouseButton(ctx context.Context, button domain.MouseButton, pressed bool) error {
	down, up := "0x0002", "0x0004"
	if button == domain.MouseRight {
		down, up = "0x0008", "0x0010"
	} else if button == domain.MouseMiddle {
		down, up = "0x0020", "0x0040"
	}
	action := up
	if pressed {
		action = down
	}
	_, err := b.runWin(ctx, fmt.Sprintf("[DQWin]::mouse_event(%s,0,0,0,[IntPtr]::Zero)", action))
	return err
}

func (b *windowsBackend) scroll(ctx context.Context, x, y int, dir domain.ScrollDirection, amount int) error {
	if x >= 0 && y >= 0 {
		if err := b.mouseMove(ctx, x, y, nil); err != nil {
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
	script := fmt.Sprintf("[DQWin]::mouse_event(%s,0,0,%d,[IntPtr]::Zero)", flag, delta)
	_, err := b.runWin(ctx, script)
	return err
}

func (b *windowsBackend) typeText(ctx context.Context, text string) error {
	// Keep each SendInput buffer small enough for long agent-generated content.
	for _, chunk := range splitUTF16Chunks(text, 256) {
		if err := ctx.Err(); err != nil {
			return err
		}
		script := fmt.Sprintf("$sent = [DQWin]::TypeUnicode(%s)\nif ($sent -eq 0) { throw 'SendInput failed' }", psQuoteSendKeys(chunk))
		if _, err := b.runWin(ctx, script); err != nil {
			return err
		}
	}
	return nil
}

func splitUTF16Chunks(text string, maxUnits int) []string {
	var chunks []string
	var chunk strings.Builder
	units := 0
	for _, r := range text {
		n := 1
		if r > 0xFFFF {
			n = 2
		}
		if units+n > maxUnits && chunk.Len() > 0 {
			chunks = append(chunks, chunk.String())
			chunk.Reset()
			units = 0
		}
		chunk.WriteRune(r)
		units += n
	}
	if chunk.Len() > 0 {
		chunks = append(chunks, chunk.String())
	}
	return chunks
}

func (b *windowsBackend) key(ctx context.Context, key string, hold bool) error {
	if hold {
		seconds, err := keyHoldSeconds(key)
		if err != nil {
			return err
		}
		key = strings.TrimSuffix(key, fmt.Sprintf("\x00%d", seconds))
		keys, err := windowsVirtualKeys(key)
		if err != nil {
			return err
		}
		// One PowerShell process holds the keys across the delay, then releases
		// in reverse order. No duplicate shortcut is emitted.
		var script strings.Builder
		for _, vk := range keys {
			fmt.Fprintf(&script, "[DQWin]::ChordKey(%d,$true)\n", vk)
		}
		fmt.Fprintf(&script, "Start-Sleep -Seconds %d\n", seconds)
		for i := len(keys) - 1; i >= 0; i-- {
			fmt.Fprintf(&script, "[DQWin]::ChordKey(%d,$false)\n", keys[i])
		}
		_, err = b.runWin(ctx, script.String())
		return err
	}
	_, err := b.runWin(ctx, fmt.Sprintf("[DQWin]::Chord(%s)", psQuoteSendKeys(windowsNativeChord(key))))
	return err
}

func windowsNativeChord(key string) string {
	var mods []string
	base := key
	if idx := strings.LastIndex(key, "+"); idx > 0 {
		base = key[idx+1:]
		for _, mod := range strings.Split(key[:idx], "+") {
			switch strings.ToLower(mod) {
			case "ctrl", "control":
				mods = append(mods, "ctrl")
			case "alt", "option":
				mods = append(mods, "alt")
			case "shift":
				mods = append(mods, "shift")
			case "meta", "cmd", "command", "super":
				mods = append(mods, "meta")
			}
		}
	}
	if strings.HasSuffix(key, "++") {
		base = "+"
	}
	return base + "\t" + strings.Join(mods, ",")
}

func windowsVirtualKeys(key string) ([]byte, error) {
	base, mods := key, ""
	if idx := strings.LastIndex(key, "+"); idx > 0 {
		base, mods = key[idx+1:], key[:idx]
	}
	if strings.HasSuffix(key, "++") {
		base = "+"
	}
	var keys []byte
	for _, mod := range strings.Split(mods, "+") {
		switch strings.ToLower(mod) {
		case "shift":
			keys = append(keys, 0x10)
		case "ctrl", "control":
			keys = append(keys, 0x11)
		case "alt", "option":
			keys = append(keys, 0x12)
		case "meta", "cmd", "command", "super":
			keys = append(keys, 0x5B)
		case "":
		default:
			return nil, fmt.Errorf("unsupported modifier %q", mod)
		}
	}
	vk := map[string]byte{"return": 0x0D, "enter": 0x0D, "tab": 0x09, "space": 0x20, "escape": 0x1B, "esc": 0x1B, "delete": 0x2E, "backspace": 0x08, "up": 0x26, "down": 0x28, "left": 0x25, "right": 0x27, "home": 0x24, "end": 0x23, "pageup": 0x21, "pagedown": 0x22}[strings.ToLower(base)]
	if vk == 0 && len(base) == 1 {
		if base[0] >= 'a' && base[0] <= 'z' {
			vk = base[0] - 'a' + 'A'
		} else if base[0] >= '0' && base[0] <= '9' {
			vk = base[0]
		} else if base[0] == '+' {
			vk = 0xBB
		}
	}
	if vk == 0 {
		return nil, fmt.Errorf("unsupported hold key %q", base)
	}
	return append(keys, vk), nil
}

func winModifiers(modifiers []domain.KeyModifier, down bool) string {
	var script strings.Builder
	for _, mod := range modifiers {
		vk := byte(0)
		switch mod {
		case domain.ModifierShift:
			vk = 0x10
		case domain.ModifierCtrl:
			vk = 0x11
		case domain.ModifierAlt:
			vk = 0x12
		case domain.ModifierMeta:
			vk = 0x5B
		}
		action := "KeyUp"
		if down {
			action = "KeyDown"
		}
		fmt.Fprintf(&script, "[DQWin]::%s(%d)\n", action, vk)
	}
	return script.String()
}

// psQuoteSendKeys single-quotes a string for PowerShell (doubling quotes).
func psQuoteSendKeys(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// sendKeysChord maps "ctrl+s" / "Return" to SendKeys syntax (^s, {ENTER}).
func sendKeysChord(key string) string {
	var parts []string
	if strings.HasSuffix(key, "++") {
		parts = strings.Split(strings.TrimSuffix(key, "++"), "+")
		parts = append(parts, "+")
	} else {
		parts = strings.Split(key, "+")
	}
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
		"pageup": "{PGUP}", "pagedown": "{PGDN}",
		"f1": "{F1}", "f2": "{F2}", "f3": "{F3}", "f4": "{F4}", "f5": "{F5}", "f6": "{F6}",
		"f7": "{F7}", "f8": "{F8}", "f9": "{F9}", "f10": "{F10}", "f11": "{F11}", "f12": "{F12}",
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
	out, err := b.runWin(ctx, script)
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
