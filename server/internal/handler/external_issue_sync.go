package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/integrations/vcs"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

var externalCommentMarker = regexp.MustCompile(`<!--\s*multica-comment:([0-9a-fA-F-]{36})\s*-->`)

func parseExternalTime(raw string) pgtype.Timestamptz {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return pgtype.Timestamptz{Time: t, Valid: true}
	}
	return pgtype.Timestamptz{}
}

func externalStatus(state string) string {
	if strings.EqualFold(state, "closed") {
		return "done"
	}
	return "backlog"
}

func (h *Handler) externalCreator(ctx context.Context, workspaceID, connectedBy pgtype.UUID) (pgtype.UUID, error) {
	if connectedBy.Valid {
		return connectedBy, nil
	}
	ids, err := h.Queries.ListWorkspaceManagerUserIDs(ctx, workspaceID)
	if err != nil || len(ids) == 0 {
		return pgtype.UUID{}, fmt.Errorf("external issue sync: workspace has no owner: %w", err)
	}
	return ids[0], nil
}

func (h *Handler) syncExternalIssue(ctx context.Context, workspaceID, sourceID, connectedBy pgtype.UUID, provider string, ev vcs.IssueEvent) {
	if ev.RepoOwner == "" || ev.RepoName == "" || ev.Number < 1 || ev.Title == "" {
		return
	}
	key := db.GetExternalIssueLinkParams{WorkspaceID: workspaceID, Provider: provider, SourceConnectionID: sourceID, RepoOwner: ev.RepoOwner, RepoName: ev.RepoName, IssueNumber: ev.Number}
	link, err := h.Queries.GetExternalIssueLink(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		issueID := dbid.NewV7()
		link, err = h.Queries.ReserveExternalIssueLink(ctx, db.ReserveExternalIssueLinkParams{
			WorkspaceID: workspaceID, Provider: provider, SourceConnectionID: sourceID, RepoOwner: ev.RepoOwner,
			RepoName: ev.RepoName, IssueNumber: ev.Number, IssueID: issueID, HtmlUrl: ev.HTMLURL, ExternalUpdatedAt: parseExternalTime(ev.UpdatedAt),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			link, err = h.Queries.GetExternalIssueLink(ctx, key)
		}
		if err != nil {
			slog.Warn("external issue: reserve failed", "provider", provider, "error", err)
			return
		}
		if link.IssueID == issueID {
			creator, creatorErr := h.externalCreator(ctx, workspaceID, connectedBy)
			if creatorErr != nil {
				_ = h.Queries.DeleteExternalIssueLinkReservation(ctx, db.DeleteExternalIssueLinkReservationParams{ID: link.ID, IssueID: issueID})
				return
			}
			result, createErr := h.IssueService.Create(ctx, service.IssueCreateParams{ID: issueID, WorkspaceID: workspaceID, Title: ev.Title,
				Description: pgtype.Text{String: ev.Body, Valid: ev.Body != ""}, Status: externalStatus(ev.State), Priority: "none",
				CreatorType: "member", CreatorID: creator, AllowDuplicate: true}, service.IssueCreateOpts{Platform: "webhook",
				BroadcastPayload: func(i db.Issue, _ []db.Attachment, _ []db.IssueLabel) map[string]any {
					return map[string]any{"issue": issueToResponse(i, h.getIssuePrefix(ctx, workspaceID))}
				},
			})
			if createErr != nil {
				_ = h.Queries.DeleteExternalIssueLinkReservation(ctx, db.DeleteExternalIssueLinkReservationParams{ID: link.ID, IssueID: issueID})
				slog.Warn("external issue: create failed", "error", createErr)
				return
			}
			_ = result
			return
		}
	} else if err != nil {
		slog.Warn("external issue: lookup failed", "error", err)
		return
	}

	current, err := h.Queries.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: link.IssueID, WorkspaceID: workspaceID})
	if err != nil {
		return
	}
	nextStatus := externalStatus(ev.State)
	if current.Title == ev.Title && current.Description.String == ev.Body && current.Status == nextStatus {
		_, _ = h.Queries.UpdateExternalIssueLink(ctx, db.UpdateExternalIssueLinkParams{ID: link.ID, HtmlUrl: ev.HTMLURL, ExternalUpdatedAt: parseExternalTime(ev.UpdatedAt)})
		return
	}
	updated, err := h.Queries.UpdateIssue(ctx, db.UpdateIssueParams{ID: current.ID,
		Title: pgtype.Text{String: ev.Title, Valid: true}, Description: pgtype.Text{String: ev.Body, Valid: true}, Status: pgtype.Text{String: nextStatus, Valid: true}})
	if err != nil {
		slog.Warn("external issue: update failed", "error", err)
		return
	}
	_, _ = h.Queries.UpdateExternalIssueLink(ctx, db.UpdateExternalIssueLinkParams{ID: link.ID, HtmlUrl: ev.HTMLURL, ExternalUpdatedAt: parseExternalTime(ev.UpdatedAt)})
	h.publish(protocol.EventIssueUpdated, uuidToString(workspaceID), "system", "", map[string]any{
		"issue": issueToResponse(updated, h.getIssuePrefix(ctx, workspaceID)), "title_changed": current.Title != updated.Title,
		"description_changed": current.Description != updated.Description, "status_changed": current.Status != updated.Status, "prev_status": current.Status,
	})
}

