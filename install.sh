#!/usr/bin/env bash
#
# Bootstrap dots on a fresh Omarchy install:
#
#   bash <(curl -fsSL https://raw.githubusercontent.com/fernandoaleman/dots/master/install.sh)
#
# Safe to re-run. Installs prerequisites and apps (via Omarchy's own
# installers), clones (or updates) the repo into ~/Work/dots, backs up any
# real files that would block stow, stows every package listed in
# lib/dots.sh, then adds one line to Omarchy's config files (e.g. ~/.bashrc)
# to load ours.
set -euo pipefail

REPO_URL="https://github.com/fernandoaleman/dots.git"
DOTS_DIR="$HOME/Work/dots"

step() { printf '\n\033[1;34m==> %s\033[0m\n' "$1"; }
ok() { printf '\033[1;32m✔ %s\033[0m\n' "$1"; }
warn() { printf '\033[1;33m! %s\033[0m\n' "$1"; }

# Omarchy's helper: ask for sudo once and keep it alive for the whole run
source omarchy-sudo-keepalive

step "Installing prerequisites"
omarchy-pkg-add git stow
ok "git and stow installed"

# Extra packages from the official repos that Omarchy doesn't install
PACMAN_PACKAGES=(wget nmap)
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

# Web apps (Chrome app windows), created with Omarchy's installer:
# "Name|URL|icon URL". With no icon URL, Omarchy fetches the site's own icon
# (that fails for Slack workspace subdomains, hence the explicit one). They
# open in the default Chrome profile, so logins, notification permissions
# and extensions are shared with Chrome.
WEBAPPS=(
  "Slack|https://1000bulbs.slack.com|https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/png/slack.png"
  "Teams|https://teams.cloud.microsoft"
  "Outlook|https://outlook.office.com"
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
    [[ $(realpath -m "$target") == "$DOTS_DIR"/* ]] && continue
    if [[ -e $target && ! -L $target ]]; then
      mv "$target" "$target.bak.$stamp"
      warn "Moved ~/$rel to ~/$rel.bak.$stamp"
      backed_up=1
    fi
  done < <(find "$DOTS_DIR/$pkg" -type f -print0)
done
((backed_up)) || ok "Nothing to back up"

step "Stowing packages: ${PACKAGES[*]}"
stow --dir "$DOTS_DIR" --target "$HOME" --restow "${PACKAGES[@]}"
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

# tmux plugins, cloned at pinned commits (no TPM). Bump a pin deliberately.
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

# Global mise tools (ours in ~/.config/mise/conf.d/config-dots.toml, plus
# Omarchy's); later kept current by `mise up` in every `omarchy update`
step "Installing mise tools"
mise install --yes
ok "mise tools installed"

# LazyVim Extras live in Omarchy's ~/.config/nvim/lazyvim.json, which LazyVim
# itself rewrites (:LazyExtras), so it can't be stowed. Add ours to its list;
# nothing is removed, so extras toggled locally are kept.
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
