#!/usr/bin/env bash
#
# Bootstrap dots on a fresh Omarchy install or Mac:
#
#   bash <(curl -fsSL https://raw.githubusercontent.com/fernandoaleman/dots/master/install.sh)
#
# Safe to re-run. Installs prerequisites and apps (Omarchy's own installers,
# or Homebrew on a Mac), clones (or updates) the repo into ~/Work/dots, backs
# up any real files that would block stow, stows every package listed in
# lib/dots.sh, then adds one line to Omarchy's config files (e.g. ~/.bashrc)
# to load ours.
#
# The macOS bootstrap below runs under macOS's bash 3.2: keep everything up
# to its `exec` bash-3.2 safe (no associative arrays, ${var,,}, mapfile...).
set -euo pipefail

REPO_URL="https://github.com/fernandoaleman/dots.git"
DOTS_DIR="$HOME/Work/dots"

step() { printf '\n\033[1;34m==> %s\033[0m\n' "$1"; }
ok() { printf '\033[1;32m✔ %s\033[0m\n' "$1"; }
warn() { printf '\033[1;33m! %s\033[0m\n' "$1"; }

case "$(uname -s)" in
Darwin) DOTS_OS=macos ;;
*) DOTS_OS=omarchy ;;
esac

# macOS bootstrap. A fresh Mac has curl and bash 3.2 only (git needs Apple's
# Command Line Tools). Install Homebrew (its installer also installs the
# Command Line Tools), the bootstrap tools, get dots, then re-run the repo's
# install.sh with Homebrew's bash 5: `bash <(curl ...)` leaves no file to
# re-run. ${1+"$@"}: bash 3.2 treats an empty "$@" as unbound under set -u.
if [ "$DOTS_OS" = macos ] && [ "${BASH_VERSINFO[0]}" -lt 4 ]; then
  step "Bootstrapping macOS (Homebrew, bash 5, git, gum, stow, jq)"
  sudo -v
  if [ ! -x /opt/homebrew/bin/brew ]; then
    NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
  fi
  eval "$(/opt/homebrew/bin/brew shellenv)"
  missing=""
  for formula in bash git gum stow jq; do
    brew list --formula "$formula" >/dev/null 2>&1 || missing="$missing $formula"
  done
  # shellcheck disable=SC2086 # word splitting intended (bash 3.2: no arrays of names here)
  [ -z "$missing" ] || brew install $missing
  if [ ! -d "$DOTS_DIR/.git" ]; then
    mkdir -p "$(dirname "$DOTS_DIR")"
    git clone "$REPO_URL" "$DOTS_DIR"
  elif [ -z "$(git -C "$DOTS_DIR" status --porcelain)" ]; then
    git -C "$DOTS_DIR" pull --no-rebase --ff-only
  fi
  exec /opt/homebrew/bin/bash "$DOTS_DIR/install.sh" ${1+"$@"}
fi

# Sudo once for the whole run: Omarchy's helper, or the same idea on macOS
if [[ $DOTS_OS == omarchy ]]; then
  source omarchy-sudo-keepalive
else
  eval "$(/opt/homebrew/bin/brew shellenv)"
  sudo -v
  while kill -0 "$$" 2>/dev/null; do sudo -n true; sleep 50; done 2>/dev/null &
fi

# Machine profile: "personal" or "work" (docs/decisions/macos.md). Asked on
# the first run, remembered in a machine-local file; DOTS_PROFILE overrides.
profile_file="$HOME/.config/dots/profile"
if [[ -z ${DOTS_PROFILE:-} && -r $profile_file ]]; then
  DOTS_PROFILE=$(<"$profile_file")
fi
if [[ -z ${DOTS_PROFILE:-} ]]; then
  step "Machine profile"
  if command -v gum &>/dev/null; then
    DOTS_PROFILE=$(gum choose --header "Is this a personal or a work machine?" personal work)
  else
    read -rp "Is this a personal or a work machine? [personal/work]: " DOTS_PROFILE
  fi
fi
case $DOTS_PROFILE in
personal | work) ;;
*) warn "Unknown profile '$DOTS_PROFILE' (expected personal or work)" && exit 1 ;;
esac
mkdir -p "$(dirname "$profile_file")"
printf '%s\n' "$DOTS_PROFILE" >"$profile_file"
export DOTS_PROFILE
ok "Profile: $DOTS_PROFILE ($DOTS_OS)"

# Get (or update) dots and load lib/dots.sh. Shared by both OSes.
get_dots() {
  step "Getting dots"
  if [[ -d $DOTS_DIR/.git ]]; then
    if [[ -n $(git -C "$DOTS_DIR" status --porcelain) ]]; then
      warn "$DOTS_DIR has local changes; skipping update"
    elif git -C "$DOTS_DIR" rev-parse --abbrev-ref '@{upstream}' &>/dev/null; then
      # --no-rebase overrides Omarchy's pull.rebase=true; --ff-only never merges
      git -C "$DOTS_DIR" pull --no-rebase --ff-only
      ok "Updated $DOTS_DIR"
    else
      warn "$DOTS_DIR has no upstream branch; using it as is"
    fi
  else
    mkdir -p "$(dirname "$DOTS_DIR")"
    git clone "$REPO_URL" "$DOTS_DIR"
    ok "Cloned into $DOTS_DIR"
  fi

  # Packages, include lines and drift checks shared with the Omarchy
  # post-update hook (omarchy/…/post-update.d/dots.hook) and `make doctor`
  source "$DOTS_DIR/lib/dots.sh"
}

