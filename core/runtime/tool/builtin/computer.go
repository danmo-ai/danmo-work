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
// `action` field to drive window discovery, mouse, keyboard, and screenshots
// through a port.Computer backend. It is a high-risk, host-only capability
// gated by the desktop_control permission reason.
type Computer struct {
	Ctl port.Computer
	// SupportsImage reports whether the routed model accepts image input.
	// When false, the screenshot action returns text-only guidance.
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
		Description: "Control the local desktop GUI to operate applications a human would use.\n\n" +
			"Drive real windows: find and focus a window, take a screenshot to SEE the screen, then move/click " +
			"the mouse, type text, or press keys.\n\n" +
			"Workflow: 1) `list_windows` to find the target, 2) `focus_window` to bring it forward, 3) `screenshot` " +
			"to see it, 4) act with `left_click` / `type` / `key`, 5) `screenshot` again to verify.\n\n" +
			"Coordinates: a full-display screenshot uses absolute screen pixels. A window screenshot returns pixels " +
			"relative to that image; pass the same `window_id` on the follow-up action and coordinates are mapped " +
			"back automatically.\n\n" +
			"This controls the actual machine with no sandbox. Each call requires user approval. Prefer keyboard " +
			"shortcuts (`key`) over fragile mouse clicks when possible.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type": "string",
					"enum": []string{
						"list_windows", "focus_window", "screenshot",
						"mouse_move", "left_click", "right_click", "middle_click",
						"double_click", "left_click_drag", "scroll",
						"type", "key", "cursor_position", "wait",
					},
					"description": "The desktop action to perform.",
				},
				"query": map[string]any{
					"type":        "string",
					"description": "For list_windows: case-insensitive substring to filter by window title or app.",
				},
				"window_id": map[string]any{
					"type":        "string",
					"description": "Target window id (from list_windows). For focus_window; and for screenshot / click / scroll to scope coordinates to that window.",
				},
				"coordinate": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "integer"},
					"description": "[x, y] target pixel. Required for mouse_move; optional for clicks/scroll (defaults to current cursor).",
				},
				"start_coordinate": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "integer"},
					"description": "[x, y] drag start. Required for left_click_drag.",
				},
				"text": map[string]any{
					"type":        "string",
					"description": "For type: literal text to type. For key: a key or chord like 'Return', 'ctrl+s', 'alt+Tab'.",
				},
				"scroll_direction": map[string]any{
					"type":        "string",
					"enum":        []string{"up", "down", "left", "right"},
					"description": "For scroll: direction to scroll.",
				},
				"scroll_amount": map[string]any{
					"type":        "integer",
					"description": "For scroll: number of scroll ticks (default 3).",
				},
				"duration": map[string]any{
					"type":        "integer",
					"description": "For wait: seconds to pause (1-10).",
				},
			},
			"required": []string{"action"},
		},
	}
}

func (h *Computer) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	if h.Ctl == nil {
		return domain.ToolResult{}, fmt.Errorf("desktop control is not configured")
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
		return h.focusWindow(ctx, windowID)
	case "screenshot":
		return h.screenshot(ctx, input, windowID)
	case "mouse_move":
		return h.mouseMove(ctx, input, windowID)
	case "left_click", "right_click", "middle_click", "double_click":
		return h.click(ctx, action, input, windowID)
	case "left_click_drag":
		return h.drag(ctx, input, windowID)
	case "scroll":
		return h.scroll(ctx, input, windowID)
	case "type":
		return h.typeText(ctx, input)
	case "key":
		return h.key(ctx, input)
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
	wins, err := h.Ctl.ListWindows(ctx, query)
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
		fmt.Fprintf(&b, "- id=%s%s title=%q app=%q bounds=%d,%d %dx%d\n",
			w.ID, active, w.Title, w.App, w.X, w.Y, w.Width, w.Height)
	}
	winsJSON, _ := json.Marshal(wins)
	return domain.ToolResult{
		Content: b.String(),
		Meta:    map[string]any{"op": "list_windows", "count": len(wins), "windows": json.RawMessage(winsJSON)},
	}, nil
}

func (h *Computer) focusWindow(ctx context.Context, windowID string) (domain.ToolResult, error) {
	if windowID == "" {
		return domain.ToolResult{}, fmt.Errorf("window_id is required for focus_window")
	}
	if err := h.Ctl.FocusWindow(ctx, windowID); err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{Content: "Focused window " + windowID + "."}, nil
}

