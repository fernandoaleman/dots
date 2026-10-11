# Homebrew packages for the Macs (docs/decisions/macos.md). Used only by the
# macOS branch of install.sh: brew bundle --file=Brewfile --no-upgrade.
# Not a stow package, so never linked into $HOME. Ruby: the profile
# ("personal" or "work", from install.sh) decides personal-only entries.
personal = ENV.fetch("DOTS_PROFILE", "personal") != "work"

tap "peteonrails/voxtype"

# Bootstrap (install.sh installs these first)
brew "bash"
brew "git"
brew "gum"
brew "stow"
brew "jq"

# Command-line tools mirroring Omarchy's
brew "bash-completion@2"
brew "bat"
brew "btop"
brew "dua-cli"
brew "eza"
brew "fastfetch"
brew "fd"
brew "fzf"
brew "herdr"
brew "imagemagick"
brew "lazydocker"
brew "lazygit"
brew "libyaml"
brew "mise"
brew "neovim"
brew "ripgrep"
brew "starship"
brew "tlrc" # tldr
brew "tmux"
brew "tree-sitter-cli"
brew "try"
brew "usage"
brew "zoxide"

# Ours, as on Omarchy
brew "wget"
brew "nmap"
brew "shellcheck"
brew "mysql-client", link: true # keg-only: mysql, mysqldump, mysql_config
brew "libpq", link: true        # keg-only: psql, pg_dump
brew "pandoc"
brew "mkcert"
brew "rsync" # macOS's own rsync is limited
brew "defaultbrowser"
brew "syncthing" # Obsidian vaults only

# Docker, CLI only (Colima runs the engine)
brew "colima"
brew "docker"
brew "docker-compose"
brew "docker-buildx"

brew "yt-dlp" if personal

# Apps
cask "1password"
cask "1password-cli"
cask "google-chrome"
cask "tailscale-app"
cask "moonlight"
cask "aws-vpn-client"
cask "ghostty"
cask "font-jetbrains-mono-nerd-font"
cask "slack"
cask "zoom"
cask "chatgpt"
cask "claude"
cask "obsidian"
cask "plaud"
cask "todoist-app"
cask "voxtype"
cask "appcleaner"
cask "qlmarkdown"
cask "keepingyouawake"
cask "libreoffice"

if personal
  cask "localsend"
  cask "logi-options+"
  cask "spotify"
  cask "reflex-app"
  cask "bazecor"
end
