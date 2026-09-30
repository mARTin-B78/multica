package litellm

import (
	"context"
	"errors"
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

func TestNewRejectsCredentialBearingURL(t *testing.T) {
	if _, err := New("https://user:pass@example.com", "secret"); err == nil {
		t.Fatal("expected credential-bearing URL to be rejected")
	}
}
