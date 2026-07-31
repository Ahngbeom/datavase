// Package gitlab reads SQL out of a GitLab project.
//
// Read-only, and deliberately so: this is a database client, and the useful
// thing a GitLab account adds to one is the migration sitting in a merge
// request that has not been merged yet. Writing back is version control's job.
//
// It knows nothing about screens or configuration — a base URL, a token and a
// context go in, and Go values come out. That is also what lets every one of
// its tests run against an httptest server, so the unit suite never touches
// the network.
//
// Only the standard library. The one binary this ships as is CGO-free and
// dependency-light, and an API this small does not pay for an SDK.
package gitlab

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

// MaxFileSize is the largest file that will be loaded, matching the limit on
// files read from disk. The editor holds a whole buffer in memory and cannot
// page one.
const MaxFileSize = 4 << 20

// timeout bounds every request.
//
// Longer than the two seconds git gets locally, because this crosses a
// network; still short enough that a hung proxy is an error message rather
// than a frozen session.
const timeout = 10 * time.Second

// Project is the repository a session is looking at.
type Project struct {
	ID   int    `json:"id"`
	Path string `json:"path_with_namespace"`
}

// MergeRequest is one open merge request.
type MergeRequest struct {
	IID   int    `json:"iid"`
	Title string `json:"title"`
	// SHA is the head of the source branch. Files are read at this commit
	// rather than by branch name, so that a branch pushed to while the list is
	// open cannot hand back a file from a different change.
	SHA          string `json:"sha"`
	SourceBranch string `json:"source_branch"`
}

// Snippet is a saved fragment, either the project's or the user's own.
type Snippet struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	FileName string `json:"file_name"`
	// ProjectID is zero for a personal snippet, which lives at a different
	// URL from a project's.
	ProjectID int `json:"-"`
}

// Client talks to one GitLab instance.
type Client struct {
	base  string
	token string
	http  *http.Client
}

// New builds a client for a base URL such as "https://gitlab.com".
func New(baseURL, token string) *Client {
	return &Client{
		base:  strings.TrimRight(baseURL, "/") + "/api/v4",
		token: token,
		http:  &http.Client{Timeout: timeout},
	}
}

// Project resolves a "group/project" path.
func (c *Client) Project(ctx context.Context, path string) (Project, error) {
	var p Project
	err := c.getJSON(ctx, "/projects/"+url.PathEscape(path), &p)
	return p, err
}

// MergeRequests lists the open merge requests, most recently updated first.
func (c *Client) MergeRequests(ctx context.Context, projectID int) ([]MergeRequest, error) {
	var out []MergeRequest
	path := fmt.Sprintf("/projects/%d/merge_requests?state=opened&order_by=updated_at&per_page=%d",
		projectID, listLimit)
	err := c.getJSON(ctx, path, &out)
	return out, err
}

// listLimit caps every listing. A merge request past the hundredth is not one
// anybody is scrolling a dialog to reach.
const listLimit = 100

// diffPageSize is how many diffs are asked for at a time.
//
// Small because it has to be. A live GitLab 17.2 answers 500 to per_page of
// 50 or more on the diffs endpoint — not 400, and not a shortened list, so one
// oversized request loses the entire merge request rather than its tail.
// Measured against that instance: 30 succeeded and 50 did not, and this sits
// under both. It is also the endpoint's own documented default.
const diffPageSize = 20

// SQLFiles lists the .sql files a merge request touches and still has.
func (c *Client) SQLFiles(ctx context.Context, projectID, iid int) ([]string, error) {
	var files []string

	for page := 1; len(files) < listLimit; page++ {
		var diffs []struct {
			NewPath string `json:"new_path"`
			Deleted bool   `json:"deleted_file"`
		}
		path := fmt.Sprintf("/projects/%d/merge_requests/%d/diffs?per_page=%d&page=%d",
			projectID, iid, diffPageSize, page)
		if err := c.getJSON(ctx, path, &diffs); err != nil {
			return nil, err
		}

		for _, d := range diffs {
			// A deleted file has a path and no content. Offering it means
			// offering a row that can only fail.
			if d.Deleted || !strings.EqualFold(pathExt(d.NewPath), ".sql") {
				continue
			}
			files = append(files, d.NewPath)
		}

		// A short page is the last one. Asking again would cost a round trip
		// to be told the same thing.
		if len(diffs) < diffPageSize {
			break
		}
	}
	return files, nil
}

