package ghsnapshot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) UpdateIssue(ctx context.Context, installationID int64, owner, repo string, number int32, title, body, state string) error {
	if c == nil {
		return fmt.Errorf("github App API is disabled")
	}
	endpoint := fmt.Sprintf("%s/repos/%s/%s/issues/%d", strings.TrimRight(c.apiBase, "/"), url.PathEscape(owner), url.PathEscape(repo), number)
	return c.issueWrite(ctx, installationID, http.MethodPatch, endpoint, map[string]any{"title": title, "body": body, "state": state}, nil)
}

func (c *Client) CreateIssueComment(ctx context.Context, installationID int64, owner, repo string, number int32, body string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("github App API is disabled")
	}
	endpoint := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", strings.TrimRight(c.apiBase, "/"), url.PathEscape(owner), url.PathEscape(repo), number)
	var out struct {
		ID int64 `json:"id"`
	}
	if err := c.issueWrite(ctx, installationID, http.MethodPost, endpoint, map[string]string{"body": body}, &out); err != nil {
		return "", err
	}
	return fmt.Sprint(out.ID), nil
}

func (c *Client) issueWrite(ctx context.Context, installationID int64, method, endpoint string, payload, out any) error {
	if c == nil {
		return fmt.Errorf("github App API is disabled")
	}
	token, err := c.installationToken(ctx, installationID)
	if err != nil {
		return err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("github issue API: unexpected status %d", resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
