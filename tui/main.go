package main

import (
	"fmt"
	"os"

	"danmo-work/core/bootstrap"
	"danmo-work/core/runtime/sandbox"
)

func main() {
	if sandbox.MaybeReexec() {
		return
	}
	core := bootstrap.New(bootstrap.Config{ConfigPath: os.Getenv("WORK_CONFIG")})
	defer core.Close()
	_ = core
	fmt.Println("Danmo Work TUI (placeholder)")
}
