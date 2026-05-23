// Command pkgsite-mcp is a Model Context Protocol (MCP) server that exposes the
// pkg.go.dev API as tools, letting an MCP client search for Go packages and
// inspect modules, packages, symbols, importers, and vulnerabilities.
//
// It speaks MCP over stdio, which is the transport used by Claude Code, Claude
// Desktop, and most IDE integrations.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sv-tools/pkgsite-mcp/internal/pkgsite"
	"github.com/sv-tools/pkgsite-mcp/internal/server"
)

const serverName = "pkgsite-mcp"

// version is overridable at build time via -ldflags "-X main.version=...".
// When unset, it is resolved from the binary's embedded build info, so installs
// via `go install <module>@<version>` report the correct module version.
var version string

func main() {
	log.SetFlags(0)
	log.SetPrefix(serverName + ": ")

	serverURL := flag.String("server", pkgsite.DefaultServer, "pkg.go.dev API server URL")
	timeout := flag.Duration("timeout", 30*time.Second, "HTTP request timeout")
	retries := flag.Int("retries", 2, "retry attempts for transient failures (429/5xx)")
	cacheTTL := flag.Duration("cache-ttl", 5*time.Minute, "response cache TTL; 0 disables caching")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	version = resolveVersion()

	if *showVersion {
		fmt.Println(serverName, version)
		return
	}

	if err := run(*serverURL, *timeout, *retries, *cacheTTL); err != nil {
		log.Fatal(err)
	}
}

// cacheMaxEntries bounds the response cache.
const cacheMaxEntries = 256

func run(serverURL string, timeout time.Duration, retries int, cacheTTL time.Duration) error {
	client, err := pkgsite.New(serverURL,
		pkgsite.WithHTTPClient(&http.Client{Timeout: timeout}),
		pkgsite.WithUserAgent(fmt.Sprintf("%s/%s", serverName, version)),
		pkgsite.WithRetry(retries, 0),
		pkgsite.WithCache(cacheTTL, cacheMaxEntries),
	)
	if err != nil {
		return err
	}

	// Stop serving on SIGINT/SIGTERM so the process exits cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mcpServer := server.New(client, serverName, version)
	if err := mcpServer.Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}

// resolveVersion returns the build-time override if set, otherwise the module
// version stamped into the binary by `go install <module>@<version>`. For local
// builds (where the module version is "(devel)") it falls back to a short VCS
// revision, then to "dev".
func resolveVersion() string {
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	var revision string
	var modified bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if revision != "" {
		if len(revision) > 12 {
			revision = revision[:12]
		}
		if modified {
			revision += "-dirty"
		}
		return revision
	}
	return "dev"
}
