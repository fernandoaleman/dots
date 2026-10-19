# Coding agents

Claude Code today, Codex (through opencode or pi) later: what dots sets up
so instructions, skills, sessions and memory are the same on every
machine. The private content (instructions, skills, sessions) never goes
in this public repo; dots only creates the files and links that point at
it. Work in progress: the cross-machine plan and its next steps are in
[TODO.md](../TODO.md) ("Claude Code: memory and sessions across machines").

## Global instructions: one `~/.agents/AGENTS.md`

Decided 2026-10-19. One real file, `~/.agents/AGENTS.md`, with `team.md`
and `personas.md` beside it in `~/.agents`, is the global instructions for
every agent. It started as the Mac Studio's 69 KB `~/.claude/CLAUDE.md`,
with its two `@team.md`/`@personas.md` import lines moved out and
`~/code` paths changed to `~/Work`. Each agent reaches it the way it
supports (checked in each tool's docs, 2026-10-19):

| Agent | Reads | dots creates |
|---|---|---|
| Claude Code | `~/.claude/CLAUDE.md` (it reads `AGENTS.md` only in projects, not from home) | a 3-line `~/.claude/CLAUDE.md`: `@~/.agents/AGENTS.md`, `@~/.agents/team.md`, `@~/.agents/personas.md` |
| Codex | `~/.codex/AGENTS.md` (`AGENTS.override.md` first) | link to `../.agents/AGENTS.md` |
| opencode | `~/.config/opencode/AGENTS.md`, else `~/.claude/CLAUDE.md` | link to `../../.agents/AGENTS.md` |
| pi | `~/.pi/agent/AGENTS.md` (or `CLAUDE.md`) | link to `../../.agents/AGENTS.md` |

Plus links `~/.claude/team.md` and `~/.claude/personas.md` into
`~/.agents`, for skills that read them there (`request-pr-review`).

- **Import for Claude, links for the rest.** Claude's docs (*How Claude
  remembers your project*, "Share one file with other coding tools") offer
  either, but its Edit and Write tools refuse to write through a symlink,
  and an import is how its user file can pull in `team.md`/`personas.md`.
  Codex, opencode and pi don't expand `@` imports, so the import lines live
  only in `CLAUDE.md` (the others don't get the team files).
- **Codex reads the whole file through the link.** Its docs give a
  32 KiB `project_doc_max_bytes` limit for the instruction chain, but
  `codex debug prompt-input` (0.160.0) showed every section of the 69 KB
  user file, the last one included.
- **Verified on Omarchy:** a fresh `claude -p` session saw the file's
  sections, its `~/Work/notes` path and the team roster.
- **`install.sh`** (`setup_agent_instructions`, both OSes) creates only
  what's missing, and leaves a real file in the way alone with a warning.
  The links dangle until `~/.agents` arrives on a new machine (Syncthing,
  below).
- **Later:** slim the file (Claude's docs suggest under 200 lines; it's
  1,046) by moving reference sections (Jira CLI, AWS, PagerTree, Slack,
  Cloudflare, Codex review threads) into skills.

## Skills: `~/.agents/skills`

Personal skills live in `~/.agents/skills/<name>`, which Codex reads
(OpenAI's *Build skills*: `$HOME/.agents/skills`, symlinks followed) and
opencode reads, each linked from `~/.claude/skills/<name>` for Claude Code.
Omarchy's own skills (`omarchy`, `diagnose-crash`) and `todoist-cli`
(`install.sh`, `td skill install`) are left as they install themselves.
See [desktop.md](desktop.md) for the Todoist skill.

## Sync between machines: Syncthing

Decided 2026-10-19. Two Syncthing folders, set up by `install.sh`
(`setup_syncthing_agents`, both OSes, after the Obsidian vaults) and
paired through Omarchy like the vaults
([packages.md](packages.md#syncthing-obsidian-vaults-across-machines)):

| Folder ID | Path | Kept local (`.stignore`) | Versioning on Omarchy |
|---|---|---|---|
| `claude-projects` | `~/.claude/projects` (sessions, memory) | `/-*`: Claude's old path-named project folders (and any made by a launcher that bypasses the `claude` wrapper) | trash can, 30 days, in `~/.local/state/syncthing-versions/claude-projects` |
| `agents` | `~/.agents` (instructions, team files, skills) | `/skills/omarchy`, `/skills/diagnose-crash` (Omarchy's links into `/usr/share/omarchy`), `/skills/todoist-cli` (`td skill install` on each machine) | staggered, 365 days, like the vaults |

- **Every machine, both profiles:** most sessions are work ones, so the
  work MacBook gets them too (user's call, 2026-10-19).
- **Trash can for sessions, not staggered.** Session files only grow (one
  here is 57 MB) and change on every turn; staggered keeps up to ~200 old
  copies of each. Trash can keeps one previous copy per file
  (docs.syncthing.net, *File Versioning*). Its folder is outside
  `~/.claude/projects` so Claude never lists those copies; Syncthing wants
  it on the same filesystem.
- **Versions only on Omarchy, the hub.** Syncthing versions only changes
  that come from another device, so the hub keeps what the Macs change; the
  `~/claude-migration` backups stay as a second copy.
- **Rule: one machine per session at a time.** Two machines writing the
  same session make a `*.sync-conflict-*.jsonl` copy
  ([troubleshooting](../guides/troubleshooting.md#claude-code-an-extra-session-or-a-sync-conflict-file)).
- **Claude's 30-day cleanup** (`cleanupPeriodDays`) now runs everywhere at
  once: whichever machine deletes old sessions first deletes them on all.
  Same history as one machine; raise the setting to keep more.
- **A machine joins only after its own migration** (`claude-migrate-projects
  --apply` and the `~/.claude/.claude.json` switch), or its new sessions
  land in ignored `-*` folders and don't sync.
