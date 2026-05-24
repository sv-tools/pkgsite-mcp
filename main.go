// Command pkgsite-mcp is a Model Context Protocol (MCP) server that exposes the
// pkg.go.dev API as tools, letting an MCP client search for Go packages and
// inspect modules, packages, symbols, importers, and vulnerabilities.
//
// It speaks MCP over stdio, which is the transport used by Claude Code, Claude
// Desktop, and most IDE integrations.
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sv-tools/pkgsite-mcp/internal/pkgsite"
	"github.com/sv-tools/pkgsite-mcp/internal/server"
)

const serverName = "pkgsite-mcp"

// skillsFS holds the bundled agent skills, installed by the install-skills
// subcommand. Embedding them lets a `go install`-ed binary carry the skills.
//
//go:embed skills
var skillsFS embed.FS

// defaultSkillsDir is where install-skills writes when no directory is given. It
// is the conventional per-repo agent-skills location.
const defaultSkillsDir = ".agents/skills"

// version is overridable at build time via -ldflags "-X main.version=...".
// When unset, it is resolved from the binary's embedded build info, so installs
// via `go install <module>@<version>` report the correct module version.
var version string

func main() {
	log.SetFlags(0)
	log.SetPrefix(serverName + ": ")

	if len(os.Args) > 1 && os.Args[1] == "install-skills" {
		if err := installSkills(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		return
	}

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

// cacheMaxEntries and cacheMaxBytes bound the response cache by entry count and
// total buffered body bytes, respectively, so large doc/README/license payloads
// cannot grow it without limit.
const (
	cacheMaxEntries = 256
	cacheMaxBytes   = 64 << 20 // 64 MiB
)

func run(serverURL string, timeout time.Duration, retries int, cacheTTL time.Duration) error {
	client, err := pkgsite.New(serverURL,
		pkgsite.WithHTTPClient(&http.Client{Timeout: timeout}),
		pkgsite.WithUserAgent(fmt.Sprintf("%s/%s", serverName, version)),
		pkgsite.WithRetry(retries, 0),
		pkgsite.WithCache(cacheTTL, cacheMaxEntries, cacheMaxBytes),
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

// installSkills copies the bundled agent skills into a target directory
// (default defaultSkillsDir), preserving each skill's <name>/SKILL.md layout so
// agents like Codex can discover them. Existing files are left untouched unless
// -force is given. The top-level skills/README.md is not a skill, so it is skipped.
func installSkills(args []string) error {
	flags := flag.NewFlagSet(serverName+" install-skills", flag.ExitOnError)
	force := flags.Bool("force", false, "overwrite skill files that already exist")
	flags.Usage = func() {
		out := flags.Output()
		fmt.Fprintf(out, "Usage: %s install-skills [-force] [dir]\n\n", serverName)
		fmt.Fprintf(out, "Install the bundled agent skills into dir (default %q),\n", defaultSkillsDir)
		fmt.Fprintln(out, "preserving each skill's <name>/SKILL.md layout.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}

	dir := defaultSkillsDir
	if flags.NArg() > 0 {
		dir = flags.Arg(0)
	}

	var installed, skipped int
	walkErr := fs.WalkDir(skillsFS, "skills", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		// rel is the path under skills/, e.g. "audit-go-project/SKILL.md". A rel
		// with no separator is a top-level file (the README); skip it.
		rel := filepath.FromSlash(strings.TrimPrefix(p, "skills/"))
		if !strings.ContainsRune(rel, filepath.Separator) {
			return nil
		}
		dst := filepath.Join(dir, rel)
		if !*force {
			if _, statErr := os.Stat(dst); statErr == nil {
				skipped++
				log.Printf("skip (exists): %s", dst)
				return nil
			}
		}
		data, err := skillsFS.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
		installed++
		log.Printf("installed: %s", dst)
		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	log.Printf("installed %d skill file(s) into %s (%d skipped)", installed, dir, skipped)
	if skipped > 0 && !*force {
		log.Print("re-run with -force to overwrite skipped files")
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
