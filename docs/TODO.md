# TODO

Ideas and follow-ups to pick up **after** the dots migration is done (or
when their trigger happens). Not decisions yet: when an item is picked up,
its outcome goes in `docs/decisions/` and the item is removed here.

## After dots

### Snippets and bookmarks pickers (Omarchy + Raycast)

Export Raycast's snippets and quicklinks (Mac) to JSON as the single
source, then use them on both machines with the **same keybindings**:

- **Omarchy:** a *picker* (press a key, fuzzy-search, insert/open), not
  keyword expansion: too many keywords to remember; the picker gets used
  anyway.
- **Mac:** Raycast (free tier: local snippets and quicklinks), importing
  the same JSON.

Check the Omarchy plugin catalog (`https://plugins.omarchy.org`) before
building anything. Candidates seen 2026-10-04:

- Snippets: `community.shoxjaxon.snippets`, `io.github.prohner.snippets`,
  `leninzapata.super-clipboard-snippet`, `io.github.ferc10110.readily`,
  `cylon58.paste-layer`, `reclip`.
- Bookmarks: `stefanmara.bookmarks`, `io.github.idr4n.bookmarks`,
  `eudionelima.bookomarchy`, `io.github.decadentsavant.bookmark-everything`,
  `weblauncher`, `io.github.equa-tory.quicksearch` (keyword quicklinks).
- Launchers that bundle both: `io.github.hominluo.launcher`,
  `io.github.terrifiedbug.omacast`.

Raycast is being dropped on the Macs (2026-10-10; Spotlight instead), so
its quicklinks and snippets need a new home. Candidates to weigh then:
**Raindrop.io** for bookmarks (Mac cask `raindropio`; the Omarchy catalog
has a Raindrop Bookmarks picker plugin), and **espanso** for snippets on
Omarchy and the Macs (dropped earlier only because Raycast covered it; the
catalog has an espanso plugin, `io.github.taisau.espanso`), with its
snippet files synced by Syncthing.
Things to settle then: the JSON format (Raycast's export vs. the plugin's),
where the JSON lives (dots if nothing private, else 1Password), and the
shared keybindings.

### Meeting recording on Omarchy (Plaud)

Plaud Desktop (records calls, then transcript and AI notes on the paid
plan) is Mac/Windows only. On Omarchy, in this order:

1. **First: the Plaud device itself** (records the room; works for calls
   played through speakers, not headphones). The user tries this first.
2. Record system audio + mic with PipeWire (OBS or a small `pw-record`
   script in dots), then import the file into Plaud (verify Plaud imports
   audio files).
3. Evaluate the catalog plugin **OmaScribe** (local transcription + AI
   meeting notes), or a DIY Whisper (as Voxtype uses) + Claude Code pipeline
   into Obsidian.

Re-evaluate Plaud at renewal.

### Sunshine: back to Omarchy's installer when fixed

