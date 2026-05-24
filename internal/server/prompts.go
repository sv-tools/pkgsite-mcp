package server

import (
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// promptTemplates holds the prompt bodies, kept as Markdown templates under
// docs/prompts so the prose can be edited without touching Go. Parsing at
// startup surfaces a malformed template immediately.
var promptTemplates = template.Must(template.ParseFS(docsFS, "docs/prompts/*.md"))

// renderPrompt executes the named prompt template with data and wraps the result
// as a single user-role message.
func renderPrompt(name string, data any) (*mcp.GetPromptResult, error) {
	var b strings.Builder
	if err := promptTemplates.ExecuteTemplate(&b, name, data); err != nil {
		return nil, fmt.Errorf("rendering prompt %s: %w", name, err)
	}
	return promptResult(strings.TrimSpace(b.String())), nil
}

// registerPrompts adds guided, multi-tool workflows that an assistant can offer
// as ready-made entry points.
func (s *Server) registerPrompts(m *mcp.Server) {
	m.AddPrompt(&mcp.Prompt{
		Name:        "audit_module",
		Title:       "Audit a Go module",
		Description: "Check a module for known vulnerabilities and whether it is up to date.",
		Arguments: []*mcp.PromptArgument{
			{Name: "module", Description: "module path, e.g. github.com/google/uuid", Required: true},
			{Name: "version", Description: "module version to audit; defaults to the latest"},
		},
	}, auditModulePrompt)

	m.AddPrompt(&mcp.Prompt{
		Name:        "audit_project",
		Title:       "Audit a Go project",
		Description: "Scan a Go project and all of its dependencies for known vulnerabilities.",
		Arguments: []*mcp.PromptArgument{
			{Name: "path", Description: "path to the project directory; defaults to the current directory"},
		},
	}, auditProjectPrompt)

	m.AddPrompt(&mcp.Prompt{
		Name:        "find_package",
		Title:       "Find a Go package",
		Description: "Search for packages that meet a need and recommend one.",
		Arguments: []*mcp.PromptArgument{
			{Name: "need", Description: `what the package should do, e.g. "parse YAML"`, Required: true},
		},
	}, findPackagePrompt)
}

func auditModulePrompt(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	module := req.Params.Arguments["module"]
	if module == "" {
		return nil, fmt.Errorf(`the "module" argument is required`)
	}
	return renderPrompt("audit_module.md", map[string]string{
		"Module":  module,
		"Version": req.Params.Arguments["version"],
	})
}

func auditProjectPrompt(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return renderPrompt("audit_project.md", map[string]string{
		"Path": req.Params.Arguments["path"],
	})
}

func findPackagePrompt(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	need := req.Params.Arguments["need"]
	if need == "" {
		return nil, fmt.Errorf(`the "need" argument is required`)
	}
	return renderPrompt("find_package.md", map[string]string{"Need": need})
}

// promptResult wraps prompt text as a single user-role message.
func promptResult(text string) *mcp.GetPromptResult {
	return &mcp.GetPromptResult{
		Messages: []*mcp.PromptMessage{
			{Role: "user", Content: &mcp.TextContent{Text: text}},
		},
	}
}
