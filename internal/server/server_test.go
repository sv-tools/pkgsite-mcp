package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sv-tools/pkgsite-mcp/internal/pkgsite"
)

// connect starts our MCP server backed by a stubbed pkg.go.dev API and returns
// a connected client session.
func connect(t *testing.T, handler http.HandlerFunc) *mcp.ClientSession {
	t.Helper()
	api := httptest.NewServer(handler)
	t.Cleanup(api.Close)

	client, err := pkgsite.New(api.URL)
	if err != nil {
		t.Fatalf("pkgsite.New: %v", err)
	}
	srv := New(client, "pkgsite-mcp-test", "test")

	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func TestListToolsRegistersAll(t *testing.T) {
	cs := connect(t, func(w http.ResponseWriter, r *http.Request) {})

	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	got := make(map[string]bool, len(res.Tools))
	for _, tool := range res.Tools {
		got[tool.Name] = true
		if tool.InputSchema == nil {
			t.Errorf("tool %q has nil input schema", tool.Name)
		}
	}
	want := []string{
		"search", "get_package", "get_package_symbols", "get_imported_by",
		"get_module", "list_module_versions", "list_module_packages", "get_vulnerabilities",
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("tool %q not registered", name)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d tools, want %d: %v", len(got), len(want), got)
	}
}

func TestSearchToolReturnsStructuredContent(t *testing.T) {
	const body = `{"items":[{"packagePath":"github.com/google/uuid","synopsis":"UUIDs"}],"total":1}`
	cs := connect(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/search" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if q := r.URL.Query().Get("q"); q != "uuid" {
			t.Errorf("q = %q, want uuid", q)
		}
		_, _ = w.Write([]byte(body))
	})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "search",
		Arguments: map[string]any{"query": "uuid"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error result: %+v", res.Content)
	}
	sc, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("StructuredContent type = %T, want map", res.StructuredContent)
	}
	if total, _ := sc["total"].(float64); total != 1 {
		t.Errorf("total = %v, want 1", sc["total"])
	}
}

func TestToolPropagatesAPIError(t *testing.T) {
	const body = `{"code":404,"message":"package not found"}`
	cs := connect(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(body))
	})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_package",
		Arguments: map[string]any{"path": "does/not/exist"},
	})
	if err != nil {
		t.Fatalf("CallTool returned protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError result for API 404")
	}
	text := contentText(res)
	if !strings.Contains(text, "package not found") {
		t.Errorf("error text = %q, want it to mention 'package not found'", text)
	}
}

func TestToolValidatesRequiredArgs(t *testing.T) {
	cs := connect(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("API should not be called when required args are missing")
	})

	// "search" requires "query"; omit it. The SDK validates input against the
	// inferred schema and returns an IsError result rather than a protocol error.
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "search",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool returned protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError result for missing required argument")
	}
}

func contentText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}
