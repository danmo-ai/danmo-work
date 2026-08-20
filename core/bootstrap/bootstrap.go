package bootstrap

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"danmo-work/core/adapter/config"
	"danmo-work/core/adapter/feishu"
	"danmo-work/core/adapter/llm"
	marketadapter "danmo-work/core/adapter/market"
	adaptermcp "danmo-work/core/adapter/mcp"
	"danmo-work/core/adapter/qq"
	"danmo-work/core/domain"
	"danmo-work/core/paths"
	"danmo-work/core/port"
	"danmo-work/core/resource/home"
	dqruntime "danmo-work/core/runtime"
	dqbrowser "danmo-work/core/runtime/browser"
	"danmo-work/core/runtime/sandbox"
	"danmo-work/core/runtime/tool/builtin"
	"danmo-work/core/service"
	sqlitestore "danmo-work/core/store/sqlite"
	"danmo-work/core/store/tablestore"
	"danmo-work/core/store/turnlog"
)

type Config struct {
	ConfigPath             string
	AutoApprove            bool
	DataDir                string
	LLM                    port.LLMProvider
	CompactionEnabled      bool
	CompactionTurnInterval int
	CompactionSubInterval  int
	CompactionMaxTokens    int
	CompactionCutTokens    int
}

type Core struct {
	Store         port.Repository
	TableStore    port.TableStoreRepo
	Engine        port.Engine
	Sandbox       port.Sandbox
	Execution     port.ExecutionBackend
	Browser       port.Browser
	Config        *domain.ConfigFile
	Loader        *config.Loader
	Sessions      *service.SessionManager
	Projects      *service.ProjectManager
	Git           *service.GitManager
	LLMConfig     *service.LLMConfigManager
	ConfigManager *service.ConfigManager
	SearchConfig  *service.SearchConfigManager
	Agents        *service.AgentManager
	Skills        *service.SkillManager
	Knowledge     *service.KnowledgeManager
	Market        *service.MarketManager
	TurnLogs      *service.TurnLogManager
	MCPServers    *service.MCPManager
	Automations   *service.AutomationManager
	Weixin        *service.WeixinBridge
	Feishu        *service.FeishuBridge
	Wecom         *service.WecomBridge
	QQ            *service.QQBridge
	Channels      *service.ChannelManager
	AIReview      *service.AIReviewManager
	PluginManager *service.PluginManager
}

