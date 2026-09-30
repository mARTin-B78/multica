package litellm

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSkillsBuildsImportURLsAndFiltersUnsupportedSources(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/public/skill_hub" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"plugins":[{"id":"review","name":"Review","version":"1.2.0","description":"Review code","source":{"source":"git-subdir","url":"https://github.com/acme/skills","path":"skills/review"}},{"id":"local","name":"Local","source":{"source":"local","path":"/tmp/local"}}]}`))
	}))
	defer server.Close()

	client, err := New(server.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	skills, err := client.Skills(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if got, want := skills[0].ImportURL, "https://github.com/acme/skills/tree/HEAD/skills/review"; got != want {
		t.Fatalf("import URL = %q, want %q", got, want)
	}
}

func TestMCPServersAuthenticatesAndSanitizes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Bearer secret"; got != want {
			t.Fatalf("authorization = %q, want %q", got, want)
		}
		_, _ = w.Write([]byte(`[{"server_name":"github","alias":"GitHub","description":"Issues","transport":"http","url":"https://secret.invalid","credentials":{"token":"do-not-return"}},{"server_name":""}]`))
	}))
	defer server.Close()

	client, _ := New(server.URL, "secret")
	servers, err := client.MCPServers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 || servers[0].ServerName != "github" || servers[0].Alias != "GitHub" {
		t.Fatalf("unexpected servers: %#v", servers)
	}
}

func TestMCPServersReturnsUnauthorizedSentinel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	client, _ := New(server.URL, "wrong")
	_, err := client.MCPServers(context.Background())
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("error = %v, want ErrUnauthorized", err)
	}
}

func TestModelsProjectsPublicHubAcrossFieldVariants(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/public/model_hub" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
			{"model_group":"claude-sonnet","model_id":"deployment-1","provider":"anthropic","mode":"chat"},
			{"model_name":"embedding-small","custom_llm_provider":"openai","model_mode":"embedding"},
			{"description":"missing a name"}
		]`))
	}))
	defer server.Close()

	client, _ := New(server.URL, "secret")
	models, err := client.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 {
		t.Fatalf("got %d models, want 2", len(models))
	}
	if models[0].ID != "deployment-1" || models[0].Name != "claude-sonnet" || models[0].Provider != "anthropic" {
		t.Fatalf("first model = %#v", models[0])
	}
	if models[1].ID != "embedding-small" || models[1].Mode != "embedding" {
		t.Fatalf("second model = %#v", models[1])
	}
}

func TestAgentsProjectsPublicA2ACards(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/public/agent_hub" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
			{"agent_id":"researcher-id","name":"Researcher","description":"Finds sources","url":"https://example.test/a2a/researcher","version":"2","protocolVersion":"1.0"},
			{"agent_name":"Writer","protocol_version":"0.3"},
			{"description":"missing a name"}
		]`))
	}))
	defer server.Close()

	client, _ := New(server.URL, "secret")
	agents, err := client.Agents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 2 {
		t.Fatalf("got %d agents, want 2", len(agents))
	}
	if agents[0].ID != "researcher-id" || agents[0].ProtocolVersion != "1.0" {
		t.Fatalf("first agent = %#v", agents[0])
	}
	if agents[1].ID != "Writer" || agents[1].Name != "Writer" || agents[1].ProtocolVersion != "0.3" {
		t.Fatalf("second agent = %#v", agents[1])
	}
}

func TestPublishMCPServerNormalizesMulticaConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/mcp/server" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer management-key" {
			t.Fatalf("authorization = %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["server_name"] != "docs" || body["transport"] != "http" || body["url"] != "https://mcp.example.test" {
			t.Fatalf("body = %#v", body)
		}
		if _, ok := body["static_headers"].(map[string]any); !ok {
			t.Fatalf("static_headers = %#v", body["static_headers"])
		}
		if _, leaked := body["headers"]; leaked {
			t.Fatalf("body retained Multica-only headers field: %#v", body)
		}
		_, _ = w.Write([]byte(`{"server_id":"server-1"}`))
	}))
	defer server.Close()

	client, _ := New(server.URL, "management-key")
	published, err := client.PublishMCPServer(context.Background(), "docs", json.RawMessage(`{"type":"streamable-http","url":"https://mcp.example.test","headers":{"X-Key":"secret"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if published.ID != "server-1" || published.Name != "docs" {
		t.Fatalf("published = %#v", published)
	}
}

func TestPublishSkillUploadsPortableArchive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/skills" || r.URL.Query().Get("custom_llm_provider") != "litellm_proxy" {
			t.Fatalf("request = %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			t.Fatal(err)
		}
		if got := r.FormValue("display_title"); got != "review" {
			t.Fatalf("display_title = %q", got)
		}
		file, _, err := r.FormFile("files[]")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		got := make(map[string]string)
		for _, item := range zr.File {
			rc, err := item.Open()
			if err != nil {
				t.Fatal(err)
			}
			content, _ := io.ReadAll(rc)
			_ = rc.Close()
			got[item.Name] = string(content)
		}
		if got["SKILL.md"] != "# Review" || got["references/checklist.md"] != "Be precise" {
			t.Fatalf("archive = %#v", got)
		}
		_, _ = w.Write([]byte(`{"id":"skill-1"}`))
	}))
	defer server.Close()

	client, _ := New(server.URL, "management-key")
	published, err := client.PublishSkill(context.Background(), "review", "Review code", "# Review", map[string]string{"references/checklist.md": "Be precise"})
	if err != nil {
		t.Fatal(err)
	}
	if published.ID != "skill-1" {
		t.Fatalf("published = %#v", published)
	}
}

func TestNewRejectsCredentialBearingURL(t *testing.T) {
	if _, err := New("https://user:pass@example.com", "secret"); err == nil {
		t.Fatal("expected credential-bearing URL to be rejected")
	}
}
