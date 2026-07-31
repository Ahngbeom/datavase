package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The project a session is looking at is nearly always the one the attached
// worktree came from, so the remote has to be read rather than asked for.
func TestTheProjectIsReadFromWhicheverFormTheRemoteTakes(t *testing.T) {
	for _, tc := range []struct {
		remote   string
		wantHost string
		wantPath string
		wantOK   bool
	}{
		{"git@gitlab.com:group/project.git", "gitlab.com", "group/project", true},
		{"git@gitlab.example.com:group/sub/project.git", "gitlab.example.com", "group/sub/project", true},
		{"https://gitlab.com/group/project.git", "gitlab.com", "group/project", true},
		{"https://user@gitlab.com/group/project", "gitlab.com", "group/project", true},
		{"ssh://git@gitlab.com:2222/group/project.git", "gitlab.com", "group/project", true},
		{"", "", "", false},
		{"/some/local/path", "", "", false},
		{"git@gitlab.com:", "", "", false},
	} {
		host, path, ok := ProjectFromRemote(tc.remote)
		if ok != tc.wantOK || host != tc.wantHost || path != tc.wantPath {
			t.Errorf("ProjectFromRemote(%q) = %q, %q, %v; want %q, %q, %v",
				tc.remote, host, path, ok, tc.wantHost, tc.wantPath, tc.wantOK)
		}
	}
}

// A project is named by a path with a slash in it, and a slash that reaches
// the server unescaped is a different URL entirely.
func TestTheProjectPathIsEscapedIntoOneSegment(t *testing.T) {
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.EscapedPath()
		w.Write([]byte(`{"id":7,"path_with_namespace":"group/project"}`))
	}))
	defer server.Close()

	project, err := New(server.URL, "tok").Project(context.Background(), "group/sub/project")
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	if want := "/api/v4/projects/group%2Fsub%2Fproject"; got != want {
		t.Errorf("requested %q, want %q", got, want)
	}
	if project.ID != 7 {
		t.Errorf("ID = %d, want 7", project.ID)
	}
}

func TestTheTokenIsSentOnEveryRequest(t *testing.T) {
	var header string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Get("PRIVATE-TOKEN")
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	if _, err := New(server.URL, "s3cret").MergeRequests(context.Background(), 1); err != nil {
		t.Fatalf("MergeRequests() error = %v", err)
	}
	if header != "s3cret" {
		t.Errorf("PRIVATE-TOKEN = %q, want the configured token", header)
	}
}

