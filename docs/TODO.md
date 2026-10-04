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

Things to settle then: the JSON format (Raycast's export vs. the plugin's),
where the JSON lives (dots if nothing private, else 1Password), and the
shared keybindings.

### Remote desktop from the Mac into Omarchy (over Tailscale)

See and use the full Omarchy desktop (GUI, not just SSH) from the Mac
laptop when traveling or at the office, over Tailscale. Why: one machine
holds the AI tools' state (Claude Code, opencode memories, sessions), so
nothing has to be synced across machines or OSes.

Known to be possible (Hyprland supports VNC servers such as `wayvnc`; macOS
has a built-in VNC client; the Omarchy plugin catalog has `io.github.rsd.omavnc`).
To research then: Omarchy's own way first, which server and client, which
of the three monitors to share (or a headless output), sharing while the
desktop is locked, security (Tailscale-only listening, auth), and latency.

### tmux-ssh project

- README: the install command uses `…/tmux-ssh/main/tmux-ssh`, which is a
  404 (the branch is `master`).
- Tag releases, so dots can pin a version instead of a commit
  (`TMUX_SSH_COMMIT` in `install.sh`).
- Investigate whether tmux-ssh works inside herdr.

### 1Password naming cleanup

Retitle items to `<Service> (<account>)` (e.g. `Cloudflare (1000Bulbs)`,
`Atlassian (Personal)`), as started in the secrets section
([secrets.md](decisions/secrets.md)): the `AWS …`, `GitHub - … - …` and
other mixed styles.

### Audit 1Password for Rackspace leftovers

The Rackspace AWS account was retired (2025). Archive every 1Password item
related to it (logins, keys, notes) across vaults.

### AWS VPN Omarchy plugin

Install the user's own AWS VPN client plugin (outside dots; see
[desktop.md](decisions/desktop.md)) and review it for submission to the
Omarchy plugin catalog.

### Mac phase: Mac Studio sync scripts

`push-to-mac-studio` / `pull-from-mac-studio` were dropped on Omarchy
(remote desktop replaces syncing). Decide whether the Macs still need them
once remote desktop into Omarchy works ([bin.md](decisions/bin.md)).

### Retire the old repos (Mac phase)

Archive `fernandoaleman/nvim` once the Mac uses dots.

## When it happens

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
