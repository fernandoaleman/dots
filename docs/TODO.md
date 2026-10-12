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

### OneDark "dark" theme for Omarchy

The Mac's Ghostty uses OneDark "dark" (generated from
[onedark.nvim](https://github.com/navarasu/onedark.nvim)'s palette;
[macos.md](decisions/macos.md)). Make it an Omarchy custom theme too
(`~/.config/omarchy/themes/`: terminal colors, Neovim via onedark.nvim
`style = "dark"`, Hyprland/shell colors, background), so both machines
match; then the Mac could seed its Ghostty colors from that theme.

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

### Mac phase: script the Mac checklist

[docs/setup/macos.md](setup/macos.md) collects the Mac steps decided so far
(Tailscale DNS off, AWS VPN client, Moonlight + its keyboard setting,
Option as Alt, F-keys, `.hushlogin`, the Mac Studio's HostName). Automate
what can be in the Mac `install.sh`.

### Mac phase: Mac Studio sync scripts

`push-to-mac-studio` / `pull-from-mac-studio` were dropped on Omarchy
(remote desktop replaces syncing). Decide whether the Macs still need them
once remote desktop into Omarchy works ([bin.md](decisions/bin.md)).

### Mac: Tailscale's "Launch at login" checkbox

On the work MacBook the checkbox fails ("Start on Login could not be set")
because macOS has no background-items record of Tailscale's login helper;
a manual Open at Login entry is the workaround
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