// This is a SQL client. A merge request touching thirty files has at most a
// couple worth opening here, and a deleted one cannot be opened at all.
func TestOnlyTheSQLFilesStillPresentAreOffered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[
			{"new_path":"db/001_init.sql","deleted_file":false},
			{"new_path":"README.md","deleted_file":false},
			{"new_path":"db/old.sql","deleted_file":true},
			{"new_path":"db/002_index.SQL","deleted_file":false}
		]`))
	}))
	defer server.Close()

	files, err := New(server.URL, "tok").SQLFiles(context.Background(), 1, 42)
	if err != nil {
		t.Fatalf("SQLFiles() error = %v", err)
	}

	want := []string{"db/001_init.sql", "db/002_index.SQL"}
	if len(files) != len(want) {
		t.Fatalf("SQLFiles() = %v, want %v", files, want)
	}
	for i := range want {
		if files[i] != want[i] {
			t.Errorf("SQLFiles()[%d] = %q, want %q", i, files[i], want[i])
		}
	}
}

// A merge request's diffs are asked for a page at a time, and the page must
// stay small.
//
// This is not a preference. A real GitLab 17.2 answers 500 — not 400, not a
// truncated list — to per_page=50 or more on this endpoint, so a single large
// request loses the whole merge request rather than the tail of it. Measured
// against a live instance: 30 succeeded, 50 did not.
func TestDiffsArePagedInSmallEnoughPagesToSurvive(t *testing.T) {
	var asked []string
	pages := [][]string{
		make([]string, diffPageSize), // a full page, so another is fetched
		{"db/late.sql"},              // a short page, which ends it
	}
	for i := range pages[0] {
		pages[0][i] = fmt.Sprintf("db/%03d.sql", i)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Query().Get("per_page"))

		page := 1
		if p := r.URL.Query().Get("page"); p != "" {
			fmt.Sscanf(p, "%d", &page)
		}
		var body []string
		if page <= len(pages) {
			for _, path := range pages[page-1] {
				body = append(body, fmt.Sprintf(`{"new_path":%q,"deleted_file":false}`, path))
			}
		}
		fmt.Fprintf(w, "[%s]", strings.Join(body, ","))
	}))
	defer server.Close()

	files, err := New(server.URL, "tok").SQLFiles(context.Background(), 1, 42)
	if err != nil {
		t.Fatalf("SQLFiles() error = %v", err)
	}

	if want := diffPageSize + 1; len(files) != want {
		t.Errorf("SQLFiles() returned %d files, want %d — the second page was dropped", len(files), want)
	}
	for _, per := range asked {
		if per != fmt.Sprint(diffPageSize) {
			t.Errorf("asked for per_page=%s; anything past %d makes GitLab answer 500", per, diffPageSize)
		}
	}
}

// A merge request nobody should be scrolling still has to stop somewhere.
func TestPagingStopsRatherThanFollowingAMergeRequestForever(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always a full page: without a cap this never ends.
		var body []string
		for i := 0; i < diffPageSize; i++ {
			body = append(body, fmt.Sprintf(`{"new_path":"db/%d.sql","deleted_file":false}`, i))
		}
		fmt.Fprintf(w, "[%s]", strings.Join(body, ","))
	}))
	defer server.Close()

	files, err := New(server.URL, "tok").SQLFiles(context.Background(), 1, 42)
	if err != nil {
		t.Fatalf("SQLFiles() error = %v", err)
	}
	if len(files) > listLimit {
		t.Errorf("SQLFiles() returned %d files, want no more than %d", len(files), listLimit)
	}
}

func TestAFileIsFetchedRawAtTheGivenRef(t *testing.T) {
	var path, ref string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, ref = r.URL.EscapedPath(), r.URL.Query().Get("ref")
		w.Write([]byte("SELECT 1;\n"))
	}))
	defer server.Close()

	text, err := New(server.URL, "tok").File(context.Background(), 3, "abc123", "db/001_init.sql")
	if err != nil {
		t.Fatalf("File() error = %v", err)
	}
	if text != "SELECT 1;\n" {
		t.Errorf("File() = %q", text)
	}
	if want := "/api/v4/projects/3/repository/files/db%2F001_init.sql/raw"; path != want {
		t.Errorf("requested %q, want %q", path, want)
	}
	if ref != "abc123" {
		t.Errorf("ref = %q, want abc123", ref)
	}
}

// A token that has expired is the most likely failure by far, and "401" on
// its own sends people to look at the wrong thing.
func TestAnUnauthorisedReplySaysItIsTheToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	_, err := New(server.URL, "stale").MergeRequests(context.Background(), 1)
	if err == nil {
		t.Fatal("an unauthorised reply produced no error")
	}
	if !strings.Contains(err.Error(), "token") {
		t.Errorf("error = %q, want it to name the token", err)
	}
}

// The editor holds the whole file in memory and has no way to page one.
func TestAFileTooLargeToEditIsRefusedRatherThanLoaded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("x", MaxFileSize+1)))
	}))
	defer server.Close()

	_, err := New(server.URL, "tok").File(context.Background(), 1, "main", "huge.sql")
	if err == nil {
		t.Fatal("an oversized file was loaded")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error = %q, want it to say the file is too large", err)
	}
}

// Personal snippets and project snippets are fetched from different places and
// have to come back able to say where they live, or the raw fetch goes to the
// wrong URL.
func TestSnippetsRememberWhetherTheyBelongToTheProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/api/v4/projects/9/snippets":
			w.Write([]byte(`[{"id":1,"title":"cleanup","file_name":"a.sql"}]`))
		case "/api/v4/snippets":
			w.Write([]byte(`[{"id":2,"title":"mine","file_name":"b.sql"}]`))
		case "/api/v4/projects/9/snippets/1/raw":
			w.Write([]byte("-- project\n"))
		case "/api/v4/snippets/2/raw":
			w.Write([]byte("-- personal\n"))
		default:
			t.Errorf("unexpected request to %q", r.URL.EscapedPath())
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	c := New(server.URL, "tok")
	snippets, err := c.Snippets(context.Background(), 9)
	if err != nil {
		t.Fatalf("Snippets() error = %v", err)
	}
	if len(snippets) != 2 {
		t.Fatalf("Snippets() returned %d, want the project's and the personal one", len(snippets))
	}

	for _, s := range snippets {
		text, err := c.SnippetContent(context.Background(), s)
		if err != nil {
			t.Fatalf("SnippetContent(%q) error = %v", s.Title, err)
		}
		if !strings.HasPrefix(text, "-- ") {
			t.Errorf("SnippetContent(%q) = %q", s.Title, text)
		}
	}
}
