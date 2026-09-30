package handler

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	litellmintegration "github.com/multica-ai/multica/server/internal/integrations/litellm"
	"github.com/multica-ai/multica/server/internal/middleware"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type liteLLMConnectionResponse struct {
	Connected  bool   `json:"connected"`
	Configured bool   `json:"configured"`
	CanManage  bool   `json:"can_manage"`
	BaseURL    string `json:"base_url,omitempty"`
	SkillCount int    `json:"skill_count"`
	MCPCount   int    `json:"mcp_count"`
}

func (h *Handler) liteLLMConnection(ctxRequest *http.Request, workspaceID pgtype.UUID) (db.LitellmConnection, string, error) {
	row, err := h.Queries.GetLiteLLMConnectionByWorkspace(ctxRequest.Context(), workspaceID)
	if err != nil {
		return db.LitellmConnection{}, "", err
	}
	if h.LiteLLMSecretBox == nil {
		return db.LitellmConnection{}, "", errors.New("LiteLLM secrets are not configured")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(row.ApiKeyEncrypted)
	if err != nil {
		return db.LitellmConnection{}, "", err
	}
	plaintext, err := h.LiteLLMSecretBox.Open(ciphertext)
	if err != nil {
		return db.LitellmConnection{}, "", err
	}
	return row, string(plaintext), nil
}

func (h *Handler) ListLiteLLMConnection(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	member, _ := middleware.MemberFromContext(r.Context())
	response := liteLLMConnectionResponse{
		Configured: h.LiteLLMSecretBox != nil,
		CanManage:  roleAllowed(member.Role, "owner", "admin"),
	}
	row, key, err := h.liteLLMConnection(r, wsUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusOK, response)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load LiteLLM connection")
		return
	}
	client, err := litellmintegration.New(row.BaseUrl, key)
	if err == nil {
		response.SkillCount, response.MCPCount, _ = client.Validate(r.Context())
	}
	response.Connected = true
	response.BaseURL = row.BaseUrl
	writeJSON(w, http.StatusOK, response)
}

type connectLiteLLMRequest struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}

func (h *Handler) ConnectLiteLLM(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	if h.LiteLLMSecretBox == nil {
		writeError(w, http.StatusServiceUnavailable, "LiteLLM integration not configured (MULTICA_LITELLM_SECRET_KEY unset)")
		return
	}
	var body connectLiteLLMRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	client, err := litellmintegration.New(body.BaseURL, body.APIKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	skillCount, mcpCount, err := client.Validate(r.Context())
	if err != nil {
		if errors.Is(err, litellmintegration.ErrUnauthorized) {
			writeError(w, http.StatusBadRequest, "LiteLLM rejected the API key")
			return
		}
		writeError(w, http.StatusBadGateway, "could not validate the LiteLLM gateway")
		return
	}
	sealed, err := h.LiteLLMSecretBox.Seal([]byte(strings.TrimSpace(body.APIKey)))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to protect LiteLLM credentials")
		return
	}
	member, _ := middleware.MemberFromContext(r.Context())
	row, err := h.Queries.UpsertLiteLLMConnection(r.Context(), db.UpsertLiteLLMConnectionParams{
		WorkspaceID:     wsUUID,
		BaseUrl:         strings.TrimRight(strings.TrimSpace(body.BaseURL), "/"),
		ApiKeyEncrypted: base64.StdEncoding.EncodeToString(sealed),
		ConnectedByID:   member.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save LiteLLM connection")
		return
	}
	writeJSON(w, http.StatusOK, liteLLMConnectionResponse{Connected: true, Configured: true, CanManage: true, BaseURL: row.BaseUrl, SkillCount: skillCount, MCPCount: mcpCount})
}

func (h *Handler) DeleteLiteLLMConnection(w http.ResponseWriter, r *http.Request) {
	wsUUID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workspace id")
	if !ok {
		return
	}
	if err := h.Queries.DeleteLiteLLMConnection(r.Context(), wsUUID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to disconnect LiteLLM")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListLiteLLMSkills(w http.ResponseWriter, r *http.Request) {
	h.withLiteLLMClient(w, r, func(client *litellmintegration.Client) error {
		items, err := client.Skills(r.Context())
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]any{"skills": items})
		}
		return err
	})
}

func (h *Handler) ListLiteLLMMCPServers(w http.ResponseWriter, r *http.Request) {
	h.withLiteLLMClient(w, r, func(client *litellmintegration.Client) error {
		items, err := client.MCPServers(r.Context())
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]any{"servers": items})
		}
		return err
	})
}

func (h *Handler) withLiteLLMClient(w http.ResponseWriter, r *http.Request, fn func(*litellmintegration.Client) error) {
	wsUUID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workspace id")
	if !ok {
		return
	}
	row, key, err := h.liteLLMConnection(r, wsUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "LiteLLM is not connected")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load LiteLLM connection")
		return
	}
	client, err := litellmintegration.New(row.BaseUrl, key)
	if err == nil {
		err = fn(client)
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "LiteLLM gateway request failed")
	}
}

func (h *Handler) ImportLiteLLMMCPServer(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	serverName, err := url.PathUnescape(chi.URLParam(r, "serverName"))
	if err != nil || validateWorkspaceMcpServerName(serverName) != nil {
		writeError(w, http.StatusBadRequest, "invalid MCP server name")
		return
	}
	row, key, err := h.liteLLMConnection(r, wsUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "LiteLLM is not connected")
		return
	}
	client, err := litellmintegration.New(row.BaseUrl, key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid LiteLLM connection")
		return
	}
	servers, err := client.MCPServers(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not verify the LiteLLM MCP server")
		return
	}
	found := false
	for _, server := range servers {
		if server.ServerName == serverName {
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "LiteLLM MCP server not found")
		return
	}
	config, _ := json.Marshal(map[string]any{
		"type":    "http",
		"url":     strings.TrimRight(row.BaseUrl, "/") + "/" + url.PathEscape(serverName) + "/mcp",
		"headers": map[string]string{"x-litellm-api-key": "Bearer " + key},
	})
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to import MCP server")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)
	if _, err := qtx.LockWorkspaceForChatSessionCreate(r.Context(), wsUUID); err != nil {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	member, _ := middleware.MemberFromContext(r.Context())
	created, err := qtx.CreateWorkspaceMcpServer(r.Context(), db.CreateWorkspaceMcpServerParams{WorkspaceID: wsUUID, Name: serverName, Config: config, CreatedBy: member.ID})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "an MCP server with this name already exists in the workspace")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to import MCP server")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to import MCP server")
		return
	}
	writeJSON(w, http.StatusCreated, workspaceMcpServerToResponse(created))
}
