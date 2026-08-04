package v1

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/creack/pty"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var terminalUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type terminalClientMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols uint16 `json:"cols,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
}

func defaultShell() string {
	if runtime.GOOS == "windows" {
		return "cmd.exe"
	}
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh
	}
	return "/bin/bash"
}

func projectTerminal(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, err := h.Projects.Get(c, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		dir := p.Directory
		if dir == "" {
			dir = filepath.Join(h.Projects.ProjectDir(p.ID), "files")
		}
		_ = os.MkdirAll(dir, 0755)

		cols := uint16(120)
		rows := uint16(32)
		if v := c.Query("cols"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n < 1000 {
				cols = uint16(n)
			}
		}
		if v := c.Query("rows"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n < 1000 {
				rows = uint16(n)
			}
		}

		conn, err := terminalUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		shell := defaultShell()
		var cmd *exec.Cmd
		// zsh PROMPT_SP prints a bare "%" when it thinks the prior line had no
		// trailing newline — common when a PTY starts at 0×0 / wrong size.
		if runtime.GOOS != "windows" && filepath.Base(shell) == "zsh" {
			cmd = exec.Command(shell, "-o", "NO_PROMPT_SP", "-i")
		} else {
			cmd = exec.Command(shell)
		}
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "TERM=xterm-256color")

		ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: cols, Rows: rows})
		if err != nil {
			_ = conn.WriteMessage(websocket.TextMessage, []byte("\x1b[31m启动终端失败: "+err.Error()+"\x1b[0m\r\n"))
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "pty start failed"))
			return
		}
		defer func() {
			_ = ptmx.Close()
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
				_, _ = cmd.Process.Wait()
			}
		}()

		go func() {
			buf := make([]byte, 8192)
			for {
				n, rerr := ptmx.Read(buf)
				if n > 0 {
					_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
					if werr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
						return
					}
				}
				if rerr != nil {
					_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
					_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "terminal exited"))
					_ = conn.Close()
					return
				}
			}
		}()

		for {
			msgType, data, rerr := conn.ReadMessage()
			if rerr != nil {
				return
			}
			switch msgType {
			case websocket.TextMessage:
				var msg terminalClientMessage
				if err := json.Unmarshal(data, &msg); err != nil {
					continue
				}
				switch msg.Type {
				case "input":
					_, _ = ptmx.Write([]byte(msg.Data))
				case "resize":
					if msg.Cols > 0 && msg.Rows > 0 {
						_ = pty.Setsize(ptmx, &pty.Winsize{Cols: msg.Cols, Rows: msg.Rows})
					}
				}
			case websocket.BinaryMessage:
				_, _ = ptmx.Write(data)
			}
		}
	}
}
