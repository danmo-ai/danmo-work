package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"danmo-work/core/bootstrap"
	"danmo-work/core/remote/connector"
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

	remoteCfg := connector.Config{
		Enabled:     core.Config.Remote.Enabled,
		HubURL:      core.Config.Remote.HubURL,
		LocalBase:   core.Config.Remote.LocalBase,
		TLSInsecure: core.Config.Remote.TLSInsecure,
		AppVersion:  apiv1.Version,
	}.WithEnv()
	// Prefer loopback for the tunnel target even when the API listens on 0.0.0.0.
	if remoteCfg.LocalBase == "" {
		remoteCfg.LocalBase = localBaseFromListen(core.Config.Server.ListenAddr)
	}
	remote, err := connector.New(remoteCfg)
	if err != nil {
		log.Printf("[remote] init failed: %v", err)
	}

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
		Knowledge:   core.Knowledge,
		TurnLogs:    core.TurnLogs,
		MCPServers:  core.MCPServers,
		Automations: core.Automations,
		Weixin:      core.Weixin,
		Feishu:      core.Feishu,
		Wecom:       core.Wecom,
		QQ:          core.QQ,
		Channels:    core.Channels,
		Sandbox:     core.Sandbox,
		Execution:   core.Execution,
		Browser:     core.Browser,
		Store:       core.Store,
		TableStore:  core.TableStore,
		AIReview:    core.AIReview,
		Remote:      remote,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- apiv1.NewRouter(h, apiv1.RouterConfig{}).Run(core.Config.Server.ListenAddr)
	}()

	// Start connector after a short delay so local listen is up for health probes.
	if remote != nil {
		go func() {
			select {
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
				remote.Start(ctx)
			}
		}()
		defer remote.Stop()
	}

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			os.Exit(1)
		}
	}
}

func localBaseFromListen(addr string) string {
	port := "7801"
	if i := strings.LastIndex(addr, ":"); i >= 0 && i+1 < len(addr) {
		port = addr[i+1:]
	}
	return "http://127.0.0.1:" + port
}
