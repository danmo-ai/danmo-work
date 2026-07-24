package main

import (
	"fmt"
	"os"

	"danqing-teams/core/bootstrap"
	"danqing-teams/core/runtime/sandbox"
)

func main() {
	if sandbox.MaybeReexec() {
		return
	}
	core := bootstrap.New(bootstrap.Config{ConfigPath: os.Getenv("TEAMS_CONFIG")})
	defer core.Close()
	_ = core
	fmt.Println("Danmo Work TUI (placeholder)")
}
