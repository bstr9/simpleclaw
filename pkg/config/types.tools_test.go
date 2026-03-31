package config

import (
	"testing"
)

func TestWebSearchConfig_IsSearchEnabled(t *testing.T) {
	t.Run("nil config returns true (duckduckgo default)", func(t *testing.T) {
		var c *WebSearchConfig
		if !c.IsSearchEnabled() {
			t.Error("expected true for nil config (duckduckgo is free)")
		}
	})

	t.Run("enabled true", func(t *testing.T) {
		enabled := true
		c := &WebSearchConfig{Enabled: &enabled}
		if !c.IsSearchEnabled() {
			t.Error("expected true when Enabled is true")
		}
	})

	t.Run("enabled false", func(t *testing.T) {
		enabled := false
		c := &WebSearchConfig{Enabled: &enabled}
		if c.IsSearchEnabled() {
			t.Error("expected false when Enabled is false")
		}
	})

	t.Run("no api key still enabled (duckduckgo)", func(t *testing.T) {
		c := &WebSearchConfig{}
		if !c.IsSearchEnabled() {
			t.Error("expected true when no API key (duckduckgo is free)")
		}
	})
}

func TestWebFetchConfig_IsFetchEnabled(t *testing.T) {
	t.Run("nil config returns true (default)", func(t *testing.T) {
		var c *WebFetchConfig
		if !c.IsFetchEnabled() {
			t.Error("expected true for nil config (default)")
		}
	})

	t.Run("enabled true", func(t *testing.T) {
		enabled := true
		c := &WebFetchConfig{Enabled: &enabled}
		if !c.IsFetchEnabled() {
			t.Error("expected true when Enabled is true")
		}
	})

	t.Run("enabled false", func(t *testing.T) {
		enabled := false
		c := &WebFetchConfig{Enabled: &enabled}
		if c.IsFetchEnabled() {
			t.Error("expected false when Enabled is false")
		}
	})
}

func TestWebSearchConfig_GetSearchProvider(t *testing.T) {
	t.Run("nil config returns duckduckgo", func(t *testing.T) {
		var c *WebSearchConfig
		if c.GetSearchProvider() != "duckduckgo" {
			t.Error("expected duckduckgo for nil config (free default)")
		}
	})

	t.Run("provider set", func(t *testing.T) {
		c := &WebSearchConfig{Provider: "brave"}
		if c.GetSearchProvider() != "brave" {
			t.Error("expected brave")
		}
	})

	t.Run("auto-detect from api key", func(t *testing.T) {
		c := &WebSearchConfig{APIKey: "test-key"}
		if c.GetSearchProvider() != "brave" {
			t.Error("expected brave from auto-detect")
		}
	})

	t.Run("auto-detect from gemini", func(t *testing.T) {
		c := &WebSearchConfig{Gemini: &GeminiSearchConfig{APIKey: "test-key"}}
		if c.GetSearchProvider() != "gemini" {
			t.Error("expected gemini from auto-detect")
		}
	})

	t.Run("no api key returns duckduckgo", func(t *testing.T) {
		c := &WebSearchConfig{}
		if c.GetSearchProvider() != "duckduckgo" {
			t.Error("expected duckduckgo when no API key (free default)")
		}
	})
}

func TestConfig_WebTools(t *testing.T) {
	t.Run("no tools config", func(t *testing.T) {
		c := &Config{}
		if !c.IsWebSearchEnabled() {
			t.Error("expected true when no tools config (duckduckgo is free)")
		}
		if !c.IsWebFetchEnabled() {
			t.Error("expected true (default) when no tools config")
		}
	})

	t.Run("search disabled", func(t *testing.T) {
		enabled := false
		c := &Config{
			Tools: &ToolsConfig{
				Web: &WebToolsConfig{
					Search: &WebSearchConfig{Enabled: &enabled},
				},
			},
		}
		if c.IsWebSearchEnabled() {
			t.Error("expected false when search disabled")
		}
	})

	t.Run("fetch disabled", func(t *testing.T) {
		enabled := false
		c := &Config{
			Tools: &ToolsConfig{
				Web: &WebToolsConfig{
					Fetch: &WebFetchConfig{Enabled: &enabled},
				},
			},
		}
		if c.IsWebFetchEnabled() {
			t.Error("expected false when fetch disabled")
		}
	})
}