func New(cfg Config) *Core {
	loader := config.NewLoader(cfg.ConfigPath)
	appCfg, err := loader.Load(context.Background())
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	if cfg.DataDir != "" {
		appCfg.Data.Dir = cfg.DataDir
	}
	if cfg.AutoApprove {
		appCfg.Runtime.AutoApprove = true
	}
	if cfg.CompactionEnabled {
		appCfg.Runtime.Compaction.Enabled = true
	}
	if cfg.CompactionTurnInterval > 0 {
		appCfg.Runtime.Compaction.TurnInterval = cfg.CompactionTurnInterval
	}
	if cfg.CompactionSubInterval > 0 {
		appCfg.Runtime.Compaction.SubInterval = cfg.CompactionSubInterval
	}
	if cfg.CompactionMaxTokens > 0 {
		appCfg.Runtime.Compaction.MaxTokens = cfg.CompactionMaxTokens
	}
	if cfg.CompactionCutTokens > 0 {
		appCfg.Runtime.Compaction.CutTokens = cfg.CompactionCutTokens
	}

	if appCfg.Data.Dir == "" {
		appCfg.Data.Dir = paths.DataDir()
	}
	if appCfg.Data.Database == "" {
		appCfg.Data.Database = paths.DatabaseFile()
	}
	if appCfg.Data.StoreDatabase == "" {
		appCfg.Data.StoreDatabase = paths.StoreDatabaseFile()
	}
	if !filepath.IsAbs(appCfg.Data.Dir) {
		appCfg.Data.Dir = paths.ResolveAgainstHome(appCfg.Data.Dir)
	}
	if !filepath.IsAbs(appCfg.Data.Database) {
		appCfg.Data.Database = paths.ResolveAgainstHome(appCfg.Data.Database)
	}
	if !filepath.IsAbs(appCfg.Data.StoreDatabase) {
		appCfg.Data.StoreDatabase = paths.ResolveAgainstHome(appCfg.Data.StoreDatabase)
	}
	// History plane derives from the control DB location (same directory)
	// unless explicitly configured — one root, everything else derived.
	if appCfg.Data.HistoryDatabase == "" {
		appCfg.Data.HistoryDatabase = paths.HistoryDatabaseFileFor(appCfg.Data.Database)
	}
	if !filepath.IsAbs(appCfg.Data.HistoryDatabase) {
		appCfg.Data.HistoryDatabase = paths.ResolveAgainstHome(appCfg.Data.HistoryDatabase)
	}
	if appCfg.Instance.ID == "" {
		appCfg.Instance.ID = os.Getenv("WORK_INSTANCE_ID")
		if appCfg.Instance.ID == "" {
			appCfg.Instance.ID, _ = os.Hostname()
		}
	}

	log.Printf("[bootstrap] work.db=%s history.db=%s store.db=%s data=%s",
		appCfg.Data.Database, appCfg.Data.HistoryDatabase, appCfg.Data.StoreDatabase, appCfg.Data.Dir)
	st, err := sqlitestore.New(appCfg.Data.Database)
	if err != nil {
		panic("failed to open database: " + err.Error())
	}
	hist, err := sqlitestore.NewHistory(appCfg.Data.HistoryDatabase)
	if err != nil {
		panic("failed to open history database: " + err.Error())
	}
	st.SetHistory(hist)
	// One-time move of bulk tables out of work.db. Must run before anything
	// reads stream events (MaxSeq seeding) or turn entries.
	if err := sqlitestore.MigrateHistoryTables(context.Background(), st, hist); err != nil {
		log.Printf("[bootstrap] history split migration (will retry next start): %v", err)
	}
	tableStore, err := tablestore.New(appCfg.Data.StoreDatabase)
	if err != nil {
		panic("failed to open table store: " + err.Error())
	}

	pm := service.NewProjectManager(st, appCfg.Data.Dir)
	ensureDefaultProject(pm)

	gitMgr := service.NewGitManager(pm)
	gitMgr.SetSecretStore(st.Secrets())

	turnLog := turnlog.NewTurnLogStore(st.TurnLogs())
	migrateLegacyTurnLogs(st, pm)
	migrateLegacySessionArtifacts(st, pm)
	agents := service.NewAgentManager(appCfg.Data.Dir)
	skills := service.NewSkillManager(appCfg.Data.Dir)
	knowledgeMgr := service.NewKnowledgeManager(
		st.KnowledgeBases(),
		st.KnowledgeDocs(),
		st.KnowledgeIndex(),
		paths.KnowledgeDir(),
		func() domain.ConfigKnowledgeSection {
			c, err := loader.Load(context.Background())
			if err != nil {
				return domain.ConfigKnowledgeSection{SearchTopK: 3, ChapterMaxTokens: 512}
			}
			sec := c.Runtime.Knowledge
			if sec.SearchTopK <= 0 {
				sec.SearchTopK = 3
			}
			if sec.ChapterMaxTokens <= 0 {
				sec.ChapterMaxTokens = 512
			}
			return sec
		},
	)
	if err := knowledgeMgr.MigrateLegacyContents(context.Background(), st.LegacyDocContent, st.ClearLegacyDocContent); err != nil {
		log.Printf("[bootstrap] knowledge legacy migrate: %v", err)
	}
	if _, err := knowledgeMgr.EnsureDefaultBase(context.Background()); err != nil {
		log.Printf("[bootstrap] knowledge default base: %v", err)
	}
	knowledge := builtin.NewKnowledge()
	knowledge.SetBackend(knowledgeMgr)
	turnManager := service.NewTurnManager(st.Turns())
	turnLogManager := service.NewTurnLogManager(turnLog)
	approvalManager := service.NewApprovalManager(st.Approvals())
	mcpManager := service.NewMCPManager(appCfg.Data.Dir)
	mcpManager.SetSecretStore(st.Secrets())
	mcpDialer := adaptermcp.NewDialer()
	mcpManager.SetDialer(mcpDialer)

	pluginManager := service.NewPluginManager(appCfg.Data.Dir, skills, agents, mcpManager, knowledgeMgr)
	if err := pluginManager.Init(context.Background()); err != nil {
		log.Printf("[bootstrap] plugin init: %v", err)
	}
	knowledgeMgr.StartAsyncIndex(context.Background())

	llmConfigRepo := st.LLMConfig()
	searchConfig := service.NewSearchConfigManager(loader)
	configManager := service.NewConfigManager(loader)

	// Create model config registry for generation params and context window lookups.
	modelCfg := service.NewModelConfigRegistry()
	modelCfg.LoadFromConfig(context.Background(), loader)

	llmConfig := service.NewLLMConfigManager(llmConfigRepo, modelCfg)

	client := llm.NewDefaultLLMProvider(llmConfig, modelCfg, configManager)

	// Always use the config-backed client so providers added after startup
	// (Settings → LLM) are picked up on the next Chat call. Mock is only used
	// when explicitly injected via bootstrap.Config.LLM (tests).
	provider := cfg.LLM
	if provider == nil {
		provider = client
	}

	if err := service.SyncBuiltinToFS(appCfg.Data.Dir); err != nil {
		log.Printf("[bootstrap] builtin sync: %v", err)
	}
	ensureBuiltinKnowledge(knowledgeMgr)
	if err := mcpManager.SyncBuiltinMCP(); err != nil {
		log.Printf("[bootstrap] builtin mcp sync: %v", err)
	}

	marketReg := marketadapter.NewRegistry(appCfg.Market.Sources)
	marketMgr := service.NewMarketManager(configManager, marketReg, skills, agents, mcpManager, pluginManager)

	stream := dqruntime.NewStreamEventManager(st.StreamEvents())
	usageSink := dqruntime.NewUsageSink(st.Usage(), st.Sessions())
	stream.SetUsageSink(usageSink)
	go dqruntime.BackfillUsageFromStreamEvents(context.Background(), st.Usage(), st.Sessions(), st.StreamEvents())
	checkpointStore := turnlog.NewCheckpointStore(st.Checkpoints())
	fileChangeStore := turnlog.NewFileChangeStore(st.FileChanges())
	snapshotStore := turnlog.NewSnapshotStore(pm.ProjectDir)
	aiReview := service.NewAIReviewManager(pm, snapshotStore, fileChangeStore)

	sessions := service.NewSessionManager(st, nil, provider)
	eng := dqruntime.NewEngine(sessions, turnManager, pm, approvalManager, turnLogManager, agents, skills, knowledge, st.Memories(), provider, stream, checkpointStore, loader, appCfg.Data.Dir)
	eng.SetFileChangeStore(fileChangeStore)
	eng.SetTableStore(tableStore)
	eng.SetPreTurnSnapshot(func(ctx context.Context, projectID, sessionID, turnID, userInput string, extraPaths []string) {
		seen := map[string]bool{}
		var paths []string
		for _, p := range service.PathsFromOfficeEdit(userInput) {
			if p == "" || seen[p] {
				continue
			}
			seen[p] = true
			paths = append(paths, p)
		}
		for _, p := range extraPaths {
			p = strings.TrimSpace(p)
			if p == "" || seen[p] {
				continue
			}
			seen[p] = true
			paths = append(paths, p)
		}
		if len(paths) == 0 {
			return
		}
		_, _ = aiReview.SnapshotPaths(ctx, projectID, sessionID, turnID, paths)
	})
	sessions.SetEngine(eng)

	sb := sandbox.New(appCfg.Runtime.Sandbox)
	sb.SetGitCredentials(gitMgr)
	eng.SetSandbox(sb)
	mcpDialer.PrepareStdio = sb.PrepareMCPStdio
	// The unified sandbox manager is also the execution backend (container
	// lifecycle + env tar status). Tools only face port.Sandbox + the factory.
	eng.SetExecution(sb)
	br := dqbrowser.New(appCfg.Runtime.Browser)
	eng.RegisterTool(&builtin.ExecShell{Sandbox: sb})
	eng.RegisterTool(&builtin.ReadFile{})
	eng.RegisterTool(&builtin.ReadImage{
		SupportsImage: func(modelID string) bool {
			return modelCfg.SupportsVision(modelID)
		},
	})
	eng.RegisterTool(&builtin.Edit{})
	eng.RegisterTool(&builtin.Write{})
	eng.RegisterTool(&builtin.FileOp{})
	eng.RegisterTool(&builtin.ApplyPatch{})
	eng.RegisterTool(&builtin.Grep{})
	eng.RegisterTool(&builtin.Glob{})
	eng.RegisterTool(&builtin.TodoWrite{})
	searchCfgFn := func(ctx context.Context) (domain.SearchConfig, error) {
		return searchConfig.Get(ctx)
	}
	eng.RegisterTool(&builtin.WebFetch{ConfigFunc: searchCfgFn, Browser: br, Egress: sb})
	eng.RegisterTool(&builtin.WebSearch{ConfigFunc: searchCfgFn, Egress: sb})
	eng.RegisterTool(&builtin.HTTPRequest{ConfigFunc: searchCfgFn, Egress: sb})
	eng.RegisterTool(&builtin.AskUser{})
	eng.RegisterTool(&builtin.Sleep{})
	eng.RegisterTool(&builtin.ReadSkill{})
	memTopK := appCfg.Runtime.Memory.ReadTopK
	if memTopK <= 0 {
		memTopK = 10
	}
	eng.RegisterTool(&builtin.MemoryUpdate{Store: st.Memories()})
	eng.RegisterTool(&builtin.MemoryRead{Store: st.Memories(), TopK: memTopK})
	tableQuotas := func() domain.ConfigTableSection {
		q := domain.DefaultTableSection()
		if c, err := loader.Load(context.Background()); err == nil {
			got := c.Runtime.Table
			if got.MaxRowsPerUpsert > 0 {
				q.MaxRowsPerUpsert = got.MaxRowsPerUpsert
			}
			if got.MaxRowsPerTurn > 0 {
				q.MaxRowsPerTurn = got.MaxRowsPerTurn
			}
			if got.MaxRowsPerTable > 0 {
				q.MaxRowsPerTable = got.MaxRowsPerTable
			}
			if got.MaxRowBytes > 0 {
				q.MaxRowBytes = got.MaxRowBytes
			}
			if got.MaxTablesPerScope > 0 {
				q.MaxTablesPerScope = got.MaxTablesPerScope
			}
			if got.QueryDefaultLimit > 0 {
				q.QueryDefaultLimit = got.QueryDefaultLimit
			}
			if got.QueryMaxLimit > 0 {
				q.QueryMaxLimit = got.QueryMaxLimit
			}
			if got.MaxRowChars > 0 {
				q.MaxRowChars = got.MaxRowChars
			}
		}
		return q
	}
	tableBudget := builtin.NewTableTurnBudget()
	eng.RegisterTool(&builtin.TableUpsert{Store: tableStore, Quotas: tableQuotas, Budget: tableBudget})
	eng.RegisterTool(&builtin.TableGet{Store: tableStore, Quotas: tableQuotas})
	eng.RegisterTool(&builtin.TableQuery{Store: tableStore, Quotas: tableQuotas})
	eng.RegisterTool(&builtin.TableDelete{Store: tableStore})
	eng.RegisterTool(&builtin.TableList{Store: tableStore})

	eng.SetMCPCaller(mcpManager)
	eng.SetGitHubMCPReady(mcpManager.GitHubMCPReady)
	mcpManager.SetToolSync(eng)
	go mcpManager.AutoDiscoverAll(context.Background())

	automations := service.NewAutomationManager(st.Automations(), sessions, pm)
	automations.StartScheduler()

	eng.RecoverRunning(context.Background())

	// History retention: orphan cleanup always; age-based pruning opt-in via
	// runtime.retention.history_max_age_days. Runs at boot and daily.
	retentionAge := time.Duration(appCfg.Runtime.Retention.HistoryMaxAgeDays) * 24 * time.Hour
	go func() {
		for {
			stats, err := st.PruneHistory(context.Background(), retentionAge)
			switch {
			case err != nil:
				log.Printf("[bootstrap] history retention: %v", err)
			case stats.OrphanSessions > 0 || stats.AgedSessions > 0:
				log.Printf("[bootstrap] history retention: orphans=%d aged=%d events=%d entries=%d",
					stats.OrphanSessions, stats.AgedSessions, stats.DeletedEvents, stats.DeletedEntries)
			}
			time.Sleep(24 * time.Hour)
		}
	}()

	weixinPeer := service.NewWeixinPeerStore(st)
	feishuPeer := service.NewFeishuPeerStore(st, configManager)
	wecomPeer := service.NewWecomPeerStore(st, configManager)
	qqPeer := service.NewQQPeerStore(st, configManager)
	peers := service.NewMultiplexPeerStore(map[port.ChannelType]port.ChannelPeerStore{
		port.ChannelWeixin: weixinPeer,
		port.ChannelFeishu: feishuPeer,
		port.ChannelWecom:  wecomPeer,
		port.ChannelQQ:     qqPeer,
	})
	defaults := service.NewConfigChannelDefaults(configManager)
	ingress := service.NewChannelIngress(sessions, pm, peers, defaults)
	channels := service.NewChannelManager(ingress)

	weixin := service.NewWeixinBridge(st, sessions, pm, configManager, ingress)
	channels.RegisterRuntime(weixin)

	feishuAdapter := feishu.NewAdapter(appCfg.Channels.Feishu)
	feishuBridge := service.NewFeishuBridge(configManager, feishuAdapter, ingress)
	channels.RegisterRuntime(feishuBridge)

	wecomBridge := service.NewWecomBridge(configManager, ingress)
	channels.RegisterRuntime(wecomBridge)

	qqAdapter := qq.NewAdapter(appCfg.Channels.QQ)
	qqBridge := service.NewQQBridge(configManager, qqAdapter, ingress)
	channels.RegisterRuntime(qqBridge)

	if err := channels.SyncAll(context.Background()); err != nil {
		// Non-fatal: channels may be disabled or incomplete.
		_ = err
	}

	return &Core{
		Store:         st,
		TableStore:    tableStore,
		Engine:        eng,
		Sandbox:       sb,
		Execution:     sb,
		Browser:       br,
		Config:        appCfg,
		Loader:        loader,
		Sessions:      sessions,
		Projects:      pm,
		Git:           gitMgr,
		LLMConfig:     llmConfig,
		ConfigManager: configManager,
		SearchConfig:  searchConfig,
		Agents:        agents,
		Skills:        skills,
		Knowledge:     knowledgeMgr,
		Market:        marketMgr,
		TurnLogs:      turnLogManager,
		MCPServers:    mcpManager,
		Automations:   automations,
		Weixin:        weixin,
		Channels:      channels,
		Feishu:        feishuBridge,
		Wecom:         wecomBridge,
		QQ:            qqBridge,
		AIReview:      aiReview,
		PluginManager: pluginManager,
	}
}

