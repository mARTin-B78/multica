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
	Connected            bool   `json:"connected"`
	Configured           bool   `json:"configured"`
	ManagementConfigured bool   `json:"management_configured"`
	CanManage            bool   `json:"can_manage"`
	BaseURL              string `json:"base_url,omitempty"`
	ModelCount           int    `json:"model_count"`
	AgentCount           int    `json:"agent_count"`
	SkillCount           int    `json:"skill_count"`
	MCPCount             int    `json:"mcp_count"`
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
		if models, modelErr := client.Models(r.Context()); modelErr == nil {
			response.ModelCount = len(models)
		}
		if agents, agentErr := client.Agents(r.Context()); agentErr == nil {
			response.AgentCount = len(agents)
		}
	}
	response.Connected = true
	response.ManagementConfigured = row.ManagementApiKeyEncrypted.Valid && row.ManagementApiKeyEncrypted.String != ""
	response.BaseURL = row.BaseUrl
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) liteLLMManagementClient(r *http.Request, workspaceID pgtype.UUID) (*litellmintegration.Client, error) {
	row, err := h.Queries.GetLiteLLMConnectionByWorkspace(r.Context(), workspaceID)
	if err != nil {
		return nil, err
	}
	if h.LiteLLMSecretBox == nil || !row.ManagementApiKeyEncrypted.Valid || row.ManagementApiKeyEncrypted.String == "" {
		return nil, errors.New("LiteLLM management key is not configured")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(row.ManagementApiKeyEncrypted.String)
	if err != nil {
		return nil, err
	}
	plaintext, err := h.LiteLLMSecretBox.Open(ciphertext)
	if err != nil {
		return nil, err
	}
	return litellmintegration.New(row.BaseUrl, string(plaintext))
}

func (h *Handler) UpdateLiteLLMManagementKey(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workspace id")
	if !ok {
		return
	}
	if h.LiteLLMSecretBox == nil {
		writeError(w, http.StatusServiceUnavailable, "LiteLLM secrets are not configured")
		return
	}
	var body struct {
		APIKey string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.APIKey) == "" {
		writeError(w, http.StatusBadRequest, "LiteLLM management API key is required")
		return
	}
	row, _, err := h.liteLLMConnection(r, workspaceID)
	if err != nil {
		writeError(w, http.StatusNotFound, "LiteLLM is not connected")
		return
	}
	client, err := litellmintegration.New(row.BaseUrl, body.APIKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := client.MCPServers(r.Context()); err != nil {
		if errors.Is(err, litellmintegration.ErrUnauthorized) {
			writeError(w, http.StatusBadRequest, "LiteLLM rejected the management key")
			return
		}
		writeError(w, http.StatusBadGateway, "could not validate the LiteLLM management key")
		return
	}
	sealed, err := h.LiteLLMSecretBox.Seal([]byte(strings.TrimSpace(body.APIKey)))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to protect LiteLLM credentials")
		return
	}
	if _, err := h.Queries.UpdateLiteLLMManagementKey(r.Context(), db.UpdateLiteLLMManagementKeyParams{
		WorkspaceID:               workspaceID,
		ManagementApiKeyEncrypted: pgtype.Text{String: base64.StdEncoding.EncodeToString(sealed), Valid: true},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save LiteLLM management key")
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	response := liteLLMConnectionResponse{Connected: true, Configured: true, ManagementConfigured: row.ManagementApiKeyEncrypted.Valid && row.ManagementApiKeyEncrypted.String != "", CanManage: true, BaseURL: row.BaseUrl, SkillCount: skillCount, MCPCount: mcpCount}
	if models, modelErr := client.Models(r.Context()); modelErr == nil {
		response.ModelCount = len(models)
	}
	if agents, agentErr := client.Agents(r.Context()); agentErr == nil {
		response.AgentCount = len(agents)
	}
	writeJSON(w, http.StatusOK, response)
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

func (h *Handler) ListLiteLLMModels(w http.ResponseWriter, r *http.Request) {
	h.withLiteLLMClient(w, r, func(client *litellmintegration.Client) error {
		items, err := client.Models(r.Context())
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]any{"models": items})
		}
		return err
	})
}

func (h *Handler) ListLiteLLMAgents(w http.ResponseWriter, r *http.Request) {
	h.withLiteLLMClient(w, r, func(client *litellmintegration.Client) error {
		items, err := client.Agents(r.Context())
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]any{"agents": items})
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

func (h *Handler) PublishLiteLLMMCPServer(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workspace id")
	if !ok {
		return
	}
	serverID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "serverId"), "server id")
	if !ok {
		return
	}
	server, err := h.Queries.GetWorkspaceMcpServer(r.Context(), db.GetWorkspaceMcpServerParams{ID: serverID, WorkspaceID: workspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "workspace MCP server not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workspace MCP server")
		return
	}
	client, err := h.liteLLMManagementClient(r, workspaceID)
	if err != nil {
		writeError(w, http.StatusPreconditionFailed, err.Error())
		return
	}
	published, err := client.PublishMCPServer(r.Context(), server.Name, server.Config)
	if err != nil {
		if errors.Is(err, litellmintegration.ErrUnauthorized) {
			writeError(w, http.StatusForbidden, "LiteLLM management key cannot publish MCP servers")
			return
		}
		writeError(w, http.StatusBadGateway, "LiteLLM rejected the MCP server")
		return
	}
	writeJSON(w, http.StatusCreated, published)
}

func (h *Handler) PublishLiteLLMSkill(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workspace id")
	if !ok {
		return
	}
	skillID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "skillId"), "skill id")
	if !ok {
		return
	}
	skill, err := h.Queries.GetSkillInWorkspace(r.Context(), db.GetSkillInWorkspaceParams{ID: skillID, WorkspaceID: workspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "skill not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load skill")
		return
	}
	rows, err := h.Queries.ListSkillFiles(r.Context(), skillID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load skill files")
		return
	}
	files := make(map[string]string, len(rows))
	for _, file := range rows {
		files[file.Path] = file.Content
	}
	client, err := h.liteLLMManagementClient(r, workspaceID)
	if err != nil {
		writeError(w, http.StatusPreconditionFailed, err.Error())
		return
	}
	published, err := client.PublishSkill(r.Context(), skill.Name, skill.Description, skill.Content, files)
	if err != nil {
		if errors.Is(err, litellmintegration.ErrUnauthorized) {
			writeError(w, http.StatusForbidden, "LiteLLM management key cannot publish skills")
			return
		}
		writeError(w, http.StatusBadGateway, "LiteLLM rejected the skill")
		return
	}
	writeJSON(w, http.StatusCreated, published)
}
