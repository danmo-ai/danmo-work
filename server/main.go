package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"danmo-work/core/bootstrap"
	"danmo-work/core/runtime/sandbox"
	"danmo-work/core/service"
	apiv1 "danmo-work/server/api/v1"
)

func main() {
	if sandbox.MaybeReexec() {
		return
	}
	core := bootstrap.New(bootstrap.Config{ConfigPath: os.Getenv("WORK_CONFIG")})
	defer core.Close()

	h := &apiv1.Handler{
		Sessions:     core.Sessions,
		Projects:     core.Projects,
		LLMConfig:    core.LLMConfig,
		Config:       core.ConfigManager,
		SearchConfig: core.SearchConfig,
		Agents:       core.Agents,
		Skills:       core.Skills,
		SkillHandler: &apiv1.SkillHandler{
			Skills:   core.Skills,
			Importer: service.NewSkillImporter(),
		},
		MarketHandler: &apiv1.MarketHandler{
			Market: core.Market,
		},
		TurnLogs:   core.TurnLogs,
		MCPServers: core.MCPServers,
		Weixin:     core.Weixin,
		Feishu:     core.Feishu,
		Wecom:      core.Wecom,
		Channels:   core.Channels,
		Sandbox:    core.Sandbox,
		Browser:    core.Browser,
		Store:      core.Store,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- apiv1.NewRouter(h, apiv1.RouterConfig{}).Run(core.Config.Server.ListenAddr)
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			os.Exit(1)
		}
	}
}
