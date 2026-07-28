package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"danmo-work/core/domain"
)

const (
	defaultMaxResults = 5
	maxResults        = 10
)

type webSearchEntry struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

type webSearchResult struct {
	Query   string           `json:"query"`
	Source  string           `json:"source"`
	Count   int              `json:"count"`
	Message string           `json:"message"`
	Results []webSearchEntry `json:"results"`
}

type searchLocale struct {
	Region   string
	Language string
}

// WebSearch searches the web using configurable providers.
type WebSearch struct {
	ConfigFunc func(context.Context) (domain.SearchConfig, error)
	Egress     HostEgressChecker
}

func (h *WebSearch) Name() string                { return "web_search" }
func (h *WebSearch) RiskLevel() domain.RiskLevel { return domain.RiskLow }
func (h *WebSearch) Describe(args map[string]any) string {
	query := extractSearchQuery(args)
	if len(query) > 80 {
		query = query[:80] + "..."
	}
	return query
}
func (h *WebSearch) Schema() domain.ToolSchema {
	return domain.ToolSchema{
		Name: "web_search",
		Description: "Search the web and return ranked results with URLs and snippets.\n\n" +
			"- query: search query string (required).\n" +
			"- max_results: result count (default 5, max 10).\n" +
			"- timeout_ms: timeout in milliseconds (default 15000, max 60000).\n" +
			"- region / language: optional locale hints (honored by Brave, Tavily, SearXNG when set).\n" +
			"- Use for accessing current events, latest docs, or best practices beyond your knowledge cutoff.\n" +
			"- Results include title, URL, and snippet for each match.\n" +
			"- Provider is configurable (DuckDuckGo default; also supports Bing, Brave, Tavily, SearXNG, Baidu, Volcengine, Sofya, Metaso, Bocha).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":       map[string]any{"type": "string", "description": "Search query"},
				"q":           map[string]any{"type": "string", "description": "Search query alias"},
				"max_results": map[string]any{"type": "integer", "description": "Maximum number of results (default 5, max 10)"},
				"timeout_ms":  map[string]any{"type": "integer", "description": "Timeout in milliseconds (default 15000, max 60000)"},
				"region":      map[string]any{"type": "string", "description": "Optional region/country code (e.g. us, cn)"},
				"language":    map[string]any{"type": "string", "description": "Optional language code (e.g. en, zh)"},
			},
			"required": []string{"query"},
		},
	}
}

func (h *WebSearch) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	query := extractSearchQuery(input)
	if query == "" {
		return domain.ToolResult{}, fmt.Errorf("query is required")
	}

	cfg := h.currentConfig(ctx)

	max := cfg.MaxResults
	if max <= 0 {
		max = defaultMaxResults
	}
	if m, ok := input["max_results"].(float64); ok {
		max = int(m)
	}
	if max < 1 {
		max = 1
	}
	if max > maxResults {
		max = maxResults
	}
	timeoutMs := cfg.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = defaultTimeoutMs
	}
	if t, ok := input["timeout_ms"].(float64); ok {
		timeoutMs = int(t)
	}
	if timeoutMs < 1000 {
		timeoutMs = 1000
	}
	if timeoutMs > maxTimeoutMs {
		timeoutMs = maxTimeoutMs
	}
	timeout := time.Duration(timeoutMs) * time.Millisecond
	locale := searchLocale{
		Region:   strings.TrimSpace(stringArg(input, "region")),
		Language: strings.TrimSpace(stringArg(input, "language")),
	}
	opts := withToolEgress(clientOpts(cfg.Proxy, cfg.UserAgent, timeout, true), h.Egress, input)

	if cfg.BaseURL != "" && cfg.Provider != domain.SearchProviderDuckDuckGo && cfg.Provider != domain.SearchProviderSearxng {
		return domain.ToolResult{}, fmt.Errorf("base_url is only supported with provider \"duckduckgo\" or \"searxng\"")
	}

	results, source, answer, err := h.runProvider(ctx, cfg, query, max, opts, locale)

	if (err != nil || len(results) == 0) && cfg.HTMLFallbackEnabled() && isAPISearchProvider(cfg.Provider) {
		fallback, fbSource, fbErr := runDuckDuckGoWithBingFallback(ctx, query, max, opts, cfg.BaseURL)
		if fbErr == nil && len(fallback) > 0 {
			results = fallback
			source = fbSource
			err = nil
			if answer == "" {
				answer = "Fell back to HTML search after primary provider returned no usable results"
			}
		}
	}

	if err != nil {
		return domain.ToolResult{}, err
	}

	msg := fmt.Sprintf("Found %d result(s)", len(results))
	if len(results) == 0 {
		msg = "No results found"
	}
	if answer != "" {
		msg = answer
	}
	res := webSearchResult{
		Query:   query,
		Source:  source,
		Count:   len(results),
		Message: msg,
		Results: results,
	}
	b, _ := json.Marshal(res)
	return domain.ToolResult{Content: string(b)}, nil
}

