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

**In progress (2026-10-10):** Omarchy's Sunshine + Moonlight chosen and
added to `install.sh` ([desktop.md](decisions/desktop.md)). Still to decide:
which of the three monitors to stream (or a headless output at the
MacBook's size; catalog plugins: Stream View, Virtual Display), unlocking
the desktop remotely, the Sunshine Admin password from 1Password, Moonlight
keyboard behavior from the Mac (Cmd/Super), and the Tailscale cleanup
(duplicate offline Mac devices).
When Tailscale is set up: name the devices `omarchy` and `mac-studio` and
switch their `HostName`s in `ssh/.ssh/config` to those Tailscale names;
decide whether sshd stays reachable on the LAN or only over Tailscale
([secrets.md](decisions/secrets.md)).

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
([setup step 4](setup/omarchy.md#4-work-claude-code-plugins)). The company
is evaluating a different way to distribute them; once decided, revisit
whether `install.sh` should install them (with the names kept out of this
public repo, e.g. a machine-local list or 1Password).

### Ruby 2.7.8 work app on this machine

Set up ruby-lsp through a project-local `.lazy.lua` and finish the parts
of the [Ruby guide](guides/ruby.md) marked "to be verified".