func (h *Computer) screenshot(ctx context.Context, input map[string]any, windowID string) (domain.ToolResult, error) {
	img, err := h.Ctl.Screenshot(ctx, windowID)
	if err != nil {
		return domain.ToolResult{}, err
	}
	if len(img.Data) > maxComputerImageBytes {
		return domain.ToolResult{}, fmt.Errorf("screenshot is %d bytes, larger than the %d MB cap", len(img.Data), maxComputerImageBytes/(1024*1024))
	}
	space := "absolute"
	if windowID != "" {
		space = "window-relative"
	}
	meta := map[string]any{
		"op":               "screenshot",
		"width":            img.Width,
		"height":           img.Height,
		"coordinate_space": space,
		"origin_x":         img.OriginX,
		"origin_y":         img.OriginY,
	}
	if windowID != "" {
		meta["window_id"] = windowID
	}
	desc := fmt.Sprintf("Captured screenshot (%dx%d, %s coordinates).", img.Width, img.Height, space)

	modelID, _ := input["__model_id"].(string)
	if h.SupportsImage != nil && modelID != "" && !h.SupportsImage(modelID) {
		return domain.ToolResult{
			Content: desc + " The current model cannot view images, so the screenshot bytes were omitted.",
			Meta:    meta,
		}, nil
	}
	return domain.ToolResult{
		Content: desc,
		Meta:    meta,
		Parts: []domain.ToolResultPart{{
			Type:     "image",
			MimeType: img.MimeType,
			Data:     base64.StdEncoding.EncodeToString(img.Data),
		}},
	}, nil
}

// resolvePoint maps a possibly window-relative coordinate to absolute screen
// coordinates using the target window's bounds.
func (h *Computer) resolvePoint(ctx context.Context, windowID string, x, y int) (int, int, error) {
	if windowID == "" {
		return x, y, nil
	}
	b, err := h.Ctl.WindowBounds(ctx, windowID)
	if err != nil {
		return 0, 0, err
	}
	return b.X + x, b.Y + y, nil
}

func (h *Computer) mouseMove(ctx context.Context, input map[string]any, windowID string) (domain.ToolResult, error) {
	coord := parseCoordinate(input["coordinate"])
	if coord == nil {
		return domain.ToolResult{}, fmt.Errorf("coordinate [x, y] is required for mouse_move")
	}
	x, y, err := h.resolvePoint(ctx, windowID, coord[0], coord[1])
	if err != nil {
		return domain.ToolResult{}, err
	}
	if err := h.Ctl.MouseMove(ctx, x, y); err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{Content: fmt.Sprintf("Moved cursor to (%d, %d).", x, y)}, nil
}

func (h *Computer) click(ctx context.Context, action string, input map[string]any, windowID string) (domain.ToolResult, error) {
	button := domain.MouseLeft
	clicks := 1
	switch action {
	case "right_click":
		button = domain.MouseRight
	case "middle_click":
		button = domain.MouseMiddle
	case "double_click":
		clicks = 2
	}
	x, y := -1, -1
	if coord := parseCoordinate(input["coordinate"]); coord != nil {
		var err error
		x, y, err = h.resolvePoint(ctx, windowID, coord[0], coord[1])
		if err != nil {
			return domain.ToolResult{}, err
		}
	}
	if err := h.Ctl.MouseClick(ctx, button, x, y, clicks); err != nil {
		return domain.ToolResult{}, err
	}
	where := "at current position"
	if x >= 0 {
		where = fmt.Sprintf("at (%d, %d)", x, y)
	}
	return domain.ToolResult{Content: fmt.Sprintf("Performed %s %s.", action, where)}, nil
}

func (h *Computer) drag(ctx context.Context, input map[string]any, windowID string) (domain.ToolResult, error) {
	start := parseCoordinate(input["start_coordinate"])
	end := parseCoordinate(input["coordinate"])
	if start == nil || end == nil {
		return domain.ToolResult{}, fmt.Errorf("left_click_drag requires start_coordinate and coordinate")
	}
	x0, y0, err := h.resolvePoint(ctx, windowID, start[0], start[1])
	if err != nil {
		return domain.ToolResult{}, err
	}
	x1, y1, err := h.resolvePoint(ctx, windowID, end[0], end[1])
	if err != nil {
		return domain.ToolResult{}, err
	}
	if err := h.Ctl.MouseDrag(ctx, x0, y0, x1, y1); err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{Content: fmt.Sprintf("Dragged from (%d, %d) to (%d, %d).", x0, y0, x1, y1)}, nil
}

func (h *Computer) scroll(ctx context.Context, input map[string]any, windowID string) (domain.ToolResult, error) {
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
		var err error
		x, y, err = h.resolvePoint(ctx, windowID, coord[0], coord[1])
		if err != nil {
			return domain.ToolResult{}, err
		}
	}
	if err := h.Ctl.Scroll(ctx, x, y, dir, amount); err != nil {
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

func (h *Computer) key(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	key, _ := input["text"].(string)
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.ToolResult{}, fmt.Errorf("text (key or chord) is required for key")
	}
	if err := h.Ctl.Key(ctx, key); err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{Content: "Pressed " + key + "."}, nil
}

func (h *Computer) cursorPosition(ctx context.Context) (domain.ToolResult, error) {
	x, y, err := h.Ctl.CursorPosition(ctx)
	if err != nil {
		return domain.ToolResult{}, err
	}
	return domain.ToolResult{
		Content: fmt.Sprintf("Cursor is at (%d, %d).", x, y),
		Meta:    map[string]any{"op": "cursor_position", "x": x, "y": y},
	}, nil
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