// Close releases runtime resources (headless browser sessions and channel bridges).
func (c *Core) Close() error {
	if c == nil {
		return nil
	}
	if c.Automations != nil {
		c.Automations.StopScheduler()
	}
	if c.Channels != nil {
		c.Channels.StopAll()
	} else if c.Weixin != nil {
		c.Weixin.Stop()
	}
	var first error
	if c.Sandbox != nil {
		if err := c.Sandbox.Close(); err != nil && first == nil {
			first = err
		}
	}
	if c.TableStore != nil {
		if err := c.TableStore.Close(); err != nil && first == nil {
			first = err
		}
	}
	if c.Browser != nil {
		if err := c.Browser.Close(context.Background()); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// migrateLegacyTurnLogs runs the one-time import of on-disk turn JSONL files
// into the DB-backed turn log (single source of truth). Guarded by an
// app_meta marker; on partial failure the marker is not set so the
// idempotent import retries at next startup. Legacy files stay on disk as
// inert backups. Must run before Engine.RecoverRunning.
func migrateLegacyTurnLogs(st *sqlitestore.Store, pm *service.ProjectManager) {
	const marker = "turnlog_jsonl_imported_v1"
	ctx := context.Background()
	if _, ok, err := st.AppMeta().Get(ctx, marker); err != nil || ok {
		if err != nil {
			log.Printf("[bootstrap] turnlog import marker: %v", err)
		}
		return
	}
	n, err := turnlog.ImportLegacyJSONL(ctx, st.TurnLogs(), pm.ProjectDir)
	if err != nil {
		log.Printf("[bootstrap] turnlog jsonl import (will retry next start): imported=%d err=%v", n, err)
		return
	}
	if n > 0 {
		log.Printf("[bootstrap] turnlog jsonl import: %d turns migrated to DB", n)
	}
	if err := st.AppMeta().Set(ctx, marker, "done"); err != nil {
		log.Printf("[bootstrap] turnlog import marker set: %v", err)
	}
}

// migrateLegacySessionArtifacts runs the one-time import of on-disk session
// artifacts (checkpoint_*.json, file_changes.jsonl) into the DB. Files stay
// on disk as inert backups.
func migrateLegacySessionArtifacts(st *sqlitestore.Store, pm *service.ProjectManager) {
	const marker = "session_artifacts_imported_v1"
	ctx := context.Background()
	if _, ok, err := st.AppMeta().Get(ctx, marker); err != nil || ok {
		if err != nil {
			log.Printf("[bootstrap] session artifacts import marker: %v", err)
		}
		return
	}
	n, err := turnlog.ImportLegacyArtifacts(ctx, st.Checkpoints(), st.FileChanges(), pm.ProjectDir)
	if err != nil {
		log.Printf("[bootstrap] session artifacts import (will retry next start): imported=%d err=%v", n, err)
		return
	}
	if n > 0 {
		log.Printf("[bootstrap] session artifacts import: %d artifacts migrated to DB", n)
	}
	if err := st.AppMeta().Set(ctx, marker, "done"); err != nil {
		log.Printf("[bootstrap] session artifacts import marker set: %v", err)
	}
}

func ensureDefaultProject(pm *service.ProjectManager) {
	ctx := context.Background()
	projects, err := pm.List(ctx)
	if err != nil || len(projects) > 0 {
		return
	}
	pm.Create(ctx, domain.CreateProjectRequest{Name: "默认项目"})
}

func ensureBuiltinKnowledge(knowledgeMgr *service.KnowledgeManager) {
	for _, id := range home.KnowledgeDirs() {
		root := filepath.Join(paths.KnowledgeDir(), id)
		if err := knowledgeMgr.ScanBuiltinKnowledgeDir(context.Background(), id, root); err != nil {
			log.Printf("[bootstrap] scan builtin KB %s: %v", id, err)
		}
	}
}