`install.sh` replicates `omarchy-install-service-sunshine` with fixes
because it's broken in Omarchy 4.0.4 ([desktop.md](decisions/desktop.md)).
When Omarchy merges a fix (issues #12053/#14048, PRs #12505/#13230/#13925/
#14412, certificate PRs #12834/#14918), switch back to calling Omarchy's
installer. **Also update Sunshine as soon as Omarchy's repo ships
v2026.914.233613 or newer** (#10836, #11956): the installed 2026.516 has
GHSA-fp6g-27w5-489j (high, local privilege escalation through its file
capabilities). If Omarchy stays behind, reconsider AUR `sunshine-bin`.

### tmux-ssh project

- README: the install command uses `…/tmux-ssh/main/tmux-ssh`, which is a
  404 (the branch is `master`).
- Tag releases, so dots can pin a version instead of a commit
  (`TMUX_SSH_COMMIT` in `install.sh`).
- Investigate whether tmux-ssh works inside herdr.

### 1Password naming cleanup

Retitle items to `<Service> (<account>)` (e.g. `Cloudflare (work)`,
`Atlassian (Personal)`), as started in the secrets section
([secrets.md](decisions/secrets.md)): the `AWS …`, `GitHub - … - …` and
other mixed styles.

### Research: move AWS access to SSO (CLI and console)

Today: CLI via role + MFA with a long-lived IAM key (`aws-role-login`);
console via the Chrome extension *AWS Extend Switch Roles*. Research
whether both can move to SSO (IAM Identity Center), so there is one way to
sign in and no long-lived key to rotate: can the extension (or an
alternative) work with SSO, do the SSO permission sets cover what the role
profiles do, and what changes for `generate-ssh-config`. Needs time and
testing; decide with the ~2026-11-03 SSO review.

### Research: SSH keys as files vs 1Password's SSH agent

Today `install.sh` installs the SSH keys from 1Password as real files in
`~/.ssh` (no prompts when using SSH; the user's original requirement).
Alternative: keep them only in 1Password and use its SSH agent, which
asks for approval per app/terminal until 1Password locks (the pop-ups seen
so far came from the `op` CLI, not from SSH). Compare convenience,
security (no key files on disk), and how it works with `tmux-ssh`,
`generate-ssh-config` and the Mac; research only, no decision yet.

### Audit 1Password for Rackspace leftovers

The Rackspace AWS account was retired (2025). Archive every 1Password item
related to it (logins, keys, notes) across vaults.

### AWS VPN Omarchy plugin

Now installed by `install.sh` from its GitHub repo
([desktop.md](decisions/desktop.md)). Made **public** 2026-10-04 (a fresh
clone needs no GitHub login); before that its history was scrubbed of a
real VPN server/client address from an old README example and the repo
recreated, so the old commits are gone. Review it for submission to the
Omarchy plugin catalog; in the plugin repo:

- `setup` installs `openlawsvpn-daemon` with `yay`; use Omarchy's
  `omarchy-pkg-aur-add` instead (dots works around it by installing the
  daemon first).
- `setup` writes its keybinding into Omarchy's `~/.config/hypr/bindings.lua`;
  dots runs `setup --no-keybind` and binds in `hyprland.dots.lua`.
- The systemd drop-in is no longer needed: the AUR package fixed the
  daemon path in 1.2.4 (setup already only applies it when needed).
- README: `omarchy plugin add` installs into
  `~/.config/omarchy/plugins/<plugin ID>/` (`fernandoaleman.aws-vpn-client`),
  not `…/plugins/omarchy-aws-vpn-client/`; fix the `setup` and CLI-link
  paths shown there.
- FYI, not the plugin's bug: the AUR `openlawsvpn` build prints a Cargo
  "cannot create the lock file … --locked" error; its `prepare()` runs
  `cargo fetch --locked || cargo fetch`, so the fallback succeeds.

### Mac phase: Mac Studio sync scripts

`push-to-mac-studio` / `pull-from-mac-studio` were dropped on Omarchy
(remote desktop replaces syncing). Decide whether the Macs still need them
once remote desktop into Omarchy works ([bin.md](decisions/bin.md)).

### Mac: Tailscale's "Launch at login" checkbox

On the work MacBook the checkbox fails ("Start on Login could not be set")
because macOS has no background-items record of Tailscale's login helper;
an Open at Login entry is the workaround (`install.sh` adds it since
2026-10-12)
([troubleshooting](guides/troubleshooting.md#mac-tailscale-start-on-login-could-not-be-set-popup)).
Look into it later (not urgent): whether a newer Tailscale fixes it, check
`sudo sfltool dumpbtm` for Tailscale's entries, and whether the Mac Studio
shows the same; then drop the workaround if the checkbox works.

### Mac: Voxtype hotkey with Hammerspoon

The Mac records from Voxtype's menu bar (decided 2026-10-11): its own
hotkey only watches keys, so the app in front still gets F9 and beeps
([macos.md](decisions/macos.md)). Explore **Hammerspoon** (cask), which
takes the key for itself like Hyprland does on Omarchy: Voxtype ships
`voxtype setup hammerspoon` (hold or toggle). Check F9 with media keys on
(likely still fn+F9), Hammerspoon's own Accessibility permission, and
whether Hammerspoon could also serve the window manager decision.

### Claude Code: my own skills in a repo or plugin marketplace

Personal skills now live in `~/.agents/skills/<name>` (read by Codex and
opencode), each linked from `~/.claude/skills/<name>` (copied from the Mac
Studio to Omarchy 2026-10-19; the work Mac still has its hand-copied
2026-10-13 set); they'll sync with Syncthing alongside the Claude projects.
`todoist-cli` stays `install.sh`'s (`td skill install`). Later, once
Omarchy has been used for a while (user, 2026-10-19): move them to a
private repo, or a private plugin marketplace like the company's, so every
machine installs the same versions; keep any work-only skills off this
public repo.

### Claude Code: memory and sessions across machines

Wanted: memories and sessions available on Omarchy and both Macs. Facts
(Claude Code docs, *Manage sessions* and *How Claude remembers your
project*): transcripts are `~/.claude/projects/<path>/<id>.jsonl` and auto
memory `~/.claude/projects/<repo path>/memory/`, both "machine-local", named
after the absolute path (`-Users-faleman-Work-x` on a Mac,
`-home-faleman-Work-x` on Omarchy); `claude --resume <id>` and the picker's
`Ctrl+A` find sessions in any project folder; transcripts are deleted after
`cleanupPeriodDays` (30); one session written from two places interleaves.
To decide: what to sync (instructions and skills via git; memory and
sessions via Syncthing?), how to map the Mac and Omarchy paths, conflicts,
keeping personal projects and secrets in transcripts off the work Mac.

Tested 2026-10-18 on Omarchy (v2.1.288, throwaway sessions):
`CLAUDE_CONFIG_DIR=$HOME/.claude` plus `CLAUDE_CODE_PROJECT_DIR_NAME=<name>`
(docs, *Manage sessions*: "Name the project directory yourself"; v2.1.234+)
puts sessions and memory under `projects/<name>/` whatever the path: a
session started in one folder is listed by the `--resume` picker, resumed by
`--continue` and `--resume <id>` from another folder; memory reads and writes
work; a session moved by hand from its old path-named folder resumes; a
session whose folder no longer exists resumes in the current one. Gotcha:
that variable moves `~/.claude.json` (troubleshooting). Symlinking the
path-named folders to one shared folder instead breaks memory (troubleshooting).
Prior art (none fits Syncthing + different homes): `VirelNode/session-roam`
(Syncthing, needs identical paths; cross-machine session locks),
`tawanorg/claude-sync` (S3/R2/WebDAV, rewrites paths inside transcripts),
`NeoAcar/claude-handoff` (export/import one session), anthropics/claude-code
#31992 and #25739 (open feature requests).
Agreed 2026-10-18: names are the repo name (from the git remote; else the
repo's or folder's name; no personal/work prefix, everything syncs
everywhere). Done: `claude-project-name`, the `claude` wrapper (off until
`~/.claude/.claude.json` exists), `claude-migrate-projects` (`--apply` tested on a scratch copy; Mac
Studio mapping reviewed, overrides in `~/.config/dots/claude-projects`;
Omarchy backed up to `~/claude-migration` 2026-10-19).
Omarchy migrated (its own folders, then the Mac Studio backup) and
switched on 2026-10-19; old path-named folders kept. Mac Studio extras
brought over by hand 2026-10-19 (machine-local): its 11 plugins (plus
`gopls`/`lua-language-server`), `settings.json` choices (`autoMode` with
`~/Work` paths, `effortLevel`, `editorMode`, `voiceEnabled`; skipped its
herdr hook and its unused sandbox domains), `team.md`, `personas.md`,
`team-interview-notes.md`, `jira/team_directory.md` (kim-daily's local
override); skipped `tasks/`. Next: Syncthing for `~/.claude/projects`
(ignore `/-*`), shared `AGENTS.md` and `~/.agents/skills`, Mac Studio
cutover. Other launchers (herdr, editors) bypass the bash wrapper.
The global `~/.claude/CLAUDE.md`, `team.md` and `personas.md` were copied
by hand from the Mac Studio to the work Mac (2026-10-13; `CLAUDE.md` points
at `~/Work/notes`, which isn't there); keep them in step (rsync or
Syncthing) once the Mac Studio is rebuilt on dots and every machine uses
`~/Work` paths.

### Work notes repo: how to use it across machines

The private notes repo (work log, `outstanding-work.md`, ticket drafts) is
used on the Mac Studio, and the global Claude `CLAUDE.md` points at it.
Not on the work Mac for now (2026-10-13): the work Mac's GitHub token can't
reach personal repos, and git repos don't go in Syncthing; remote into the
Mac Studio from the office instead. To rethink: where it lives (Omarchy
long term), and how the work Mac reads or adds to it.

### herdr: try it later

Omarchy installs herdr (a terminal multiplexer for coding agents); used a
little on the Mac Studio, not on the work Mac. Revisit whether to use it;
the Studio's Claude Code SessionStart hook (`herdr-agent-state.sh`) and
`herdr` skill come with it.

### Git aliases: retire the muscle-memory ones

Old zsh `50-aliases` git aliases re-added 2026-10-19 in their own section
of `~/.config/bash/aliases` ([shell.md](decisions/shell.md)) because muscle
memory was slowing work: `gs`, `ggpur`. Once Omarchy's (`g`, `gcm`, `gcam`, `gcad`)
are habit, decide which to keep and drop the rest.

### Office: remote desktop slow, fuzzy, freezes when idle

First office day (2026-10-14), work Mac to the Mac Studio (Screen Sharing)
and to Omarchy (Moonlight): both slow (Moonlight a bit more), text a little
fuzzy, and the window froze after a while untouched (close and reconnect).
Office ~500 Mbps, home ~900. Next office day, check before changing
anything: `tailscale status` / `tailscale ping <host>` from the office
(direct or `via DERP`: a relay is likely if the office blocks UDP), with
and without the AWS VPN; then Screen Sharing's quality mode, Moonlight's
bitrate/resolution/codec, and what the idle freeze is (display sleep on
the host, or the connection dropping).

### Teams: native app or web app (ask again)

Installed 2026-10-17 to try, alongside the Teams web app (neither change is
in dots yet): AUR `teams-for-linux-bin`, an unofficial Electron wrapper of
the Teams website. Works so far: stays in the tray when closed, sign-in in
the keyring (`v11`). Notes: its Teams page runs without Chromium's sandbox by
design (`sandbox: false` in its code); it runs under XWayland
(`--ozone-platform=x11` in its launcher, GPU off). A test notification showed
inside the app, not as a desktop popup: Teams sent none (checked with
`dbus-monitor`), so it's a Teams setting; the user is fine with that for
now. To decide: keep it (dots installs it, removes the Teams web app, maybe a
workspace rule) or remove it.

### Outlook PWA stops refreshing

On the work Mac the Outlook PWA showed mail only up to 8 AM while Outlook
on the web had mail through 1 PM; closing and reopening it fixed it (no
sync button in Outlook on the web). Find the cause (Chrome freezing or
discarding the app while in the background? the connection dropping after
VPN or network changes?) and a fix, or try the Outlook app (cask).

### 1Password: Touch ID prompt when remote

Remoted into the Mac Studio, `aws-role-login` showed the 1Password
authorization with Touch ID only, with no way to type the password
(1Password community: a known gap with Touch ID on; workaround so far:
turn Touch ID off). Same likely on any remote session to a machine that
uses the CLI integration (Omarchy too). Find a fix that keeps the
security: e.g. an Apple Watch, the session's password, or doing the
`op` step on the local machine.

### Omarchy issues to report (found 2026-10-14)

Check upstream first (none found 2026-10-14); an issue before any PR.

- **tmux reload duplicates entries:** Omarchy's `tmux.conf` appends with
  `set -ag terminal-overrides ",*:RGB"`, `set -ag terminal-features
  "xterm-kitty:extkeys"`, `set -as terminal-features ",*:clipboard"`, and
  its own `prefix q` re-sources that file, so each reload adds another copy
  (seen: 12 and 36 entries instead of 2 and 6). Harmless, low priority.
  dots' theme-set hook resets them before reloading
  ([desktop.md](decisions/desktop.md)).
- **foot padding keeps the old theme's color after a switch:**
  `omarchy-theme-set-tmux` sets tmux's `window-style` to the new theme's
  background, so tmux paints every cell in it, but foot can't reload its
  colors and draws its padding (`pad=14x14`) in the launch theme's
  background: a visible band until the terminal is reopened. With foot
  `alpha` below 1, explicitly colored cells also stay solid (default
  `alpha-mode`), so only the padding is see-through
  ([troubleshooting](guides/troubleshooting.md#a-band-around-the-inside-edge-of-the-terminal)).

### Retire the old repos (Mac phase)

Archive `fernandoaleman/nvim` once the Mac uses dots.

## When it happens

### Around 2026-10-19/20: keep monitor scale 1.25 or go back to 1.6?

Trying 1.25 on the three desk monitors since 2026-10-18 (more space; is it
too small for the eyes?). The trial is the machine-local
`~/.config/hypr/monitors.local.lua` (same rules, scale 1.25, positions 0 /
2048 / 4096); dots still says 1.6. Ask the user, then either move 1.25 into
`monitors.dots.lua` and `docs/decisions/desktop.md`, or delete the local
file. Either way, Omarchy's menu also set `monitors.lua`'s catch-all to 1.25
with `GDK_SCALE` 1 (was 1.6 and 2): ask whether to restore it.

### Around 2026-11-03: is AWS SSO login still used?

SSO login was kept as a rarely used alternative to the daily role + MFA
login (`.aws/config.sso`, one merged `sso-session`; see
[secrets.md](decisions/secrets.md)). After ~30 days, check how often it
was used (the user expects zero); if unused, drop `.aws/config.sso` and
`aws-sso-login`.

### Work Claude Code plugins: automate or keep manual?

The work plugin marketplace is installed by hand
([setup step 5](setup/omarchy.md#5-work-claude-code-plugins)). The company
is evaluating a different way to distribute them; once decided, revisit
whether `install.sh` should install them (with the names kept out of this
public repo, e.g. a machine-local list or 1Password).

### Ruby 2.7.8 work app on this machine

Set up ruby-lsp through a project-local `.lazy.lua` and finish the parts
of the [Ruby guide](guides/ruby.md) marked "to be verified".