func (h *WebSearch) runProvider(ctx context.Context, cfg domain.SearchConfig, query string, max int, opts webClientOpts, locale searchLocale) ([]webSearchEntry, string, string, error) {
	var results []webSearchEntry
	var source string
	var answer string
	var err error

	switch cfg.Provider {
	case domain.SearchProviderTavily:
		results, answer, err = runTavilySearch(ctx, query, max, opts, cfg.APIKey, locale)
		source = "tavily"
	case domain.SearchProviderBocha:
		results, err = runBochaSearch(ctx, query, max, opts, cfg.APIKey)
		source = "bocha"
	case domain.SearchProviderMetaso:
		results, err = runMetasoSearch(ctx, query, max, opts, cfg.APIKey)
		source = "metaso"
	case domain.SearchProviderSearxng:
		results, err = runSearxngSearch(ctx, query, max, opts, cfg.BaseURL, locale)
		source = "searxng"
	case domain.SearchProviderBaidu:
		results, err = runBaiduSearch(ctx, query, max, opts, cfg.APIKey)
		source = "baidu"
	case domain.SearchProviderVolcengine:
		results, err = runVolcengineSearch(ctx, query, max, opts, cfg.APIKey)
		source = "volcengine"
	case domain.SearchProviderSofya:
		results, err = runSofyaSearch(ctx, query, max, opts, cfg.APIKey)
		source = "sofya"
	case domain.SearchProviderBrave:
		results, err = runBraveSearch(ctx, query, max, opts, cfg.APIKey, locale)
		source = "brave"
	case domain.SearchProviderBing:
		results, err = searchBing(ctx, query, max, opts)
		source = "bing"
		if err == nil && len(results) == 0 {
			fallback, err2 := searchDuckDuckGo(ctx, query, max, opts, "")
			if err2 == nil && len(fallback) > 0 {
				results = fallback
				source = "duckduckgo"
			}
		}
	default:
		results, source, err = runDuckDuckGoWithBingFallback(ctx, query, max, opts, cfg.BaseURL)
	}
	return results, source, answer, err
}

func (h *WebSearch) currentConfig(ctx context.Context) domain.SearchConfig {
	if h.ConfigFunc == nil {
		return domain.SearchConfig{Provider: domain.SearchProviderDuckDuckGo}
	}
	cfg, err := h.ConfigFunc(ctx)
	if err != nil || cfg.Provider == "" {
		return domain.SearchConfig{Provider: domain.SearchProviderDuckDuckGo}
	}
	return cfg
}

func isAPISearchProvider(p domain.SearchProvider) bool {
	switch p {
	case domain.SearchProviderTavily, domain.SearchProviderBocha, domain.SearchProviderMetaso,
		domain.SearchProviderSearxng, domain.SearchProviderBaidu, domain.SearchProviderVolcengine,
		domain.SearchProviderSofya, domain.SearchProviderBrave:
		return true
	}
	return false
}

func extractSearchQuery(input map[string]any) string {
	for _, key := range []string{"query", "q"} {
		if v, ok := input[key].(string); ok {
			v = strings.TrimSpace(v)
			if v != "" {
				return v
			}
		}
	}
	if arr, ok := input["search_query"].([]any); ok {
		for _, item := range arr {
			if m, ok := item.(map[string]any); ok {
				for _, key := range []string{"q", "query"} {
					if v, ok := m[key].(string); ok {
						v = strings.TrimSpace(v)
						if v != "" {
							return v
						}
					}
				}
			}
		}
	}
	return ""
}

func stringArg(input map[string]any, key string) string {
	v, _ := input[key].(string)
	return v
}

func runDuckDuckGoWithBingFallback(ctx context.Context, query string, max int, opts webClientOpts, baseURL string) ([]webSearchEntry, string, error) {
	results, err := searchDuckDuckGo(ctx, query, max, opts, baseURL)
	if err != nil || len(results) == 0 {
		fallback, err2 := searchBing(ctx, query, max, opts)
		if err2 == nil && len(fallback) > 0 {
			return fallback, "bing", nil
		}
	}
	if err != nil {
		return nil, "", err
	}
	if baseURL == "" {
		return results, "duckduckgo", nil
	}
	u, _ := url.Parse(strings.TrimSpace(baseURL))
	if u != nil && u.Host != "" {
		return results, u.Host, nil
	}
	return results, "duckduckgo", nil
}
