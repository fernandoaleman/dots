# shellcheck shell=bash
#
# Shared by install.sh, the Omarchy post-update hook (dots.hook) and
# `make doctor`: which packages dots stows, which lines make Omarchy's own
# config files load ours (config pattern C), and the drift checks.

DOTS_DIR="${DOTS_DIR:-$HOME/Work/dots}"

# Stow packages (top-level directories mirroring $HOME)
PACKAGES=(bash git nvim tmux mise omarchy hypr bin ssh ruby)

# "Omarchy file|line that loads our .dots file" (appended when missing)
INCLUDES=(
  "$HOME/.bashrc|[[ -r ~/.bashrc.dots ]] && source ~/.bashrc.dots"
  "$HOME/.config/git/config|[include] path = ~/.config/git/config.dots"
  "$HOME/.config/tmux/tmux.conf|source-file -q ~/.config/tmux/tmux.dots.conf"
  "$HOME/.config/hypr/hyprland.lua|do local f = os.getenv(\"HOME\") .. \"/.config/hypr/hyprland.dots.lua\"; local h = io.open(f); if h then h:close() dofile(f) end end"
  "$HOME/.config/hypr/monitors.lua|do local f = os.getenv(\"HOME\") .. \"/.config/hypr/monitors.dots.lua\"; local h = io.open(f); if h then h:close() dofile(f) end end"
)

# Docker networks moved off 172.17.0.0/16, which a work VPN routes: the
# default bridge (and container DNS) to this address, other networks
# (e.g. docker compose) to this pool. Applied by install.sh.
DOCKER_BRIDGE_IP=172.31.0.1
DOCKER_POOL=192.168.128.0/17
DOCKER_DNS_DROPIN=/etc/systemd/resolved.conf.d/30-dots-docker-dns.conf

# dots_include FILE LINE: append LINE to FILE if missing.
# Prints "present", "added" or "no-file".
dots_include() {
  local file=$1 line=$2 comment="#"
  [[ $file == *.lua ]] && comment="--" # Lua comments start with --
  if [[ ! -f $file ]]; then
    echo no-file
  elif grep -qxF "$line" "$file"; then
    echo present
  else
    printf '\n%s Added by dots: https://github.com/fernandoaleman/dots\n%s\n' "$comment" "$line" >>"$file"
    echo added
  fi
}

# dots_doctor [--notify]: re-add missing include lines, then report drift:
# dots files in $HOME no longer linked into the repo (e.g. replaced by an
# Omarchy migration's `sed -i`), and uncommitted changes in the repo (e.g.
# a migration writing through a link). Silent apart from one line when all
# is well; with --notify, also sends one Omarchy notification on problems.
dots_doctor() {
  local notify=0 entry file line path rel issues=()
  [[ ${1:-} == --notify ]] && notify=1

  for entry in "${INCLUDES[@]}"; do
    file=${entry%%|*} line=${entry#*|}
    case $(dots_include "$file" "$line") in
    added) issues+=("Re-added the dots include line to ${file/#$HOME/\~}") ;;
    no-file) issues+=("${file/#$HOME/\~} is missing") ;;
    esac
  done

  while IFS= read -r path; do
    rel=${path#*/}
    if [[ $(realpath -m "$HOME/$rel") != "$DOTS_DIR/$path" ]]; then
      issues+=("$HOME/$rel is not linked to dots (replaced or missing; run install.sh)")
    fi
  done < <(git -C "$DOTS_DIR" ls-files -- "${PACKAGES[@]}")

  # Files inside a package that aren't committed (untracked or ignored)
  # exist only on this machine: a fresh clone would lack them (machine-local
  # overrides live in $HOME, not in the package folders)
  while IFS= read -r path; do
    issues+=("$DOTS_DIR/$path is not committed (missing from fresh clones)")
  done < <(git -C "$DOTS_DIR" ls-files --others -- "${PACKAGES[@]}")

  # Docker moved off 172.17.0.0/16 (install.sh); Omarchy owns daemon.json,
  # so an update could put it back
  if [[ -f /etc/docker/daemon.json ]] &&
    [[ $(jq -r '.bip // ""' /etc/docker/daemon.json) != "$DOCKER_BRIDGE_IP/16" || ! -f $DOCKER_DNS_DROPIN ]]; then
    issues+=("Docker networks are on 172.17.0.0/16, which clashes with the work VPN (run install.sh)")
  fi

  if [[ -n $(git -C "$DOTS_DIR" status --porcelain --untracked-files=no) ]]; then
    issues+=("$DOTS_DIR has uncommitted changes (review: git -C ${DOTS_DIR/#$HOME/\~} status)")
  fi

  if ((${#issues[@]} == 0)); then
    echo "dots: everything in place"
    return 0
  fi

  printf "dots: %s\n" "${issues[@]/#$HOME/\~}"
  if ((notify)) && command -v omarchy-notification-send &>/dev/null; then
    omarchy-notification-send -u normal "dots needs attention" \
      "${issues[0]/#$HOME/\~}$( ((${#issues[@]} > 1)) && echo " (+$((${#issues[@]} - 1)) more)")"
  fi
  return 1
}
