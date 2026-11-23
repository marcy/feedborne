package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// notionConfig holds the settings for the optional "bookmark to Notion"
// feature. It is configured entirely through environment variables so no
// credential ever lands in the database or the repository:
//
//	FEEDBORNE_NOTION_TOKEN           internal integration token (ntn_...)
//	FEEDBORNE_NOTION_DATA_SOURCE_ID  data source (database) ID of the
//	                                 bookmark database, e.g. a Notion
//	                                 "My Links" database shared with the
//	                                 integration
//	FEEDBORNE_NOTION_API_BASE        API base URL (testing only)
type notionConfig struct {
	token        string
	dataSourceID string
	apiBase      string
}

// notionFromEnv returns the config and whether the feature is enabled
// (both the token and the data source ID are set).
func notionFromEnv() (notionConfig, bool) {
	cfg := notionConfig{
		token:        os.Getenv("FEEDBORNE_NOTION_TOKEN"),
		dataSourceID: os.Getenv("FEEDBORNE_NOTION_DATA_SOURCE_ID"),
		apiBase:      os.Getenv("FEEDBORNE_NOTION_API_BASE"),
	}
	if cfg.apiBase == "" {
		cfg.apiBase = "https://api.notion.com"
	}
	return cfg, cfg.token != "" && cfg.dataSourceID != ""
}

// createBookmarkPage creates one page in the configured data source with
// the entry's title in the "Name" title property and its URL in the
// "userDefined:URL" url property, matching the My Links database schema.
func (c notionConfig) createBookmarkPage(ctx context.Context, title, url string) error {
	payload := map[string]any{
		"parent": map[string]any{
			"type":          "data_source_id",
			"data_source_id": c.dataSourceID,
		},
		"properties": map[string]any{
			"Name": map[string]any{
				"title": []any{
					map[string]any{"text": map[string]any{"content": title}},
				},
			},
			"userDefined:URL": map[string]any{"url": url},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.apiBase+"/v1/pages", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Notion-Version", "2025-09-03")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("notion api: %s: %s", resp.Status, string(msg))
	}
	return nil
}