// File reads a file at a ref.
func (c *Client) File(ctx context.Context, projectID int, ref, path string) (string, error) {
	at := fmt.Sprintf("/projects/%d/repository/files/%s/raw?ref=%s",
		projectID, url.PathEscape(path), url.QueryEscape(ref))
	return c.getText(ctx, at)
}

// Snippets lists the project's snippets and the user's own.
//
// Both, because a query worth keeping is as often personal as shared, and
// which of the two a given team uses is not something this can guess.
func (c *Client) Snippets(ctx context.Context, projectID int) ([]Snippet, error) {
	var own []Snippet
	if err := c.getJSON(ctx, fmt.Sprintf("/projects/%d/snippets?per_page=%d", projectID, listLimit), &own); err != nil {
		return nil, err
	}
	for i := range own {
		own[i].ProjectID = projectID
	}

	var personal []Snippet
	if err := c.getJSON(ctx, fmt.Sprintf("/snippets?per_page=%d", listLimit), &personal); err != nil {
		return nil, err
	}
	return append(own, personal...), nil
}

// SnippetContent reads a snippet's text.
func (c *Client) SnippetContent(ctx context.Context, s Snippet) (string, error) {
	if s.ProjectID == 0 {
		return c.getText(ctx, fmt.Sprintf("/snippets/%d/raw", s.ID))
	}
	return c.getText(ctx, fmt.Sprintf("/projects/%d/snippets/%d/raw", s.ProjectID, s.ID))
}

func (c *Client) getJSON(ctx context.Context, path string, into any) error {
	body, err := c.get(ctx, path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, into); err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	return nil
}

func (c *Client) getText(ctx context.Context, path string) (string, error) {
	body, err := c.get(ctx, path)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, statusError(resp.StatusCode, path)
	}

	// One byte past the limit is read so that a file exactly at it is still
	// allowed while anything larger is caught rather than silently halved.
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(body) > MaxFileSize {
		return nil, fmt.Errorf("%s is too large to open (over %d MB)", path, MaxFileSize>>20)
	}
	return body, nil
}

// ErrUnauthorized is what an expired or under-scoped token looks like.
//
// Where the token came from is not this package's business, so the advice for
// renewing it is left to whoever supplied one.
var ErrUnauthorized = errors.New("GitLab refused the token — it may have expired, or lack the api scope")

func statusError(status int, path string) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrUnauthorized
	case http.StatusNotFound:
		return fmt.Errorf("GitLab has no %s", strings.TrimPrefix(path, "/"))
	default:
		return fmt.Errorf("GitLab answered %d for %s", status, strings.TrimPrefix(path, "/"))
	}
}

// ProjectFromRemote reads the host and project path out of a git remote.
//
// Both spellings, because which one a checkout uses is a matter of how it was
// cloned and says nothing about how it should be reached:
//
//	git@host:group/project.git
//	ssh://git@host:2222/group/project.git
//	https://host/group/project.git
func ProjectFromRemote(remote string) (host, path string, ok bool) {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return "", "", false
	}

	if strings.Contains(remote, "://") {
		u, err := url.Parse(remote)
		if err != nil || u.Hostname() == "" {
			return "", "", false
		}
		path, ok = cleanProjectPath(u.Path)
		if !ok {
			return "", "", false
		}
		return u.Hostname(), path, true
	}

	// The scp-like form, which is not a URL and cannot be parsed as one: the
	// text after the colon is a path, not a port.
	at := strings.Index(remote, "@")
	colon := strings.Index(remote, ":")
	if colon < 0 || colon < at {
		return "", "", false
	}
	host = remote[at+1 : colon]
	path, ok = cleanProjectPath(remote[colon+1:])
	if host == "" || !ok {
		// Nothing half-parsed: a caller given a host but no project would go
		// on to build a URL out of the half it got.
		return "", "", false
	}
	return host, path, true
}

func cleanProjectPath(path string) (string, bool) {
	path = strings.Trim(path, "/")
	path = strings.TrimSuffix(path, ".git")
	if path == "" || !strings.Contains(path, "/") {
		return "", false
	}
	return path, true
}

// pathExt is filepath.Ext for a URL path, which always uses forward slashes
// whatever the running machine does.
func pathExt(path string) string {
	slash := strings.LastIndex(path, "/")
	dot := strings.LastIndex(path, ".")
	if dot <= slash {
		return ""
	}
	return path[dot:]
}
