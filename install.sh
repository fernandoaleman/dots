#!/usr/bin/env bash
#
# Bootstrap dots on a fresh Omarchy install:
#
#   bash <(curl -fsSL https://raw.githubusercontent.com/fernandoaleman/dots/master/install.sh)
#
# Safe to re-run. Installs prerequisites, clones (or updates) the repo into
# ~/Work/dots, backs up any real files that would block stow, then stows
# every package listed in PACKAGES.
set -euo pipefail

REPO_URL="https://github.com/fernandoaleman/dots.git"
DOTS_DIR="$HOME/Work/dots"
PACKAGES=(bash)

step() { printf '\n\033[1;34m==> %s\033[0m\n' "$1"; }
ok() { printf '\033[1;32m✔ %s\033[0m\n' "$1"; }
warn() { printf '\033[1;33m! %s\033[0m\n' "$1"; }

step "Installing prerequisites"
sudo pacman -S --needed --noconfirm git stow
ok "git and stow installed"

step "Getting dots"
if [[ -d $DOTS_DIR/.git ]]; then
  if git -C "$DOTS_DIR" rev-parse --abbrev-ref '@{upstream}' &>/dev/null; then
    git -C "$DOTS_DIR" pull --ff-only
    ok "Updated $DOTS_DIR"
  else
    warn "$DOTS_DIR has no upstream branch; using it as is"
  fi
else
  mkdir -p "$(dirname "$DOTS_DIR")"
  git clone "$REPO_URL" "$DOTS_DIR"
  ok "Cloned into $DOTS_DIR"
fi

# stow refuses to replace real files (e.g. Omarchy's stock ~/.bashrc), so move
# them aside first. Symlinks already pointing into dots are left alone.
step "Backing up files that would conflict with stow"
stamp="$(date +%Y%m%d%H%M%S)"
backed_up=0
for pkg in "${PACKAGES[@]}"; do
  while IFS= read -r -d '' src; do
    rel="${src#"$DOTS_DIR/$pkg/"}"
    target="$HOME/$rel"
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

step "Done"
echo "Open a new terminal to load the new shell config."
