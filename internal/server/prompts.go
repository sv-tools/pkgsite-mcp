package server

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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
	target, atVersion := module, ""
	if v := req.Params.Arguments["version"]; v != "" {
		target = module + "@" + v
		atVersion = " at version " + v
	}
	text := fmt.Sprintf(`Audit the Go module %s for supply-chain risk:
1. Call get_vulnerabilities for %q%s to list known advisories from the Go vulnerability database.
2. Call get_module and list_module_versions to check whether that version is the latest and when it was released.
3. For each vulnerability, report its ID, summary, and the version that fixes it.
Then state whether the module is safe to use and recommend an upgrade if one is warranted.`, target, module, atVersion)
	return promptResult(text), nil
}

func findPackagePrompt(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	need := req.Params.Arguments["need"]
	if need == "" {
		return nil, fmt.Errorf(`the "need" argument is required`)
	}
	text := fmt.Sprintf(`Find a Go package that can %s:
1. Call search with relevant keywords.
2. For the most promising results, call get_package to compare synopsis, latest version, and license.
3. Optionally call get_imported_by to gauge real-world adoption.
Recommend one package and explain why, giving its import path and a short usage note.`, need)
	return promptResult(text), nil
}

// promptResult wraps prompt text as a single user-role message.
func promptResult(text string) *mcp.GetPromptResult {
	return &mcp.GetPromptResult{
		Messages: []*mcp.PromptMessage{
			{Role: "user", Content: &mcp.TextContent{Text: text}},
		},
	}
}
