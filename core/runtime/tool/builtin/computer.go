package builtin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
)

// maxComputerImageBytes caps a screenshot before it enters the context window.
const maxComputerImageBytes = 20 * 1024 * 1024

// Computer is a single Anthropic-style desktop-control tool. It dispatches on an
// action field to drive window discovery, mouse, keyboard, and screenshots
// through a port.Computer backend. The backend owns screenshot-scope mapping.
type Computer struct {
	Ctl port.Computer
	// SupportsImage reports whether the routed model accepts image input.
	SupportsImage func(modelID string) bool
}

func (h *Computer) Name() string                { return "computer" }
func (h *Computer) RiskLevel() domain.RiskLevel { return domain.RiskHigh }

func (h *Computer) Describe(args map[string]any) string {
	action, _ := args["action"].(string)
	if action == "" {
		return "computer"
	}
	if q, ok := args["query"].(string); ok && q != "" {
		return "computer " + action + " " + q
	}
	if wid, ok := args["window_id"].(string); ok && wid != "" {
		return "computer " + action + " window=" + wid
	}
	if coord := parseCoordinate(args["coordinate"]); coord != nil {
		return fmt.Sprintf("computer %s (%d,%d)", action, coord[0], coord[1])
	}
	return "computer " + action
}

func (h *Computer) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "computer",
		Description: "Control a desktop GUI to operate applications a human would use.\n\n" +
			"Drive real windows: find and focus a window, take a screenshot to SEE the screen, then move/click " +
			"the mouse, type text, or press keys.\n\n" +
			"Workflow: 1) `list_windows` to find the target, 2) `focus_window` to bring it forward, 3) `screenshot` " +
			"to see it, 4) act with `left_click` / `type` / `key`, 5) `screenshot` again to verify.\n\n" +
			"Coordinates are pixels in the most recent screenshot. Full-display captures use absolute screen pixels; " +
			"window captures use image pixels relative to that window. Always pass the same `window_id` on follow-up " +
			"actions. The runtime handles display density, downscaling, and origins. Never click without a fresh screenshot.\n\n" +
			"When the selected target is a sandbox desktop, actions target its Xvfb display rather than the host. " +
			"Each call requires user approval. Prefer keyboard shortcuts over fragile clicks; prefer browser_* for web UI.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type": "string",
					"enum": []string{
						"list_windows", "focus_window", "screenshot", "zoom", "get_screen_size",
						"mouse_move", "left_click", "right_click", "middle_click", "double_click", "triple_click",
						"left_mouse_down", "left_mouse_up", "left_click_drag", "scroll",
						"type", "key", "hold_key", "cursor_position", "wait",
					},
					"description": "The desktop action to perform.",
				},
				"query": map[string]any{
					"type": "string", "description": "For list_windows: case-insensitive window title/app filter.",
				},
				"window_id": map[string]any{
					"type": "string", "description": "Target window id; scopes screenshot and follow-up pointer coordinates.",
				},
				"coordinate": map[string]any{
					"type": "array", "items": map[string]any{"type": "integer"},
					"description": "[x, y] image pixel. Required for mouse_move/drag end; optional for clicks/scroll.",
				},
				"start_coordinate": map[string]any{
					"type": "array", "items": map[string]any{"type": "integer"},
					"description": "[x, y] drag start image pixel. Required for left_click_drag.",
				},
				"text": map[string]any{
					"type": "string", "description": "For type: literal text. For key/hold_key: key or chord such as Return or ctrl+s.",
				},
				"duration": map[string]any{
					"type": "integer", "description": "For wait/hold_key: seconds (1-10).",
				},
				"scroll_direction": map[string]any{
					"type": "string", "enum": []string{"up", "down", "left", "right"},
				},
				"scroll_amount": map[string]any{"type": "integer", "description": "Scroll ticks (default 3)."},
				"modifiers": map[string]any{
					"type": "array", "items": map[string]any{"type": "string", "enum": []string{"shift", "ctrl", "alt", "meta"}},
					"description": "Modifiers to hold during a mouse action.",
				},
				"zoom": map[string]any{
					"type": "object", "description": "For zoom: screen-coordinate rectangle {x,y,width,height} to capture at native resolution.",
					"properties": map[string]any{
						"x": map[string]any{"type": "integer"}, "y": map[string]any{"type": "integer"},
						"width": map[string]any{"type": "integer"}, "height": map[string]any{"type": "integer"},
					},
					"required": []string{"x", "y", "width", "height"},
				},
			},
			"required": []string{"action"},
		},
	}
}

// computerSessionSetter is optionally implemented by the computer manager when
// sandbox desktop exec needs project/workdir.
type computerSessionSetter interface {
	SetSession(opts port.SandboxRunOptions, sessionID string)
}

