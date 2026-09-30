package litellm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseBytes = 4 << 20

var ErrUnauthorized = errors.New("litellm rejected the API key")

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

type Skill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	ImportURL   string `json:"import_url"`
}

type MCPServer struct {
	ServerName  string `json:"server_name"`
	Alias       string `json:"alias,omitempty"`
	Description string `json:"description,omitempty"`
	Transport   string `json:"transport"`
}

func New(baseURL, apiKey string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return nil, errors.New("LiteLLM URL must be an absolute http(s) URL without credentials")
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("LiteLLM API key is required")
	}
	return &Client{
		baseURL: baseURL,
		apiKey:  strings.TrimSpace(apiKey),
		http:    &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (c *Client) get(ctx context.Context, path string, authenticated bool, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	if authenticated {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("reach LiteLLM: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("LiteLLM returned HTTP %d", resp.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode LiteLLM response: %w", err)
	}
	return nil
}

func (c *Client) Skills(ctx context.Context) ([]Skill, error) {
	var payload struct {
		Plugins []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Version     string `json:"version"`
			Description string `json:"description"`
			Source      struct {
				Source string `json:"source"`
				URL    string `json:"url"`
				Path   string `json:"path"`
			} `json:"source"`
		} `json:"plugins"`
	}
	if err := c.get(ctx, "/public/skill_hub", false, &payload); err != nil {
		return nil, err
	}
	out := make([]Skill, 0, len(payload.Plugins))
	for _, item := range payload.Plugins {
		if item.Name == "" || item.Source.Source != "git-subdir" || item.Source.URL == "" || item.Source.Path == "" {
			continue
		}
		importURL := strings.TrimRight(item.Source.URL, "/") + "/tree/HEAD/" + strings.TrimLeft(item.Source.Path, "/")
		out = append(out, Skill{ID: item.ID, Name: item.Name, Version: item.Version, Description: item.Description, ImportURL: importURL})
	}
	return out, nil
}

func (c *Client) MCPServers(ctx context.Context) ([]MCPServer, error) {
	var rows []struct {
		ServerName  string `json:"server_name"`
		Alias       string `json:"alias"`
		Description string `json:"description"`
		Transport   string `json:"transport"`
	}
	if err := c.get(ctx, "/v1/mcp/server", true, &rows); err != nil {
		return nil, err
	}
	out := make([]MCPServer, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.ServerName) == "" {
			continue
		}
		out = append(out, MCPServer(row))
	}
	return out, nil
}

func (c *Client) Validate(ctx context.Context) (int, int, error) {
	skills, err := c.Skills(ctx)
	if err != nil {
		return 0, 0, err
	}
	servers, err := c.MCPServers(ctx)
	if err != nil {
		return 0, 0, err
	}
	return len(skills), len(servers), nil
}
