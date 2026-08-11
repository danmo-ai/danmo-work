package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"danmo-work/core/bootstrap"
	"danmo-work/core/domain"
	"danmo-work/core/runtime/sandbox"
)

func main() {
	if sandbox.MaybeReexec() {
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "run" {
		os.Exit(runHeadless(os.Args[2:]))
	}

	core := bootstrap.New(bootstrap.Config{ConfigPath: os.Getenv("WORK_CONFIG")})
	defer core.Close()
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Danmo Work CLI. Type 'exit' to quit. Use 'danmo-work-cli run --help' for headless eval.")
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "exit" {
			break
		}
		if input == "" {
			continue
		}
		session, err := core.Sessions.Create(context.Background(), domain.CreateSessionRequest{
			Content: input,
			AgentID: "team",
		})
		if err != nil {
			fmt.Println("error:", err)
			continue
		}
		fmt.Println("session created:", session.ID)
		var since int64
		for {
			events := core.Sessions.StreamEvents(session.ID, since)
			done := false
			for _, ev := range events {
				since = ev.Seq
				fmt.Printf("[%s] %s\n", ev.Type, string(ev.Payload))
				if ev.Type == domain.EventReport || ev.Type == domain.EventSessionCompleted || ev.Type == domain.EventTurnFailed || ev.Type == domain.EventError {
					done = true
				}
			}
			if done {
				break
			}
			// Avoid busy-spin while waiting for the next stream batch.
			select {
			case <-time.After(200 * time.Millisecond):
			}
		}
	}
}