func (h *Computer) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	if h.Ctl == nil {
		return domain.ToolResult{}, fmt.Errorf("desktop control is not configured")
	}
	sessionID, _ := input["__session_id"].(string)
	if setter, ok := h.Ctl.(computerSessionSetter); ok {
		projectID, _ := input["__project_id"].(string)
		setter.SetSession(port.SandboxRunOptions{ProjectID: projectID, WorkDir: workDirFromInput(input), Timeout: 60 * time.Second}, sessionID)
	}
	action, _ := input["action"].(string)
	action = strings.TrimSpace(action)
	if action == "" {
		return domain.ToolResult{}, fmt.Errorf("action is required")
	}
	windowID, _ := input["window_id"].(string)

	switch action {
	case "list_windows":
		return h.listWindows(ctx, input)
	case "focus_window":
		return h.focusWindow(ctx, sessionID, windowID)
	case "screenshot":
		return h.screenshot(ctx, input, sessionID, windowID, false)
	case "zoom":
		return h.screenshot(ctx, input, sessionID, windowID, true)
	case "get_screen_size":
		return h.screenSize(ctx, sessionID, windowID)
	case "mouse_move":
		return h.mouseMove(ctx, input, sessionID, windowID)
	case "left_click", "right_click", "middle_click", "double_click", "triple_click":
		return h.click(ctx, action, input, sessionID, windowID)
	case "left_mouse_down", "left_mouse_up":
		return h.mouseButton(ctx, action)
	case "left_click_drag":
		return h.drag(ctx, input, sessionID, windowID)
	case "scroll":
		return h.scroll(ctx, input, sessionID, windowID)
	case "type":
		return h.typeText(ctx, input)
	case "key", "hold_key":
		return h.keyAction(ctx, input, action == "hold_key")
	case "cursor_position":
		return h.cursorPosition(ctx)
	case "wait":
		return h.wait(ctx, input)
	default:
		return domain.ToolResult{}, fmt.Errorf("unknown action %q", action)
	}
}

