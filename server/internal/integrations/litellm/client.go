package litellm

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
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

// Model is the stable subset of LiteLLM's public Model Hub payload that
// Multica needs for discovery and agent model selection. LiteLLM adds fields
// to this response frequently, so the client deliberately projects rather
// than mirroring the complete upstream object.
type Model struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Provider    string `json:"provider,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Description string `json:"description,omitempty"`
}

// Agent is the discovery-safe subset of an A2A Agent Card. Credentials and
// provider-specific LiteLLM parameters are never returned by this client.
type Agent struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	URL             string `json:"url,omitempty"`
	Version         string `json:"version,omitempty"`
	ProtocolVersion string `json:"protocol_version,omitempty"`
}

type MCPServer struct {
	ServerID    string `json:"server_id,omitempty"`
	ServerName  string `json:"server_name"`
	Alias       string `json:"alias,omitempty"`
	Description string `json:"description,omitempty"`
	Transport   string `json:"transport"`
}

type PublishedResource struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func rawString(row map[string]json.RawMessage, keys ...string) string {
	for _, key := range keys {
		var value string
		if raw, ok := row[key]; ok && json.Unmarshal(raw, &value) == nil && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// Models lists the public catalog. The public endpoint has changed field
// names across LiteLLM releases (model_group, model_name, name), so parsing is
// intentionally tolerant while still requiring a usable name.
func (c *Client) Models(ctx context.Context) ([]Model, error) {
	var rows []map[string]json.RawMessage
	if err := c.get(ctx, "/public/model_hub", false, &rows); err != nil {
		return nil, err
	}
	out := make([]Model, 0, len(rows))
	for _, row := range rows {
		name := rawString(row, "model_group", "model_name", "name", "id")
		if name == "" {
			continue
		}
		id := rawString(row, "model_id", "id")
		if id == "" {
			id = name
		}
		out = append(out, Model{
			ID:          id,
			Name:        name,
			Provider:    rawString(row, "provider", "custom_llm_provider"),
			Mode:        rawString(row, "mode", "model_mode"),
			Description: rawString(row, "description"),
		})
	}
	return out, nil
}

// Agents lists public A2A Agent Cards from LiteLLM's Agent Hub.
func (c *Client) Agents(ctx context.Context) ([]Agent, error) {
	var rows []map[string]json.RawMessage
	if err := c.get(ctx, "/public/agent_hub", false, &rows); err != nil {
		return nil, err
	}
	out := make([]Agent, 0, len(rows))
	for _, row := range rows {
		name := rawString(row, "name", "agent_name")
		if name == "" {
			continue
		}
		id := rawString(row, "agent_id", "id")
		if id == "" {
			id = name
		}
		out = append(out, Agent{
			ID:              id,
			Name:            name,
			Description:     rawString(row, "description"),
			URL:             rawString(row, "url"),
			Version:         rawString(row, "version"),
			ProtocolVersion: rawString(row, "protocolVersion", "protocol_version"),
		})
	}
	return out, nil
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

func (c *Client) do(ctx context.Context, method, path, contentType string, body io.Reader, target any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("reach LiteLLM: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("LiteLLM returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(message)))
	}
	if target == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes+1)).Decode(target); err != nil {
		return fmt.Errorf("decode LiteLLM response: %w", err)
	}
	return nil
}

// PublishMCPServer creates a database-backed server in LiteLLM. Multica's MCP
// entry shape is close to LiteLLM's; only transport and static-header naming
// need normalization. Credential-bearing values stay server-to-server.
func (c *Client) PublishMCPServer(ctx context.Context, name string, config json.RawMessage) (PublishedResource, error) {
	var entry map[string]any
	if err := json.Unmarshal(config, &entry); err != nil {
		return PublishedResource{}, fmt.Errorf("decode Multica MCP server: %w", err)
	}
	transport, _ := entry["type"].(string)
	switch strings.ToLower(strings.TrimSpace(transport)) {
	case "", "remote", "streamable-http":
		transport = "http"
	case "local":
		transport = "stdio"
	}
	delete(entry, "type")
	if headers, ok := entry["headers"]; ok {
		entry["static_headers"] = headers
		delete(entry, "headers")
	}
	entry["server_name"] = name
	entry["alias"] = name
	entry["transport"] = transport
	body, err := json.Marshal(entry)
	if err != nil {
		return PublishedResource{}, err
	}
	var response map[string]json.RawMessage
	if err := c.do(ctx, http.MethodPost, "/v1/mcp/server", "application/json", bytes.NewReader(body), &response); err != nil {
		return PublishedResource{}, err
	}
	id := rawString(response, "server_id", "id")
	return PublishedResource{ID: id, Name: name}, nil
}

// PublishSkill uploads one complete Multica skill bundle to LiteLLM's stored
// Skills API. The archive format is the same portable .skill ZIP Multica
// already accepts on import.
func (c *Client) PublishSkill(ctx context.Context, name, description, content string, files map[string]string) (PublishedResource, error) {
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	writeFile := func(path, value string) error {
		writer, err := zw.Create(path)
		if err != nil {
			return err
		}
		_, err = io.WriteString(writer, value)
		return err
	}
	if err := writeFile("SKILL.md", content); err != nil {
		return PublishedResource{}, err
	}
	for path, value := range files {
		if err := writeFile(path, value); err != nil {
			return PublishedResource{}, err
		}
	}
	if err := zw.Close(); err != nil {
		return PublishedResource{}, err
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("display_title", name); err != nil {
		return PublishedResource{}, err
	}
	if description != "" {
		if err := mw.WriteField("description", description); err != nil {
			return PublishedResource{}, err
		}
	}
	part, err := mw.CreateFormFile("files[]", name+".skill")
	if err != nil {
		return PublishedResource{}, err
	}
	if _, err := part.Write(archive.Bytes()); err != nil {
		return PublishedResource{}, err
	}
	if err := mw.Close(); err != nil {
		return PublishedResource{}, err
	}
	var response map[string]json.RawMessage
	path := "/v1/skills?beta=true&custom_llm_provider=litellm_proxy"
	if err := c.do(ctx, http.MethodPost, path, mw.FormDataContentType(), bytes.NewReader(body.Bytes()), &response); err != nil {
		return PublishedResource{}, err
	}
	id := rawString(response, "id", "skill_id")
	return PublishedResource{ID: id, Name: name}, nil
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
		ServerID    string `json:"server_id"`
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