func (h *Handler) syncExternalComment(ctx context.Context, workspaceID, sourceID, connectedBy pgtype.UUID, provider string, ev vcs.IssueCommentEvent) {
	if ev.Action != "created" || ev.IsPullRequest || ev.CommentID == "" {
		return
	}
	link, err := h.Queries.GetExternalIssueLink(ctx, db.GetExternalIssueLinkParams{WorkspaceID: workspaceID, Provider: provider, SourceConnectionID: sourceID, RepoOwner: ev.RepoOwner, RepoName: ev.RepoName, IssueNumber: ev.IssueNumber})
	if err != nil {
		return
	}
	if m := externalCommentMarker.FindStringSubmatch(ev.Body); len(m) == 2 {
		if id, parseErr := util.ParseUUID(m[1]); parseErr == nil {
			_, _ = h.Queries.ReserveExternalCommentLink(ctx, db.ReserveExternalCommentLinkParams{ExternalIssueID: link.ID, ExternalCommentID: ev.CommentID, CommentID: id})
		}
		return
	}
	commentID := dbid.NewV7()
	reserved, err := h.Queries.ReserveExternalCommentLink(ctx, db.ReserveExternalCommentLinkParams{ExternalIssueID: link.ID, ExternalCommentID: ev.CommentID, CommentID: commentID})
	if errors.Is(err, pgx.ErrNoRows) {
		return
	}
	if err != nil {
		slog.Warn("external comment: reserve failed", "error", err)
		return
	}
	creator, err := h.externalCreator(ctx, workspaceID, connectedBy)
	if err != nil {
		_ = h.Queries.DeleteExternalCommentLinkReservation(ctx, db.DeleteExternalCommentLinkReservationParams{ID: reserved.ID, CommentID: commentID})
		return
	}
	label := strings.Title(provider)
	content := fmt.Sprintf("**%s @%s:**\n\n%s", label, ev.AuthorLogin, ev.Body)
	created, err := h.Queries.CreateComment(ctx, db.CreateCommentParams{ID: commentID, IssueID: link.IssueID, WorkspaceID: workspaceID, AuthorType: "member", AuthorID: creator, Content: content, Type: "comment"})
	if err != nil {
		_ = h.Queries.DeleteExternalCommentLinkReservation(ctx, db.DeleteExternalCommentLinkReservationParams{ID: reserved.ID, CommentID: commentID})
		return
	}
	c := created.Comment()
	resp := commentToResponse(c, nil, nil)
	resp.IssueRevision = created.IssueRevision
	h.publish(protocol.EventCommentCreated, uuidToString(workspaceID), "system", "", map[string]any{"comment": resp, "issue_revision": created.IssueRevision})
}

