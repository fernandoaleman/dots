#!/usr/bin/env bash
#
# Bootstrap dots on a fresh Omarchy install:
#
#   bash <(curl -fsSL https://raw.githubusercontent.com/fernandoaleman/dots/master/install.sh)
#
# Safe to re-run. Installs prerequisites and apps (via Omarchy's own
# installers), clones (or updates) the repo into ~/Work/dots, backs up any
# real files that would block stow, stows every package in PACKAGES, then
# adds one line to Omarchy's config files (e.g. ~/.bashrc) to load ours.
set -euo pipefail

REPO_URL="https://github.com/fernandoaleman/dots.git"
DOTS_DIR="$HOME/Work/dots"
PACKAGES=(bash)

step() { printf '\n\033[1;34m==> %s\033[0m\n' "$1"; }
ok() { printf '\033[1;32m✔ %s\033[0m\n' "$1"; }
warn() { printf '\033[1;33m! %s\033[0m\n' "$1"; }

# Omarchy's helper: ask for sudo once and keep it alive for the whole run
source omarchy-sudo-keepalive

step "Installing prerequisites"
omarchy-pkg-add git stow
ok "git and stow installed"

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
# our stowed file so our settings override Omarchy's.
add_line() {
  local file=$1 line=$2
  if [[ ! -f $file ]]; then
    warn "${file/#$HOME/\~} not found; skipping"
  elif grep -qxF "$line" "$file"; then
    ok "${file/#$HOME/\~} already loads dots"
  else
    printf '\n# Added by dots: https://github.com/fernandoaleman/dots\n%s\n' "$line" >>"$file"
    ok "${file/#$HOME/\~} now loads dots"
  fi
}

step "Hooking dots into Omarchy's config files"
add_line "$HOME/.bashrc" "[[ -r ~/.bashrc.dots ]] && source ~/.bashrc.dots"

step "Done"
echo "Open a new terminal to load the new shell config."