func (h *Computer) listWindows(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	query, _ := input["query"].(string)
	sessionID, _ := input["__session_id"].(string)
	var wins []domain.WindowInfo
	var err error
	if ctl, ok := h.Ctl.(interface {
		ListWindowsForSession(context.Context, string, string) ([]domain.WindowInfo, error)
	}); ok {
		wins, err = ctl.ListWindowsForSession(ctx, sessionID, query)
	} else {
		wins, err = h.Ctl.ListWindows(ctx, query)
	}
	if err != nil {
		return domain.ToolResult{}, err
	}
	if len(wins) == 0 {
		return domain.ToolResult{Content: "No matching windows found."}, nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Found %d window(s):\n", len(wins))
	for _, w := range wins {
		active := ""
		if w.Active {
			active = " [active]"
		}
		fmt.Fprintf(&b, "- id=%s%s title=%q app=%q bounds=%d,%d %dx%d\n", w.ID, active, w.Title, w.App, w.X, w.Y, w.Width, w.Height)
	}
	winsJSON, _ := json.Marshal(wins)
	return domain.ToolResult{Content: b.String(), Meta: map[string]any{"op": "list_windows", "count": len(wins), "windows": json.RawMessage(winsJSON)}}, nil
}

func (h *Computer) focusWindow(ctx context.Context, sessionID, windowID string) (domain.ToolResult, error) {
	if windowID == "" {
		return domain.ToolResult{}, fmt.Errorf("window_id is required for focus_window")
	}
	var err error
	if ctl, ok := h.Ctl.(interface {
		FocusWindowForSession(context.Context, string, string) error
	}); ok {
		err = ctl.FocusWindowForSession(ctx, sessionID, windowID)
	} else {
		err = h.Ctl.FocusWindow(ctx, windowID)
	}
	if err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{Content: "Focused window " + windowID + "."}, nil
}

func (h *Computer) checkVision(input map[string]any) error {
	modelID, _ := input["__model_id"].(string)
	if h.SupportsImage != nil && modelID != "" && !h.SupportsImage(modelID) {
		return fmt.Errorf("the current model (%s) does not accept image input; desktop screenshots are unavailable", modelID)
	}
	return nil
}

func (h *Computer) screenshot(ctx context.Context, input map[string]any, sessionID, windowID string, zoom bool) (domain.ToolResult, error) {
	if err := h.checkVision(input); err != nil {
		return domain.ToolResult{}, err
	}
	var rect *domain.ScreenRect
	if zoom {
		r, err := parseScreenRect(input["zoom"])
		if err != nil {
			return domain.ToolResult{}, err
		}
		rect = &r
	}
	img, err := h.Ctl.Screenshot(ctx, sessionID, windowID, rect)
	if err != nil {
		return domain.ToolResult{}, err
	}
	if len(img.Data) > maxComputerImageBytes {
		return domain.ToolResult{}, fmt.Errorf("screenshot is %d bytes, larger than the %d MB cap", len(img.Data), maxComputerImageBytes/(1024*1024))
	}
	if img.Unchanged {
		return domain.ToolResult{Content: "Screenshot is unchanged from the previous capture of this scope in this turn; reuse the existing image and coordinates.", Meta: map[string]any{"op": "screenshot", "unchanged": true}}, nil
	}
	space := "absolute"
	if windowID != "" {
		space = "window-relative"
	}
	meta := map[string]any{
		"op": "screenshot", "width": img.Width, "height": img.Height,
		"coordinate_space": space, "origin_x": img.OriginX, "origin_y": img.OriginY,
		"display_width": img.DisplayWidth, "display_height": img.DisplayHeight,
	}
	if windowID != "" {
		meta["window_id"] = windowID
	}
	desc := fmt.Sprintf("Captured screenshot (%dx%d image, %dx%d input-coordinate region, %s coordinates).", img.Width, img.Height, img.DisplayWidth, img.DisplayHeight, space)
	if zoom {
		meta["op"] = "zoom"
		meta["zoom"] = rect
		desc = fmt.Sprintf("Captured zoomed screenshot (%dx%d image). Use its image pixels with a follow-up action scoped to this zoom.", img.Width, img.Height)
	}
	return domain.ToolResult{
		Content: desc, Meta: meta,
		Parts: []domain.ToolResultPart{{Type: "image", MimeType: img.MimeType, Data: base64.StdEncoding.EncodeToString(img.Data)}},
	}, nil
}

func (h *Computer) screenSize(ctx context.Context, sessionID, windowID string) (domain.ToolResult, error) {
	img, err := h.Ctl.Screenshot(ctx, sessionID, windowID, nil)
	if err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{
		Content: fmt.Sprintf("Screen/capture region is %dx%d input-coordinate units at origin (%d,%d).", img.DisplayWidth, img.DisplayHeight, img.OriginX, img.OriginY),
		Meta:    map[string]any{"op": "get_screen_size", "width": img.DisplayWidth, "height": img.DisplayHeight, "origin_x": img.OriginX, "origin_y": img.OriginY},
	}, nil
}

func (h *Computer) mouseMove(ctx context.Context, input map[string]any, sessionID, windowID string) (domain.ToolResult, error) {
	coord := parseCoordinate(input["coordinate"])
	if coord == nil {
		return domain.ToolResult{}, fmt.Errorf("coordinate [x, y] is required for mouse_move")
	}
	mods, err := parseModifiers(input["modifiers"])
	if err != nil {
		return domain.ToolResult{}, err
	}
	if err := h.Ctl.MouseMove(ctx, sessionID, windowID, coord[0], coord[1], mods); err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{Content: fmt.Sprintf("Moved cursor to image coordinate (%d, %d).", coord[0], coord[1])}, nil
}

func (h *Computer) click(ctx context.Context, action string, input map[string]any, sessionID, windowID string) (domain.ToolResult, error) {
	button := domain.MouseLeft
	clicks := 1
	switch action {
	case "right_click":
		button = domain.MouseRight
	case "middle_click":
		button = domain.MouseMiddle
	case "double_click":
		clicks = 2
	case "triple_click":
		clicks = 3
	}
	x, y := -1, -1
	if coord := parseCoordinate(input["coordinate"]); coord != nil {
		x, y = coord[0], coord[1]
	}
	mods, err := parseModifiers(input["modifiers"])
	if err != nil {
		return domain.ToolResult{}, err
	}
	if err := h.Ctl.MouseClick(ctx, sessionID, windowID, button, x, y, clicks, mods); err != nil {
		return domain.ToolResult{}, err
	}
	where := "at current position"
	if x >= 0 {
		where = fmt.Sprintf("at image coordinate (%d, %d)", x, y)
	}
	return domain.ToolResult{Content: fmt.Sprintf("Performed %s %s.", action, where)}, nil
}

func (h *Computer) mouseButton(ctx context.Context, action string) (domain.ToolResult, error) {
	pressed := action == "left_mouse_down"
	if err := h.Ctl.MouseButton(ctx, domain.MouseLeft, pressed); err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{Content: "Mouse button state changed."}, nil
}

func (h *Computer) drag(ctx context.Context, input map[string]any, sessionID, windowID string) (domain.ToolResult, error) {
	start := parseCoordinate(input["start_coordinate"])
	end := parseCoordinate(input["coordinate"])
	if start == nil || end == nil {
		return domain.ToolResult{}, fmt.Errorf("left_click_drag requires start_coordinate and coordinate")
	}
	if err := h.Ctl.MouseDrag(ctx, sessionID, windowID, start[0], start[1], end[0], end[1]); err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{Content: "Performed left-button drag."}, nil
}

func (h *Computer) scroll(ctx context.Context, input map[string]any, sessionID, windowID string) (domain.ToolResult, error) {
	dirStr, _ := input["scroll_direction"].(string)
	dir := domain.ScrollDirection(strings.ToLower(strings.TrimSpace(dirStr)))
	switch dir {
	case domain.ScrollUp, domain.ScrollDown, domain.ScrollLeft, domain.ScrollRight:
	default:
		return domain.ToolResult{}, fmt.Errorf("scroll_direction must be up, down, left, or right")
	}
	amount := intFromArg(input["scroll_amount"], 3)
	x, y := -1, -1
	if coord := parseCoordinate(input["coordinate"]); coord != nil {
		x, y = coord[0], coord[1]
	}
	if err := h.Ctl.Scroll(ctx, sessionID, windowID, x, y, dir, amount); err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{Content: fmt.Sprintf("Scrolled %s by %d.", dir, amount)}, nil
}

func (h *Computer) typeText(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	text, _ := input["text"].(string)
	if text == "" {
		return domain.ToolResult{}, fmt.Errorf("text is required for type")
	}
	if err := h.Ctl.TypeText(ctx, text); err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{Content: fmt.Sprintf("Typed %d character(s).", len([]rune(text)))}, nil
}

func (h *Computer) keyAction(ctx context.Context, input map[string]any, hold bool) (domain.ToolResult, error) {
	key, _ := input["text"].(string)
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.ToolResult{}, fmt.Errorf("text (key or chord) is required for key")
	}
	if hold {
		seconds := intFromArg(input["duration"], 1)
		if seconds < 1 {
			seconds = 1
		}
		if seconds > 10 {
			seconds = 10
		}
		key = fmt.Sprintf("%s\x00%d", key, seconds)
	}
	if err := h.Ctl.Key(ctx, key, hold); err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{Content: "Pressed " + strings.SplitN(key, "\x00", 2)[0] + "."}, nil
}

