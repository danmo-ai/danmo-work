package builtin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"danmo-work/core/domain"
)

func TestParseDuckDuckGoResults_Goquery(t *testing.T) {
	html := `
<html><body>
<div class="result">
  <a class="result__a" href="https://example.com/a">Alpha Title</a>
  <a class="result__snippet">Alpha snippet text</a>
</div>
<div class="result">
  <a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fb">Beta Title</a>
  <a class="result__snippet">Beta snippet</a>
</div>
</body></html>`
	results := parseDuckDuckGoResults(html, 5)
	if len(results) != 2 {
		t.Fatalf("len = %d, want 2", len(results))
	}
	if results[0].Title != "Alpha Title" || results[0].URL != "https://example.com/a" {
		t.Fatalf("first = %+v", results[0])
	}
	if results[1].URL != "https://example.com/b" {
		t.Fatalf("second URL = %q", results[1].URL)
	}
}

func TestParseBingResults_Goquery(t *testing.T) {
	html := `
<html><body>
<ol id="b_results">
<li class="b_algo">
  <h2><a href="https://example.com/bing">Bing Result</a></h2>
  <div class="b_caption"><p>A useful snippet</p></div>
</li>
</ol>
</body></html>`
	results := parseBingResults(html, 5)
	if len(results) != 1 {
		t.Fatalf("len = %d", len(results))
	}
	if results[0].Title != "Bing Result" || results[0].URL != "https://example.com/bing" {
		t.Fatalf("result = %+v", results[0])
	}
	if results[0].Snippet != "A useful snippet" {
		t.Fatalf("snippet = %q", results[0].Snippet)
	}
}

func TestRunTavilySearch_AdvancedPayload(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &payload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"answer":"summary answer",
			"results":[{"title":"T1","url":"https://example.com/1","content":"c1"}]
		}`))
	}))
	defer srv.Close()

	prev := tavilyEndpoint
	tavilyEndpoint = srv.URL
	defer func() { tavilyEndpoint = prev }()

	opts := webClientOpts{Timeout: 5 * time.Second, SkipSSRF: true}
	results, answer, err := runTavilySearch(context.Background(), "q", 3, opts, "key", searchLocale{Region: "us"})
	if err != nil {
		t.Fatal(err)
	}
	if payload["search_depth"] != "advanced" {
		t.Fatalf("search_depth = %v", payload["search_depth"])
	}
	if payload["include_answer"] != true {
		t.Fatalf("include_answer = %v", payload["include_answer"])
	}
	if payload["country"] != "us" {
		t.Fatalf("country = %v", payload["country"])
	}
	if answer != "summary answer" {
		t.Fatalf("answer = %q", answer)
	}
	if len(results) != 1 || results[0].Title != "T1" {
		t.Fatalf("results = %+v", results)
	}
}

func TestRunBraveSearch(t *testing.T) {
	var gotToken string
	var gotQ string
	var gotCountry string
	var gotLang string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Subscription-Token")
		gotQ = r.URL.Query().Get("q")
		gotCountry = r.URL.Query().Get("country")
		gotLang = r.URL.Query().Get("search_lang")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"web":{"results":[{"title":"Brave Hit","url":"https://example.com/b","description":"desc"}]}}`))
	}))
	defer srv.Close()

	prev := braveSearchEndpoint
	braveSearchEndpoint = srv.URL
	defer func() { braveSearchEndpoint = prev }()

	opts := webClientOpts{Timeout: 5 * time.Second, SkipSSRF: true}
	results, err := runBraveSearch(context.Background(), "golang", 5, opts, "test-key", searchLocale{Region: "us", Language: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if gotToken != "test-key" {
		t.Fatalf("token = %q", gotToken)
	}
	if gotQ != "golang" || gotCountry != "us" || gotLang != "en" {
		t.Fatalf("q/country/lang = %q/%q/%q", gotQ, gotCountry, gotLang)
	}
	if len(results) != 1 || results[0].Title != "Brave Hit" || results[0].Snippet != "desc" {
		t.Fatalf("results = %+v", results)
	}
}

func TestWebSearch_HTMLFallbackOnAPIFailure(t *testing.T) {
	testSkipSSRF = true
	defer func() { testSkipSSRF = false }()

	ddgHTML := `<html><body><div class="result"><a class="result__a" href="https://example.com/fb">Fallback</a><a class="result__snippet">ok</a></div></body></html>`
	apiFail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer apiFail.Close()
	ddgOK := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(ddgHTML))
	}))
	defer ddgOK.Close()

	prevTavily := tavilyEndpoint
	prevDDG := duckDuckGoEndpoint
	tavilyEndpoint = apiFail.URL
	duckDuckGoEndpoint = ddgOK.URL
	defer func() {
		tavilyEndpoint = prevTavily
		duckDuckGoEndpoint = prevDDG
	}()

	fb := true
	h := &WebSearch{ConfigFunc: func(context.Context) (domain.SearchConfig, error) {
		return domain.SearchConfig{
			Provider:     domain.SearchProviderTavily,
			APIKey:       "bad",
			HTMLFallback: &fb,
			MaxResults:   5,
			TimeoutMs:    5000,
		}, nil
	}}

	res, err := h.Execute(context.Background(), map[string]any{"query": "test"})
	if err != nil {
		t.Fatal(err)
	}
	var parsed webSearchResult
	if err := json.Unmarshal([]byte(res.Content), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Count < 1 || parsed.Results[0].Title != "Fallback" {
		t.Fatalf("parsed = %+v", parsed)
	}
}

func TestSearchConfig_HTMLFallbackDefault(t *testing.T) {
	var cfg domain.SearchConfig
	if !cfg.HTMLFallbackEnabled() {
		t.Fatal("default should be true")
	}
	f := false
	cfg.HTMLFallback = &f
	if cfg.HTMLFallbackEnabled() {
		t.Fatal("explicit false should disable")
	}
}
