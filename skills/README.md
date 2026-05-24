# Agent skills

Portable [agent skills](https://agentskills.io) (the `SKILL.md` format) that wrap this
server's tools into ready-made workflows. Unlike the MCP **prompts** — which only Claude
Code surfaces — these `SKILL.md` files work in any agent that implements the open skills
standard, including **OpenAI Codex** and **Claude**, because skills are discovered from the
filesystem rather than delivered over the MCP protocol.

| Skill              | Mirrors prompt  | Does                                                                              |
|--------------------|-----------------|-----------------------------------------------------------------------------------|
| `audit-go-project` | `audit_project` | Scan a project's deps for vulnerabilities, unmaintained modules, and unused ones. |
| `audit-go-module`  | `audit_module`  | Check one module for vulnerabilities and whether it is up to date.                |
| `find-go-package`  | `find_package`  | Find and recommend a package for a need.                                          |

## Prerequisite

These skills call this server's MCP **tools**, so register `pkgsite-mcp` with your agent first.

- **Codex:**

  ```sh
  codex mcp add pkgsite -- pkgsite-mcp
  ```

- **Claude Code:**

  ```sh
  claude mcp add pkgsite --scope=user -- pkgsite-mcp
  ```

## Install

Use the bundled installer to copy the skills into a directory your agent scans
(default `.agents/skills`):

```sh
pkgsite-mcp install-skills ~/.agents/skills   # omit the path for ./.agents/skills
```

It writes each skill as `<name>/SKILL.md` and skips files that already exist (pass
`-force` to overwrite). Skill locations agents scan:

- **Codex:** `.agents/skills/` (per repo) or `$HOME/.agents/skills/` (personal).
- **Claude:** your configured skills directory.

Or copy a skill directory yourself:

```sh
cp -r skills/audit-go-project ~/.agents/skills/
```

## Invoke

- **Explicit:** `$audit-go-project` (Codex), or your agent's skill-invocation syntax.
- **Implicit:** the agent activates a skill automatically when your request matches its
  `description` — no command needed.

## Optional: Codex UI metadata

Codex skills may add an `agents/openai.yaml` for a display name, icon, and to declare the
MCP tool dependency. It is optional and Codex-specific; see OpenAI's
[skill format reference](https://developers.openai.com/codex/skills). The `SKILL.md` files
here are kept tool-agnostic so they stay portable across agents.