func (h *Computer) cursorPosition(ctx context.Context) (domain.ToolResult, error) {
	x, y, err := h.Ctl.CursorPosition(ctx)
	if err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{Content: fmt.Sprintf("Cursor is at (%d, %d).", x, y), Meta: map[string]any{"op": "cursor_position", "x": x, "y": y}}, nil
}

func (h *Computer) wait(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	seconds := intFromArg(input["duration"], 1)
	if seconds < 1 {
		seconds = 1
	}
	if seconds > 10 {
		seconds = 10
	}
	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		return domain.ToolResult{Content: fmt.Sprintf("Waited %d second(s).", seconds)}, nil
	case <-ctx.Done():
		return domain.ToolResult{}, ctx.Err()
	}
}

// parseCoordinate coerces a JSON [x, y] array (any numeric form) to [2]int.
func parseCoordinate(v any) []int {
	arr, ok := v.([]any)
	if !ok || len(arr) < 2 {
		return nil
	}
	return []int{intFromArg(arr[0], 0), intFromArg(arr[1], 0)}
}

func parseModifiers(v any) ([]domain.KeyModifier, error) {
	if v == nil {
		return nil, nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("modifiers must be an array")
	}
	mods := make([]domain.KeyModifier, 0, len(items))
	for _, item := range items {
		s, _ := item.(string)
		mod := domain.KeyModifier(strings.ToLower(strings.TrimSpace(s)))
		switch mod {
		case domain.ModifierShift, domain.ModifierCtrl, domain.ModifierAlt, domain.ModifierMeta:
			mods = append(mods, mod)
		default:
			return nil, fmt.Errorf("unsupported modifier %q", s)
		}
	}
	return mods, nil
}

func parseScreenRect(v any) (domain.ScreenRect, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return domain.ScreenRect{}, fmt.Errorf("zoom requires an object with x, y, width, and height")
	}
	r := domain.ScreenRect{
		X: intFromArg(m["x"], 0), Y: intFromArg(m["y"], 0),
		Width: intFromArg(m["width"], 0), Height: intFromArg(m["height"], 0),
	}
	if r.Width <= 0 || r.Height <= 0 {
		return domain.ScreenRect{}, fmt.Errorf("zoom width and height must be positive")
	}
	return r, nil
}