func (h *Handler) registerExternalIssueSyncListeners() {
	if h.Bus == nil {
		return
	}
	h.Bus.Subscribe(protocol.EventIssueUpdated, func(e events.Event) {
		if e.ActorType == "system" {
			return
		}
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		issue, ok := payload["issue"].(IssueResponse)
		if !ok {
			return
		}
		go h.pushExternalIssue(context.Background(), issue)
	})
	h.Bus.Subscribe(protocol.EventCommentCreated, func(e events.Event) {
		if e.ActorType == "system" {
			return
		}
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		raw, ok := payload["comment"]
		if !ok {
			return
		}
		b, _ := json.Marshal(raw)
		var c struct {
			ID      string `json:"id"`
			IssueID string `json:"issue_id"`
			Content string `json:"content"`
		}
		if json.Unmarshal(b, &c) != nil || c.ID == "" || c.IssueID == "" {
			return
		}
		go h.pushExternalComment(context.Background(), e.ActorType, c.ID, c.IssueID, c.Content)
	})
}

func (h *Handler) pushExternalIssue(ctx context.Context, issue IssueResponse) {
	id, err := util.ParseUUID(issue.ID)
	if err != nil {
		return
	}
	link, err := h.Queries.GetExternalIssueLinkByIssue(ctx, id)
	if err != nil {
		return
	}
	body := ""
	if issue.Description != nil {
		body = *issue.Description
	}
	state := "open"
	if issue.StatusCategory == "done" || issue.StatusCategory == "cancelled" || issue.Status == "done" || issue.Status == "cancelled" {
		state = "closed"
	}
	if err := h.writeExternalIssue(ctx, link, issue.Title, body, state); err != nil {
		slog.Warn("external issue: outbound update failed", "provider", link.Provider, "error", err)
	}
}

func (h *Handler) pushExternalComment(ctx context.Context, actorType, commentID, issueID, content string) {
	cid, err := util.ParseUUID(commentID)
	if err != nil {
		return
	}
	if _, err = h.Queries.GetExternalCommentLinkByComment(ctx, cid); err == nil {
		return
	}
	iid, err := util.ParseUUID(issueID)
	if err != nil {
		return
	}
	link, err := h.Queries.GetExternalIssueLinkByIssue(ctx, iid)
	if err != nil {
		return
	}
	body := fmt.Sprintf("%s\n\n_Reply from Multica %s._\n\n<!-- multica-comment:%s -->", content, actorType, commentID)
	externalID, err := h.writeExternalComment(ctx, link, body)
	if err != nil {
		slog.Warn("external issue: outbound comment failed", "provider", link.Provider, "error", err)
		return
	}
	_, _ = h.Queries.ReserveExternalCommentLink(ctx, db.ReserveExternalCommentLinkParams{ExternalIssueID: link.ID, ExternalCommentID: externalID, CommentID: cid})
}

func (h *Handler) writeExternalIssue(ctx context.Context, link db.ExternalIssueLink, title, body, state string) error {
	if link.Provider == "github" {
		inst, err := h.Queries.GetGitHubInstallationByID(ctx, link.SourceConnectionID)
		if err != nil {
			return err
		}
		return h.GitHubIssues.UpdateIssue(ctx, inst.InstallationID, link.RepoOwner, link.RepoName, link.IssueNumber, title, body, state)
	}
	conn, err := h.Queries.GetVCSConnectionByID(ctx, link.SourceConnectionID)
	if err != nil {
		return err
	}
	token, err := h.openVCSSecret(conn.AccessTokenEncrypted)
	if err != nil {
		return err
	}
	return vcs.UpdateIssue(ctx, conn.InstanceUrl, token, link.RepoOwner, link.RepoName, link.IssueNumber, title, body, state)
}

func (h *Handler) writeExternalComment(ctx context.Context, link db.ExternalIssueLink, body string) (string, error) {
	if link.Provider == "github" {
		inst, err := h.Queries.GetGitHubInstallationByID(ctx, link.SourceConnectionID)
		if err != nil {
			return "", err
		}
		return h.GitHubIssues.CreateIssueComment(ctx, inst.InstallationID, link.RepoOwner, link.RepoName, link.IssueNumber, body)
	}
	conn, err := h.Queries.GetVCSConnectionByID(ctx, link.SourceConnectionID)
	if err != nil {
		return "", err
	}
	token, err := h.openVCSSecret(conn.AccessTokenEncrypted)
	if err != nil {
		return "", err
	}
	return vcs.CreateIssueComment(ctx, conn.InstanceUrl, token, link.RepoOwner, link.RepoName, link.IssueNumber, body)
}