# Link dots into $HOME: stow the packages and add the include lines to
# Omarchy's config files. Shared by both OSes; what gets stowed and hooked
# comes from lib/dots.sh (per OS).
link_dots() {
  # stow refuses to replace real files, so move any in the way aside first.
  # Anything that already resolves into dots (a stowed file, or a file under a
  # stowed directory like ~/.config/bash) is left alone.
  step "Backing up files that would conflict with stow"
  stamp="$(date +%Y%m%d%H%M%S)"
  backed_up=0
  for pkg in "${PACKAGES[@]}"; do
    while IFS= read -r -d '' src; do
      rel="${src#"$DOTS_DIR/$pkg/"}"
      target="$HOME/$rel"
      # Only real files are in the way (macOS's realpath has no -m, so test
      # existence first)
      [[ -e $target && ! -L $target ]] || continue
      [[ $(realpath "$target") == "$DOTS_DIR"/* ]] && continue
      mv "$target" "$target.bak.$stamp"
      warn "Moved ~/$rel to ~/$rel.bak.$stamp"
      backed_up=1
    done < <(find "$DOTS_DIR/$pkg" -type f -print0)
  done
  ((backed_up)) || ok "Nothing to back up"

  # --no-folding: real folders, links only for our files. With folding, a
  # folder missing on a fresh machine (e.g. ~/.ssh) becomes one link into the
  # repo, and whatever else is later written there (keys!) lands in the repo.
  step "Stowing packages: ${PACKAGES[*]}"
  [[ " ${PACKAGES[*]} " == *" ssh "* ]] && mkdir -p "$HOME/.ssh" && chmod 700 "$HOME/.ssh"
  stow --dir "$DOTS_DIR" --target "$HOME" --restow --no-folding "${PACKAGES[@]}"
  ok "Stowed"

  # Omarchy's own config files stay real files that Omarchy (and its update
  # migrations) can keep editing. Each gets one line, added last, that loads
  # our stowed file so our settings override Omarchy's (INCLUDES in lib/dots.sh).
  step "Hooking dots into Omarchy's config files"
  for entry in "${INCLUDES[@]}"; do
    file=${entry%%|*} short=${entry%%|*}
    short=${short/#$HOME/\~}
    case $(dots_include "$file" "${entry#*|}") in
    present) ok "$short already loads dots" ;;
    added) ok "$short now loads dots" ;;
    no-file) warn "$short not found; skipping" ;;
    esac
  done
}

# tmux plugins, cloned at pinned commits (no TPM). Bump a pin deliberately.
install_tmux_plugins() {
  TMUX_PLUGINS_DIR="$HOME/.local/share/tmux/plugins"
  TMUX_PLUGINS=(
    "tmux-resurrect https://github.com/tmux-plugins/tmux-resurrect cff343cf9e81983d3da0c8562b01616f12e8d548"
    "tmux-continuum https://github.com/tmux-plugins/tmux-continuum 0698e8f4b17d6454c71bf5212895ec055c578da0"
  )
  step "Installing tmux plugins"
  for entry in "${TMUX_PLUGINS[@]}"; do
    read -r name url commit <<<"$entry"
    dir="$TMUX_PLUGINS_DIR/$name"
    if [[ ! -d $dir/.git ]]; then
      git clone --quiet "$url" "$dir"
    fi
    if [[ $(git -C "$dir" rev-parse HEAD) == "$commit" ]]; then
      ok "$name already at ${commit:0:7}"
    else
      git -C "$dir" fetch --quiet origin
      git -C "$dir" checkout --quiet "$commit"
      ok "$name at ${commit:0:7}"
    fi
  done
}

# LazyVim Extras live in Omarchy's ~/.config/nvim/lazyvim.json, which LazyVim
# itself rewrites (:LazyExtras), so it can't be stowed. Add ours to its list;
# nothing is removed, so extras toggled locally are kept. Shared by both OSes.
add_nvim_extras() {
  NVIM_EXTRAS=(
    ai.sidekick
    lang.ansible lang.docker lang.git lang.go lang.json lang.markdown
    lang.python lang.ruby lang.sql lang.terraform lang.toml lang.yaml
  )
  step "Adding LazyVim Extras"
  lazyvim_json="$HOME/.config/nvim/lazyvim.json"
  if [[ -f $lazyvim_json ]]; then
    extras=$(printf 'lazyvim.plugins.extras.%s\n' "${NVIM_EXTRAS[@]}" | jq -R . | jq -s .)
    merged=$(jq --argjson add "$extras" '.extras = ((.extras // []) + $add | unique)' "$lazyvim_json")
    if [[ $merged == "$(jq . "$lazyvim_json")" ]]; then
      ok "LazyVim Extras already added"
    else
      printf '%s\n' "$merged" >"$lazyvim_json"
      ok "LazyVim Extras added (installed on next nvim start)"
    fi
  else
    warn "No ~/.config/nvim/lazyvim.json found; skipping LazyVim Extras"
  fi
}

# macOS. What isn't built for the Mac yet (secrets) comes
# in later stages; until then the Mac run stops at the end of this block.
if [[ $DOTS_OS == macos ]]; then
  get_dots

  # Omarchy's bash defaults (and ~/.bashrc template) at the pinned tag, laid
  # out like /usr/share/omarchy (macOS doesn't allow writing to /usr/share).
  # macos-env points OMARCHY_PATH here (docs/decisions/macos.md).
  omarchy_dir="$HOME/.local/share/omarchy"
  omarchy_keep=(default/bash default/bashrc config bin/omarchy-menu-tmux-keybindings bin/omarchy-mise-install)
  omarchy_want="$OMARCHY_REF ${omarchy_keep[*]}"
  step "Omarchy's defaults and templates ($OMARCHY_REF)"
  if [[ $(cat "$omarchy_dir/.dots-ref" 2>/dev/null) == "$omarchy_want" ]]; then
    ok "Already at $OMARCHY_REF"
  else
    tmp=$(mktemp -d)
    curl -fsSL "https://github.com/omacom/omarchy/archive/refs/tags/$OMARCHY_REF.tar.gz" |
      tar -xz -C "$tmp" --strip-components=1
    rm -rf "$omarchy_dir"
    for part in "${omarchy_keep[@]}"; do
      mkdir -p "$omarchy_dir/$(dirname "$part")"
      cp -R "$tmp/$part" "$omarchy_dir/$part"
    done
    rm -rf "$tmp"
    echo "$omarchy_want" >"$omarchy_dir/.dots-ref"
    ok "Downloaded into ${omarchy_dir/#$HOME/\~}"
  fi

  # Omarchy's ~/.bashrc and config templates, seeded once if missing
  # (Omarchy's installer does this on Omarchy); link_dots then adds the
  # lines that load ours
  step "Seeding Omarchy's config files"
  seeded=0
  for rel in .bashrc "${OMARCHY_TEMPLATES[@]/#/.config/}"; do
    [[ -e $HOME/$rel ]] && continue
    src="$omarchy_dir/${rel/#.config/config}"
    [[ $rel == .bashrc ]] && src="$omarchy_dir/default/bashrc"
    mkdir -p "$(dirname "$HOME/$rel")"
    cp "$src" "$HOME/$rel"
    ok "Copied ~/$rel"
    seeded=1
  done
  ((seeded)) || ok "All present; leaving them"

  # Omarchy's Neovim config, seeded once if missing: what the omarchy-nvim
  # package builds (LazyVim's starter plus Omarchy's files), from the same
  # pinned sources. Not the package itself: its plugin cache is built for
  # Linux, so lazy.nvim downloads the plugins on the first nvim start. No
  # theme.lua link (no Omarchy themes here): the macos package's
  # theme.dots.lua sets the colorscheme (docs/decisions/macos.md).
  step "Seeding Omarchy's Neovim config"
  nvim_dir="$HOME/.config/nvim"
  if [[ -e $nvim_dir ]]; then
    ok "${nvim_dir/#$HOME/\~} exists; leaving it"
  else
    tmp=$(mktemp -d)
    mkdir -p "$tmp/nvim" "$tmp/omarchy-nvim"
    curl -fsSL "https://github.com/LazyVim/starter/archive/$LAZYVIM_STARTER_REF.tar.gz" |
      tar -xz -C "$tmp/nvim" --strip-components=1
    curl -fsSL "https://github.com/omacom/omarchy-pkgs/archive/$OMARCHY_NVIM_REF.tar.gz" |
      tar -xz -C "$tmp/omarchy-nvim" --strip-components=3 "omarchy-pkgs-$OMARCHY_NVIM_REF/pkgbuilds/omarchy-nvim"
    cp -R "$tmp/omarchy-nvim/lua" "$tmp/omarchy-nvim/plugin" "$tmp/omarchy-nvim/lazyvim.json" "$tmp/nvim/"
    mkdir -p "$(dirname "$nvim_dir")"
    mv "$tmp/nvim" "$nvim_dir"
    rm -rf "$tmp"
    ok "Copied ${nvim_dir/#$HOME/\~} (plugins download on the first nvim start)"
  fi

  link_dots
  add_nvim_extras
  install_tmux_plugins

  step "Installing packages and apps (Brewfile)"
  brew bundle --file="$DOTS_DIR/Brewfile" --no-upgrade
  ok "Brewfile installed"

  # mise as Omarchy sets it up (install/user/mise.sh, mise-work.sh): no pruning
  # of a running tool's old version on `mise up`, ~/Work with its .mise.toml
  # (a project's bin/ on PATH), global Node, and Omarchy's first-run launchers
  # in ~/.local/bin for the coding tools the Mac uses (docs/decisions/macos.md).
  # Then dots' tools (config-dots.toml), as on Omarchy.
  step "mise: Omarchy's setup and dots' tools"
  mise settings set upgrade.auto_prune false
  mkdir -p "$HOME/Work/tries"
  if [[ ! -f $HOME/Work/.mise.toml ]]; then
    printf '[env]\n_.path = "{{ cwd }}/bin"\n' >"$HOME/Work/.mise.toml"
    ok "Wrote ~/Work/.mise.toml"
  fi
  mise trust --quiet "$HOME/Work/.mise.toml"
  if mise config get --global tools.node >/dev/null 2>&1; then
    ok "Node already a global mise tool"
  else
    mise use -g node@latest
    ok "Node installed (global)"
  fi
  for tool in claude codex gh opencode; do
    "$omarchy_dir/bin/omarchy-mise-install" "$tool"
  done
  ok "Launchers in ~/.local/bin: claude codex gh opencode (install on first run)"
  mise install --yes
  ok "mise tools installed"

  # Docker on the Mac: Colima runs Docker Engine in a VM (docs/decisions/macos.md).
  # Colima takes a config file as is (unset keys become zero, not defaults),
  # and its template hardcodes vmType qemu, so the first setup has Colima
  # write its full default config, then sets ours: CPUs, memory, vz +
  # virtiofs (its defaults on macOS 13+) and the docker: section (the VM's
  # daemon.json) with Omarchy's log rotation and dots' networks off
  # 172.17.0.0/16. Not Omarchy's dns: nothing listens on the bridge in the
  # VM. Later runs leave the file to `colima start --edit`.
  step "Colima (Docker Engine in a VM)"
  colima_conf="$HOME/.colima/default/colima.yaml"
  colima_tmpl="$HOME/.colima/_templates/default.yaml"
  if [[ -f $colima_conf ]]; then
    if grep -qF "bip: $DOCKER_BRIDGE_IP/16" "$colima_conf"; then
      ok "Colima config has dots' Docker networks"
    else
      warn "${colima_conf/#$HOME/\~} lacks dots' Docker networks; see docs/decisions/macos.md"
    fi
  else
    had_tmpl=0
    [[ -f $colima_tmpl ]] && had_tmpl=1
    colima template --editor true >/dev/null 2>&1
    mkdir -p "$(dirname "$colima_conf")"
    awk -v cpus="$COLIMA_CPUS" -v mem="$COLIMA_MEMORY" -v bip="$DOCKER_BRIDGE_IP/16" -v pool="$DOCKER_POOL" '
      /^cpu: / { print "cpu: " cpus; next }
      /^memory: / { print "memory: " mem; next }
      /^vmType: / { print "vmType: vz"; next }
      /^mountType: / { print "mountType: virtiofs"; next }
      /^docker: \{\}$/ {
        print "docker:"
        print "  log-driver: json-file"
        print "  log-opts:"
        print "    max-size: 10m"
        print "    max-file: \"5\""
        print "  bip: " bip
        print "  default-address-pools:"
        print "    - base: " pool
        print "      size: 24"
        next
      }
      { print }' "$colima_tmpl" >"$colima_conf"
    ((had_tmpl)) || rm -f "$colima_tmpl"
    ok "Wrote ${colima_conf/#$HOME/\~} ($COLIMA_CPUS CPUs, ${COLIMA_MEMORY} GiB)"
  fi
  if [[ $(brew services info colima --json | jq -r '.[0].running') == true ]]; then
    ok "Colima already running (starts at login)"
  else
    brew services start colima
    ok "Colima started; starts at login (the first start downloads its VM image)"
  fi

  # docker compose / buildx are Homebrew CLI plugins; Docker finds them via
  # cliPluginsExtraDirs. Docker itself writes this file (logins, current
  # context), so merge the key instead of stowing the file.
  step "Docker CLI plugins (compose, buildx)"
  docker_conf="$HOME/.docker/config.json"
  plugins_dir=/opt/homebrew/lib/docker/cli-plugins
  mkdir -p "$(dirname "$docker_conf")"
  [[ -s $docker_conf ]] || echo '{}' >"$docker_conf"
  if jq -e --arg d "$plugins_dir" '(.cliPluginsExtraDirs // []) | index($d)' "$docker_conf" >/dev/null; then
    ok "Docker already finds Homebrew's plugins"
  else
    merged=$(jq --arg d "$plugins_dir" '.cliPluginsExtraDirs = ((.cliPluginsExtraDirs // []) + [$d])' "$docker_conf")
    printf '%s\n' "$merged" >"$docker_conf"
    ok "Added $plugins_dir to ~/.docker/config.json"
  fi

  # Read macOS's own setting: `defaultbrowser` doesn't mark Chrome as current
  # (its bundle ID case differs, com.google.Chrome vs com.google.chrome)
  step "Default browser"
  if plutil -convert json -o - ~/Library/Preferences/com.apple.LaunchServices/com.apple.launchservices.secure.plist 2>/dev/null |
    jq -e '.LSHandlers[]? | select(.LSHandlerURLScheme == "https") | .LSHandlerRoleAll | ascii_downcase == "com.google.chrome"' >/dev/null; then
    ok "Chrome already the default browser"
  else
    defaultbrowser chrome || true
    ok "Asked macOS to make Chrome the default: click \"Use Chrome\""
  fi

  # Bash 5 as the login shell, as on Omarchy (Omarchy's defaults need bash
  # 4+; macOS's /bin/bash is 3.2). chsh only accepts shells in /etc/shells.
  brew_bash=/opt/homebrew/bin/bash
  step "Login shell: $brew_bash"
  grep -qxF "$brew_bash" /etc/shells || echo "$brew_bash" | sudo tee -a /etc/shells >/dev/null
  if [[ $(dscl . -read "$HOME" UserShell | awk '{print $2}') == "$brew_bash" ]]; then
    ok "Already the login shell"
  else
    sudo chsh -s "$brew_bash" "$USER"
    ok "Login shell changed (new terminal windows use it)"
  fi

  step "Done (macOS: bash, config, Neovim, mise and Docker stage)"
  echo "Open a new terminal window to load the new shell."
  echo "Secrets for macOS come in a later stage."
  exit 0
fi

# A fresh Omarchy install has no package lists yet (pacman: "database file
# for 'core' does not exist"; Omarchy shows an "Update System" notice). Run
# Omarchy's own update then, never a bare pacman -Sy (partial upgrade).
if [[ ! -f /var/lib/pacman/sync/core.db || ! -f /var/lib/pacman/sync/extra.db ]]; then
  step "Updating the system first (fresh install)"
  warn "If the update offers to reboot, answer No; reboot once install.sh is done"
  omarchy-update -y
fi

step "Installing prerequisites"
omarchy-pkg-add git stow
ok "git and stow installed"

# Extra packages from the official repos that Omarchy doesn't install
PACMAN_PACKAGES=(wget nmap mariadb-clients pandoc-cli mkcert)
step "Installing packages: ${PACMAN_PACKAGES[*]}"
omarchy-pkg-add "${PACMAN_PACKAGES[@]}"
ok "Packages installed"

# Apps installed through Omarchy's own installers (the same commands the
# Omarchy menu runs), each skipped when already done
step "Installing apps with Omarchy's installers"
if omarchy-pkg-missing 1password 1password-cli; then
  omarchy-install-service-1password
else
  ok "1Password already installed"
fi

if omarchy-pkg-missing google-chrome; then
  omarchy-install-browser chrome
else
  ok "Chrome already installed"
fi

if [[ $(omarchy-default-browser) != chrome ]]; then
  omarchy-default-browser chrome
else
  ok "Chrome already the default browser"
fi

if omarchy-pkg-missing openai-codex-desktop; then
  omarchy-install-ai-chatgpt
else
  ok "ChatGPT desktop already installed"
fi

if omarchy-pkg-missing spotify; then
  omarchy-install-service-spotify
else
  ok "Spotify already installed"
fi

# Tailscale with Omarchy's installer (package, tailscaled, `tailscale up`
# with a browser sign-in, user as operator, Taildrop to ~/Downloads, bar
# widget, admin console web app)
if omarchy-pkg-missing tailscale; then
  omarchy-install-service-tailscale
elif [[ $(tailscale status --json 2>/dev/null | jq -r '.BackendState // empty') == Running ]]; then
  ok "Tailscale already installed and signed in"
else
  warn "Tailscale is installed but not signed in; run: sudo tailscale up --accept-routes"
fi

# Remote desktop host: Sunshine, as Omarchy's omarchy-install-service-sunshine
# does it, with its open bugs fixed (Omarchy 4.0.4; see docs/TODO.md to switch
# back once fixed upstream): enable the real unit (sunshine.service is only an
# Alias, which systemd won't enable), no Hyprland autostart line (the unit is
# WantedBy=graphical-session.target; a second copy crashes), and no Sunshine
# Admin web app with --ignore-certificate-errors (it disables certificate
# checks for the whole browser). Firewall rules are Omarchy's own (same specs
# and comment, so omarchy-remove-service-sunshine still removes them): ports
# open only to private LANs and tailscale0. After Tailscale, so tailscale0
# exists. Clients (Moonlight) are paired by hand (docs/setup/omarchy.md).
SUNSHINE_UNIT=app-dev.lizardbyte.app.Sunshine.service
SUNSHINE_TCP=(47984 47989 48010)
SUNSHINE_UDP=(5353 47998 47999 48000 48002 48010)
omarchy-pkg-add sunshine
if systemctl --user is-enabled --quiet "$SUNSHINE_UNIT"; then
  ok "Sunshine service already enabled"
else
  systemctl --user enable --now "$SUNSHINE_UNIT"
  ok "Sunshine service enabled and started"
fi
for proto in tcp udp; do
  ports=("${SUNSHINE_TCP[@]}")
  [[ $proto == udp ]] && ports=("${SUNSHINE_UDP[@]}")
  for port in "${ports[@]}"; do
    for cidr in 10.0.0.0/8 172.16.0.0/12 192.168.0.0/16; do
      sudo ufw allow in proto "$proto" from "$cidr" to any port "$port" comment omarchy-sunshine >/dev/null
    done
    if ip link show tailscale0 &>/dev/null; then
      sudo ufw allow in on tailscale0 to any port "$port" proto "$proto" comment omarchy-sunshine >/dev/null
    fi
  done
done
sudo ufw reload >/dev/null
ok "Sunshine ports open to private LANs$(ip link show tailscale0 &>/dev/null && echo " and Tailscale")"
# Stream the middle desk monitor (DP-2, workspaces 4-6; see monitors.dots.lua);
# when it isn't connected, Sunshine falls back to a connected one (tested).
# Sunshine's web UI rewrites sunshine.conf, so only this line is managed.
sunshine_conf="$HOME/.config/sunshine/sunshine.conf"
SUNSHINE_OUTPUT="output_name = DP-2"
mkdir -p "$(dirname "$sunshine_conf")" && touch "$sunshine_conf"
if grep -qxF "$SUNSHINE_OUTPUT" "$sunshine_conf"; then
  ok "Sunshine streams DP-2 (middle monitor)"
else
  sed -i '/^output_name[[:space:]]*=/d' "$sunshine_conf"
  echo "$SUNSHINE_OUTPUT" >>"$sunshine_conf"
  systemctl --user try-restart "$SUNSHINE_UNIT"
  ok "Sunshine set to stream DP-2 (middle monitor)"
fi

# Bazecor: configurator for the Dygma keyboard (AUR; repackages Dygma's
# AppImage and includes its udev rule for USB access without root)
if omarchy-pkg-missing bazecor; then
  omarchy-pkg-aur-add bazecor
else
  ok "Bazecor already installed"
fi

# Dictation: Voxtype with Omarchy's installer (menu: Install > AI > Dictation;
# it asks to confirm, downloads a ~150 MB model, sets up the service; F9 hold
# or Super+Ctrl+X via Omarchy's Hyprland bindings). Local, nothing in a cloud.
if omarchy-pkg-missing voxtype-bin; then
  omarchy-voxtype-install
else
  ok "Voxtype (dictation) already installed"
fi

# Syncthing (Obsidian vaults only; never git repos): Arch package + user
# service (Omarchy has no installer). Sync (22000 tcp/udp) and LAN discovery
# (21027/udp) open only to private LANs and tailscale0, like Sunshine; the web
# UI stays on localhost:8384. Devices/folders are paired by hand.
omarchy-pkg-add syncthing
if systemctl --user is-enabled --quiet syncthing.service; then
  ok "Syncthing service already enabled"
else
  systemctl --user enable --now syncthing.service
  ok "Syncthing service enabled and started"
fi
for rule in "22000 tcp" "22000 udp" "21027 udp"; do
  read -r port proto <<<"$rule"
  for cidr in 10.0.0.0/8 172.16.0.0/12 192.168.0.0/16; do
    sudo ufw allow in proto "$proto" from "$cidr" to any port "$port" comment dots-syncthing >/dev/null
  done
  if ip link show tailscale0 &>/dev/null; then
    sudo ufw allow in on tailscale0 to any port "$port" proto "$proto" comment dots-syncthing >/dev/null
  fi
done
sudo ufw reload >/dev/null
ok "Syncthing ports open to private LANs$(ip link show tailscale0 &>/dev/null && echo " and Tailscale")"

# AWS VPN: the user's Omarchy plugin, from its GitHub repo (not yet in the
# plugin catalog). The daemon comes first, with Omarchy's AUR installer, so
# the plugin's setup skips its own yay step. setup (safe to re-run): systemd
# fallback, daemon enabled, CLI on PATH, bar widget; --no-keybind because the
# keybinding lives in hyprland.dots.lua, not Omarchy's bindings.lua.
VPN_PLUGIN_ID=fernandoaleman.aws-vpn-client
VPN_PLUGIN_DIR="$HOME/.config/omarchy/plugins/$VPN_PLUGIN_ID" # omarchy plugin add names it by ID
if omarchy-pkg-missing openlawsvpn-daemon; then
  omarchy-pkg-aur-add openlawsvpn-daemon
else
  ok "openlawsvpn-daemon already installed"
fi
if omarchy plugin list | grep -q "^$VPN_PLUGIN_ID "; then
  ok "AWS VPN plugin already added"
else
  omarchy plugin add https://github.com/fernandoaleman/omarchy-aws-vpn-client --enable --yes
fi
"$VPN_PLUGIN_DIR/setup" --no-keybind

# Web apps (Chrome app windows), created with Omarchy's installer:
# "Name|URL|icon URL". With no icon URL, Omarchy fetches the site's own icon
# (that fails for Slack workspace subdomains, hence the explicit one). They
# open in the default Chrome profile, so logins, notification permissions
# and extensions are shared with Chrome.
WEBAPPS=(
  "Slack|https://work.slack.com|https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/png/slack.png"
  "Teams|https://teams.cloud.microsoft"
  "Outlook|https://outlook.office.com"
  "Todoist|https://app.todoist.com"
)
for webapp in "${WEBAPPS[@]}"; do
  IFS='|' read -r name url icon <<<"$webapp"
  launcher="$HOME/.local/share/applications/$name.desktop"
  # Skip when the launcher exists and already opens this URL; otherwise
  # (re)create it, so a changed URL here updates the launcher too
  if [[ -f $launcher ]] && grep -qF "\"$url\"" "$launcher"; then
    ok "$name web app already installed"
  elif omarchy-webapp-install "$name" "$url" "$icon" >/dev/null; then
    ok "$name web app installed"
  else
    warn "Could not install the $name web app (icon download failed?)"
  fi
done

get_dots
link_dots

install_tmux_plugins

# tmux-ssh (synchronized SSH panes per host or group), downloaded at a
# pinned commit into ~/.local/bin (on PATH via Omarchy's bash envs). The
# SHA-256 skips the download on re-runs and rejects anything else. Bump
# both deliberately.
TMUX_SSH_COMMIT=1a27fb4064a1839e5fc04cdcd00ddf3fb2da347c
TMUX_SSH_SHA256=1ad96c5123535619d59245b2a6975d94bf4954cf3fe0c24d85034767ab78c934
step "Installing tmux-ssh"
tmux_ssh="$HOME/.local/bin/tmux-ssh"
if [[ -f $tmux_ssh ]] && sha256sum -c --status <<<"$TMUX_SSH_SHA256  $tmux_ssh"; then
  ok "tmux-ssh already at ${TMUX_SSH_COMMIT:0:7}"
else
  mkdir -p "$(dirname "$tmux_ssh")"
  download=$(mktemp)
  curl -fsSL "https://raw.githubusercontent.com/tmux-ssh/tmux-ssh/$TMUX_SSH_COMMIT/tmux-ssh" -o "$download"
  if sha256sum -c --status <<<"$TMUX_SSH_SHA256  $download"; then
    install -m 755 "$download" "$tmux_ssh"
    ok "tmux-ssh at ${TMUX_SSH_COMMIT:0:7}"
  else
    warn "tmux-ssh download doesn't match its pinned SHA-256; not installed"
  fi
  rm -f "$download"
fi

# Global mise tools (ours in ~/.config/mise/conf.d/config-dots.toml, plus
# Omarchy's); later kept current by `mise up` in every `omarchy update`
step "Installing mise tools"
mise install --yes
ok "mise tools installed"

# Todoist CLI (td, a mise tool) agent skills: claude-code -> ~/.claude/skills
# (Claude Code, opencode), universal -> ~/.agents/skills (Codex, opencode).
# `td skill install` exits 0 even when the skill exists, so ask `td skill
# list`. Run through `mise exec`: mise's shims may not be on PATH yet.
step "Installing Todoist agent skills"
td() { mise exec -- td "$@"; }
td_skill_state() { # prints e.g. "[not installed]" for agent $1
  td skill list 2>/dev/null | awk -v agent="$1" '
    $1 == agent && NF == 1 { found = 1; next }
    found && /\[/ { gsub(/^[[:space:]]+|[[:space:]]+$/, ""); print; found = 0 }'
}
for agent in claude-code universal; do
  state=$(td_skill_state "$agent")
  if [[ $state == "[not installed]" ]]; then
    td skill install "$agent" >/dev/null
    ok "Todoist skill installed for $agent"
  else
    ok "Todoist skill for $agent: ${state:-unknown}"
  fi
done

add_nvim_extras

# Sudoless Docker (opt-in; Omarchy's default needs sudo): Omarchy's script adds
# the user to the docker group (root-equivalent) after its own warning and
# confirmation. The reboot it needs is deferred (it records reboot-required).
step "Checking sudoless Docker"
if omarchy-sudo-docker --configured; then
  OMARCHY_DEFER_REBOOT=1 omarchy-setup-security-sudoless-docker
  omarchy-sudo-docker --configured || warn "Sudoless Docker takes effect after a reboot"
else
  ok "Sudoless Docker enabled (user in the docker group)"
fi

# Docker's default bridge (172.17.0.0/16) clashes with a work VPN that
# routes 172.17.x.x, and Docker's built-in pools for other networks (docker
# compose) start there too. Move the bridge to $DOCKER_BRIDGE_IP/16 and the
# pools to $DOCKER_POOL (lib/dots.sh). Omarchy points container DNS at the
# bridge address in three places, so all three follow: "dns" in its
# daemon.json (merged, its other settings kept), the resolved stub listener
# (its 20-docker-dns.conf, cleared and replaced by our later drop-in) and
# the firewall rule allowing container DNS.
step "Moving Docker's networks off 172.17.0.0/16"
daemon_json=/etc/docker/daemon.json
if [[ -f $daemon_json ]]; then
  docker_changed=0
  desired=$(jq --arg ip "$DOCKER_BRIDGE_IP" --arg pool "$DOCKER_POOL" \
    '.bip = "\($ip)/16" | .dns = [$ip] | ."default-address-pools" = [{base: $pool, size: 24}]' "$daemon_json")
  if [[ $desired == "$(jq . "$daemon_json")" ]]; then
    ok "Docker bridge already on $DOCKER_BRIDGE_IP/16"
  else
    printf '%s\n' "$desired" | sudo tee "$daemon_json" >/dev/null
    ok "Docker bridge set to $DOCKER_BRIDGE_IP/16, other networks to $DOCKER_POOL"
    docker_changed=1
  fi

  dns_dropin=$(printf '%s\n' "[Resolve]" \
    "# Docker DNS on dots' bridge address, replacing Omarchy's 20-docker-dns.conf" \
    "DNSStubListenerExtra=" "DNSStubListenerExtra=$DOCKER_BRIDGE_IP")
  if [[ $(cat "$DOCKER_DNS_DROPIN" 2>/dev/null) == "$dns_dropin" ]]; then
    ok "Container DNS already on $DOCKER_BRIDGE_IP"
  else
    printf '%s\n' "$dns_dropin" | sudo tee "$DOCKER_DNS_DROPIN" >/dev/null
    sudo systemctl restart systemd-resolved
    ok "Container DNS moved to $DOCKER_BRIDGE_IP"
  fi

  # Same rules as Omarchy's firewall.sh, for the new address (ufw skips
  # rules that already exist)
  for src in 172.16.0.0/12 192.168.0.0/16; do
    sudo ufw allow in proto udp from "$src" to "$DOCKER_BRIDGE_IP" port 53 comment 'allow-docker-dns' >/dev/null
  done
  ok "Firewall allows container DNS to $DOCKER_BRIDGE_IP"

  if ((docker_changed)) && systemctl is-active --quiet docker; then
    sudo systemctl restart docker
    ok "Docker restarted"
  fi
else
  warn "No $daemon_json (Docker not installed?); skipping"
fi

# Secrets from 1Password, found by tag (never by vault or item name; see
# docs/decisions/secrets.md). Never prints a value. A file whose contents
# differ is backed up as <file>.bak.<stamp> before the 1Password version is
# installed; unchanged files are left alone.
step "Installing secrets from 1Password"

# put_secret FILE MODE < contents: install FILE from stdin, via a temp file
# next to it (not in /tmp)
put_secret() {
  local file=$1 mode=$2 tmp short=${1/#$HOME/\~}
  mkdir -p "$(dirname "$file")"
  tmp=$(mktemp "$(dirname "$file")/.dots-secret.XXXXXX")
  cat >"$tmp"
  if [[ -f $file ]] && cmp -s "$tmp" "$file"; then
    rm -f "$tmp"
    chmod "$mode" "$file"
    ok "$short up to date"
    return
  fi
  if [[ -e $file ]]; then
    mv "$file" "$file.bak.$stamp"
    warn "Backed up $short to $short.bak.$stamp"
  fi
  chmod "$mode" "$tmp"
  mv "$tmp" "$file"
  ok "$short installed"
}
safe_name() { [[ $1 =~ ^[A-Za-z0-9._-]+$ ]]; }                 # a file name
safe_path() { [[ $1 =~ ^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)+$ && $1 != *..* ]]; } # under ~

# `op account list` never prompts; without an account, `op vault list` would
# ask to add one interactively, so it only runs once an account exists
# (stdin closed, so it can't ask anything either way)
if ! command -v op &>/dev/null ||
  [[ -z $(op account list --format json 2>/dev/null | jq -r '.[]?.url' 2>/dev/null) ]] ||
  ! op vault list </dev/null &>/dev/null; then
  warn "1Password CLI can't read your vaults: sign in to the 1Password app, turn on"
  warn "Settings > Developer > Integrate with 1Password CLI, then re-run install.sh"
else
  # dots/ssh: SSH Key items -> ~/.ssh/<title> (600) and <title>.pub (644)
  mkdir -p "$HOME/.ssh" && chmod 700 "$HOME/.ssh"
  while IFS=$'\t' read -r id vault title; do
    safe_name "$title" || { warn "Skipping SSH key with an unusable title: $title"; continue; }
    op read "op://$vault/$id/private key?ssh-format=openssh" | put_secret "$HOME/.ssh/$title" 600
    op read "op://$vault/$id/public key" | put_secret "$HOME/.ssh/$title.pub" 644
  done < <(op item list --tags dots/ssh --categories "SSH Key" --format json | jq -r '.[] | [.id, .vault.id, .title] | @tsv')

  # dots/*: Documents whose title is a path -> ~/<title> (600)
  while IFS=$'\t' read -r id vault title; do
    safe_path "$title" || continue # e.g. the VPN profile: no install path yet
    op document get "$id" --vault "$vault" | put_secret "$HOME/$title" 600
  done < <(op item list --tags dots --categories Document --format json | jq -r '.[] | [.id, .vault.id, .title] | @tsv')
  [[ -d $HOME/.aws ]] && chmod 700 "$HOME/.aws"

  # dots/aws: API Credential items -> ~/.aws/credentials profile [<title>];
  # other profiles (e.g. [mfa], written by aws-role-login) are kept. Values
  # go to awk through its environment, never on a command line.
  creds="$HOME/.aws/credentials"
  while IFS=$'\t' read -r id vault title; do
    safe_name "$title" || { warn "Skipping AWS credential with an unusable title: $title"; continue; }
    AWS_KEY_ID=$(op read "op://$vault/$id/access key id") \
      AWS_SECRET=$(op read "op://$vault/$id/secret access key") \
      awk -v profile="$title" '
        function section() {
          print "[" profile "]"
          print "aws_access_key_id = " ENVIRON["AWS_KEY_ID"]
          print "aws_secret_access_key = " ENVIRON["AWS_SECRET"]
        }
        # Replace the section in place, keeping the file layout (blank
        # lines) as the AWS CLI left it, so re-runs find nothing to change
        $0 == "[" profile "]" { section(); found = 1; skip = 1; next }
        /^\[/ { skip = 0 }
        skip && /^[[:space:]]*$/ { print; next }
        !skip { print }
        END { if (!found) { if (NR) print ""; section() } }' "$( [[ -f $creds ]] && echo "$creds" || echo /dev/null)" |
      cat -s | put_secret "$creds" 600
  done < <(op item list --tags dots/aws --categories "API Credential" --format json | jq -r '.[] | [.id, .vault.id, .title] | @tsv')

  # dots/token: every ALL_CAPS_WITH_UNDERSCORE field -> ~/.config/dots/env
  # (600), loaded by ~/.bashrc.dots
  {
    echo "# Generated by dots install.sh from 1Password (tag dots/token); edit there."
    op item list --tags dots/token --format json | jq -r '.[].id' | while read -r id; do
      op item get "$id" --format json --reveal |
        jq -r '.fields[] | select((.label // "") | test("^[A-Z][A-Z0-9]*(_[A-Z0-9]+)+$")) | [.label, .value] | @tsv'
    done | sort | while IFS=$'\t' read -r name value; do printf 'export %s=%q\n' "$name" "$value"; done
  } | put_secret "$HOME/.config/dots/env" 600

  # dots/vpn Documents -> the AWS VPN plugin's profiles (600), registered once,
  # in place, named after the file without .ovpn
  vpn_profiles="$HOME/.config/omarchy/aws-vpn-client/profiles"
  while IFS=$'\t' read -r id vault title; do
    safe_name "$title" && [[ $title == *.ovpn ]] || { warn "Skipping VPN profile with an unusable title: $title"; continue; }
    op document get "$id" --vault "$vault" | put_secret "$vpn_profiles/$title" 600
    if omarchy-aws-vpn-client list | grep -qw -- "${title%.ovpn}"; then
      ok "VPN profile ${title%.ovpn} already registered"
    else
      omarchy-aws-vpn-client add "$vpn_profiles/$title" --name "${title%.ovpn}" --link
    fi
  done < <(op item list --tags dots/vpn --categories Document --format json | jq -r '.[] | [.id, .vault.id, .title] | @tsv')

  # Incoming SSH: Omarchy's script, authorizing only the personal key
  pubkey=$(op read "op://Private/id_ed25519/public key" | awk '{print $1, $2}')
  if systemctl is-active --quiet sshd && [[ -f /etc/ssh/sshd_config.d/10-omarchy-hardening.conf ]] &&
    awk '{print $1, $2}' "$HOME/.ssh/authorized_keys" 2>/dev/null | grep -qxF "$pubkey"; then
    ok "sshd already set up (key authorized, passwords off)"
  else
    omarchy-setup-security-sshd --key="$pubkey"
  fi

  # Switch the dots remote to SSH once GitHub accepts the key
  # (ssh -T exits 1 even on success, GitHub gives no shell: match the reply)
  github_reply=$(ssh -o BatchMode=yes -T git@github.com 2>&1 || true)
  if [[ $(git -C "$DOTS_DIR" remote get-url origin) == https://* &&
    $github_reply == *"successfully authenticated"* ]]; then
    make -C "$DOTS_DIR" --no-print-directory ssh
  fi
fi

# Todoist logs in through TODOIST_API_TOKEN (a dots/token field, written to
# ~/.config/dots/env above), so there is no browser login to do
step "Checking Todoist login"
if (set -a && source "$HOME/.config/dots/env" 2>/dev/null && td auth status &>/dev/null); then
  ok "Todoist authenticated (TODOIST_API_TOKEN)"
else
  warn "Todoist not authenticated: add TODOIST_API_TOKEN to a 1Password item tagged dots/token"
fi

# Omarchy's installer sets these from the name and email you enter; ask once
# if they were left blank. `git config --global` writes them to Omarchy's
# ~/.config/git/config (as long as no ~/.gitconfig exists), never into dots.
step "Checking git identity"
ask() {
  if command -v gum &>/dev/null; then
    gum input --prompt "$1: " --placeholder "$1"
  else
    read -rp "$1: " reply && printf '%s' "$reply"
  fi
}
if [[ -e $HOME/.gitconfig ]]; then
  warn "Found ~/.gitconfig; git config --global writes there instead of ~/.config/git/config"
fi
for key in user.name user.email; do
  if [[ -n $(git config --global --get "$key") ]]; then
    ok "git $key: $(git config --global --get "$key")"
  else
    value=$(ask "git $key")
    if [[ -n $value ]]; then
      git config --global "$key" "$value"
      ok "git $key set"
    else
      warn "git $key left unset"
    fi
  fi
done

# Omarchy's ~/.XCompose types your name (CapsLock, Space, n) and email
# (CapsLock, Space, e); its installer fills them from the name/email entered
# at install time, so they can be left empty. Fill empty ones from git.
step "Checking XCompose name/email snippets"
xcompose="$HOME/.XCompose"
if [[ -f $xcompose ]]; then
  filled=0
  for entry in "n|user.name" "e|user.email"; do
    key=${entry%%|*}
    empty="<Multi_key> <space> <$key> : \"\""
    value=$(git config --global --get "${entry#*|}" || true)
    if grep -qxF "$empty" "$xcompose" && [[ -n $value ]]; then
      value=${value//\\/\\\\} value=${value//\"/\\\"} # XCompose string escapes
      content=$(<"$xcompose")
      printf '%s\n' "${content//"$empty"/"<Multi_key> <space> <$key> : \"$value\""}" >"$xcompose"
      ok "XCompose <$key> set from git ${entry#*|}"
      filled=1
    fi
  done
  if ((filled)); then
    omarchy-restart-xcompose || warn "Could not restart XCompose; log out and back in"
  else
    ok "XCompose name/email already set (or no git identity)"
  fi
else
  warn "No ~/.XCompose found; skipping"
fi

step "Checking dots"
dots_doctor || true

step "Done"
echo "Open a new terminal to load the new shell config."
