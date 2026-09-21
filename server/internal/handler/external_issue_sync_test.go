package handler

import (
	"context"
	"testing"

	"github.com/multica-ai/multica/server/internal/integrations/vcs"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func TestExternalIssueSyncCreatesUpdatesAndDeduplicates(t *testing.T) {
	ctx := context.Background()
	workspaceID, userID, sourceID := parseUUID(testWorkspaceID), parseUUID(testUserID), dbid.NewV7()
	t.Cleanup(func() {
		_, _ = testPool.Exec(ctx, `DELETE FROM external_comment_link WHERE external_issue_id IN (SELECT id FROM external_issue_link WHERE source_connection_id=$1)`, sourceID)
		_, _ = testPool.Exec(ctx, `DELETE FROM issue WHERE id IN (SELECT issue_id FROM external_issue_link WHERE source_connection_id=$1)`, sourceID)
		_, _ = testPool.Exec(ctx, `DELETE FROM external_issue_link WHERE source_connection_id=$1`, sourceID)
	})
	ev := vcs.IssueEvent{Action:"opened",RepoOwner:"acme",RepoName:"widget",Number:17,Title:"Broken build",Body:"details",State:"open",HTMLURL:"https://g/acme/widget/issues/17"}
	testHandler.syncExternalIssue(ctx,workspaceID,sourceID,userID,"gitea",ev)
	link, err := testHandler.Queries.GetExternalIssueLink(ctx,db.GetExternalIssueLinkParams{WorkspaceID:workspaceID,Provider:"gitea",SourceConnectionID:sourceID,RepoOwner:"acme",RepoName:"widget",IssueNumber:17})
	if err != nil { t.Fatal(err) }
	created, err := testHandler.Queries.GetIssueInWorkspace(ctx,db.GetIssueInWorkspaceParams{ID:link.IssueID,WorkspaceID:workspaceID})
	if err != nil { t.Fatal(err) }
	if created.Title!="Broken build" || created.Status!="backlog" { t.Fatalf("created issue: %+v",created) }
	ev.Action="closed"; ev.Title="Build repaired"; ev.State="closed"
	testHandler.syncExternalIssue(ctx,workspaceID,sourceID,userID,"gitea",ev)
	updated, err := testHandler.Queries.GetIssueInWorkspace(ctx,db.GetIssueInWorkspaceParams{ID:link.IssueID,WorkspaceID:workspaceID})
	if err != nil { t.Fatal(err) }
	if updated.ID!=created.ID || updated.Title!="Build repaired" || updated.Status!="done" { t.Fatalf("updated issue: %+v",updated) }
}

func TestExternalCommentSyncDeduplicates(t *testing.T) {
	ctx := context.Background()
	workspaceID, userID, sourceID := parseUUID(testWorkspaceID), parseUUID(testUserID), dbid.NewV7()
	t.Cleanup(func() {
		_, _ = testPool.Exec(ctx, `DELETE FROM external_comment_link WHERE external_issue_id IN (SELECT id FROM external_issue_link WHERE source_connection_id=$1)`, sourceID)
		_, _ = testPool.Exec(ctx, `DELETE FROM comment WHERE issue_id IN (SELECT issue_id FROM external_issue_link WHERE source_connection_id=$1)`, sourceID)
		_, _ = testPool.Exec(ctx, `DELETE FROM issue WHERE id IN (SELECT issue_id FROM external_issue_link WHERE source_connection_id=$1)`, sourceID)
		_, _ = testPool.Exec(ctx, `DELETE FROM external_issue_link WHERE source_connection_id=$1`, sourceID)
	})
	testHandler.syncExternalIssue(ctx,workspaceID,sourceID,userID,"gitea",vcs.IssueEvent{RepoOwner:"acme",RepoName:"widget",Number:18,Title:"Question",State:"open"})
	ev := vcs.IssueCommentEvent{Action:"created",RepoOwner:"acme",RepoName:"widget",IssueNumber:18,CommentID:"501",Body:"Please check this",AuthorLogin:"alice"}
	testHandler.syncExternalComment(ctx,workspaceID,sourceID,userID,"gitea",ev)
	testHandler.syncExternalComment(ctx,workspaceID,sourceID,userID,"gitea",ev)
	link, err := testHandler.Queries.GetExternalIssueLink(ctx,db.GetExternalIssueLinkParams{WorkspaceID:workspaceID,Provider:"gitea",SourceConnectionID:sourceID,RepoOwner:"acme",RepoName:"widget",IssueNumber:18})
	if err != nil { t.Fatal(err) }
	var count int
	if err := testPool.QueryRow(ctx,`SELECT count(*) FROM comment WHERE issue_id=$1`,link.IssueID).Scan(&count); err != nil { t.Fatal(err) }
	if count!=1 { t.Fatalf("comment count=%d, want 1",count) }
	mapping, err := testHandler.Queries.GetExternalCommentLink(ctx,db.GetExternalCommentLinkParams{ExternalIssueID:link.ID,ExternalCommentID:"501"})
	if err != nil || !mapping.CommentID.Valid { t.Fatalf("comment mapping=%+v err=%v",mapping,err) }
}
