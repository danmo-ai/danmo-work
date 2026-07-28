package permission

import (
	"strings"

	"danmo-work/core/domain"
)

// SearchProviderHosts returns Soft/Hard allowlist hosts for a web_search provider.
func SearchProviderHosts(provider, baseURL string) []string {
	if u := hostFromURL(baseURL); u != "" {
		return []string{u}
	}
	p := domain.SearchProvider(strings.ToLower(strings.TrimSpace(provider)))
	switch p {
	case domain.SearchProviderTavily:
		return []string{"api.tavily.com"}
	case domain.SearchProviderBrave:
		return []string{"api.search.brave.com"}
	case domain.SearchProviderBing:
		return []string{"www.bing.com", "bing.com"}
	case domain.SearchProviderBaidu:
		return []string{"qianfan.baidubce.com"}
	case domain.SearchProviderBocha:
		return []string{"api.bochaai.com"}
	case domain.SearchProviderMetaso:
		return []string{"metaso.cn"}
	case domain.SearchProviderVolcengine:
		return []string{"ark.cn-beijing.volces.com"}
	case domain.SearchProviderSofya:
		return []string{"sofya.co"}
	case domain.SearchProviderSearxng:
		return nil // requires base_url
	case "", domain.SearchProviderDuckDuckGo:
		return []string{"html.duckduckgo.com", "duckduckgo.com", "*.duckduckgo.com"}
	default:
		return nil
	}
}
