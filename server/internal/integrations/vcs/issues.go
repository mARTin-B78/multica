package vcs

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

func UpdateIssue(ctx context.Context, instanceURL, token, owner, repo string, number int32, title, body, state string) error {
	endpoint := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues/%d", NormalizeInstanceURL(instanceURL), url.PathEscape(owner), url.PathEscape(repo), number)
	return doIssueWrite(ctx, http.MethodPatch, endpoint, token, map[string]any{"title": title, "body": body, "state": state}, nil)
}

func CreateIssueComment(ctx context.Context, instanceURL, token, owner, repo string, number int32, body string) (string, error) {
	endpoint := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues/%d/comments", NormalizeInstanceURL(instanceURL), url.PathEscape(owner), url.PathEscape(repo), number)
	var out struct {
		ID int64 `json:"id"`
	}
	err := doIssueWrite(ctx, http.MethodPost, endpoint, token, map[string]any{"body": body}, &out)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(out.ID), nil
}

func doIssueWrite(ctx context.Context, method, endpoint, token string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("vcs issue API: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
