package tools

import (
	"testing"
)

func TestParseDuckDuckGoHTML(t *testing.T) {
	html := `
<html>
<body>
<div class="results">
<a class="result__a" href="https://example.com/result1">Example Result 1</a>
<a class="result__snippet">This is the first snippet</a>
<a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fresult2">Example Result 2</a>
<a class="result__snippet">This is the second snippet</a>
</div>
</body>
</html>
`
	results := parseDuckDuckGoHTML(html, 5)

	if len(results) == 0 {
		t.Error("Expected at least 1 result, got 0")
	}

	for i, r := range results {
		t.Logf("Result %d: Title=%s, URL=%s, Snippet=%s", i+1, r.Title, r.URL, r.Snippet)
	}

	if len(results) >= 1 {
		if results[0].Title != "Example Result 1" {
			t.Errorf("Expected title 'Example Result 1', got '%s'", results[0].Title)
		}
		if results[0].URL != "https://example.com/result1" {
			t.Errorf("Expected URL 'https://example.com/result1', got '%s'", results[0].URL)
		}
	}

	if len(results) >= 2 {
		if results[1].URL != "https://example.com/result2" {
			t.Errorf("Expected decoded URL 'https://example.com/result2', got '%s'", results[1].URL)
		}
	}
}

func TestParseDuckDuckGoLite(t *testing.T) {
	html := `
<html>
<body>
<table>
<tr><td class="result-link"><a href="https://example.com/result1">Example Result 1</a></td></tr>
<tr><td class="result-snippet"></td><td>This is the first snippet</td></tr>
<tr><td class="result-link"><a href="https://example.com/result2">Example Result 2</a></td></tr>
<tr><td class="result-snippet"></td><td>This is the second snippet</td></tr>
</table>
</body>
</html>
`
	results := parseDuckDuckGoLite(html, 5)

	if len(results) == 0 {
		t.Log("No results parsed from HTML - checking actual DuckDuckGo response format")
	}

	for i, r := range results {
		t.Logf("Result %d: Title=%s, URL=%s, Snippet=%s", i+1, r.Title, r.URL, r.Snippet)
	}
}

func TestWebSearchToolCreation(t *testing.T) {
	tool := NewWebSearchTool()

	if tool.Name() != "web_search" {
		t.Errorf("Expected name 'web_search', got '%s'", tool.Name())
	}

	if tool.provider != "duckduckgo" {
		t.Errorf("Expected default provider 'duckduckgo', got '%s'", tool.provider)
	}
}

func TestWebSearchToolWithOptions(t *testing.T) {
	tool := NewWebSearchTool(
		WithSearchProvider("searxng"),
		WithSearchBaseURL("http://localhost:8888"),
		WithSearchAPIKey("test-key"),
	)

	if tool.provider != "searxng" {
		t.Errorf("Expected provider 'searxng', got '%s'", tool.provider)
	}

	if tool.baseURL != "http://localhost:8888" {
		t.Errorf("Expected baseURL 'http://localhost:8888', got '%s'", tool.baseURL)
	}

	if tool.apiKey != "test-key" {
		t.Errorf("Expected apiKey 'test-key', got '%s'", tool.apiKey)
	}
}

// TestDuckDuckGoLive 测试实际 DuckDuckGo 搜索
// 运行: go test -v -run TestDuckDuckGoLive ./pkg/agent/tools/
func TestDuckDuckGoLive(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live test in short mode")
	}

	tool := NewWebSearchTool()
	results, err := tool.searchDuckDuckGo("golang programming language", 5)

	if err != nil {
		t.Logf("DuckDuckGo search error: %v", err)
		t.Skip("DuckDuckGo may be blocked or unavailable")
	}

	t.Logf("Found %d results", len(results))
	for i, r := range results {
		t.Logf("Result %d:\n  Title: %s\n  URL: %s\n  Snippet: %s\n", i+1, r.Title, r.URL, r.Snippet)
	}

	if len(results) == 0 {
		t.Log("No results found - DuckDuckGo format may have changed")
	}
}

func TestExtractReadableContent(t *testing.T) {
	html := `
<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
<nav>Navigation</nav>
<header>Header</header>
<article>
<h1>Main Title</h1>
<p>This is the main content paragraph.</p>
<p>Another paragraph with <strong>bold text</strong>.</p>
</article>
<footer>Footer</footer>
</body>
</html>
`
	content := extractReadableContent(html)

	t.Logf("Extracted content:\n%s", content)

	if content == "" {
		t.Error("Expected non-empty content")
	}

	if !contains(content, "Main Title") {
		t.Error("Expected content to contain 'Main Title'")
	}

	if contains(content, "Navigation") {
		t.Error("Expected content to NOT contain 'Navigation'")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
