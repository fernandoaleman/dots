# Mounting Google Drive and iCloud Drive (rclone)

**Not set up** (decided 2026-10-04). This guide records what the old setup
did and how to bring it back on Omarchy. Omarchy itself has no Google Drive
or iCloud integration (its only cloud installer is Dropbox:
`omarchy-install-service-dropbox`).

## What the old setup did

- **Google Drive** mounted at `~/GoogleDrive` by a systemd user service
  (`rclone mount gdrive:`), remote `gdrive` created from an OAuth client
  ID/secret and token kept in an encrypted tokens file.
- **iCloud Drive** mounted at `~/iCloudDrive` (`rclone mount icloud:`),
  remote `icloud` created from the Apple ID and password, then finished by
  hand with 2FA (`rclone config reconnect icloud:`).
- A **patched rclone** built from a fork (`mikegillan/rclone`, PR #9209)
  into `/usr/local/bin/rclone`, because upstream lacked Apple's SRP
  sign-in at the time.
- A **monthly reminder** (systemd timer, 1st of the month 09:00) to
  re-authenticate iCloud.

## What changed

- **No patched build needed:** PR #9209 (*"iclouddrive: replace plaintext
  signin with SRP authentication"*) was merged 2026-04-02 and shipped in
  **rclone v1.74.0**; Arch's official `rclone` (1.75.1 as of this writing)
  includes it.
- **`fusermount3`**, not `/bin/fusermount`: Omarchy ships `fuse3` only.
- **Credentials come from 1Password**, never the repo: the rclone config
  (`~/.config/rclone/rclone.conf`) holds OAuth tokens and the Apple ID
  password.

## Bringing it back

1. **Install rclone:** add `rclone` to `PACMAN_PACKAGES` in `install.sh`
   (installed with `omarchy-pkg-add`).
2. **Create the remotes** (credentials from 1Password, e.g. via `op read`):

   ```sh
   rclone config create gdrive drive \
     client_id "<id>" client_secret "<secret>" scope drive token '<token json>'
   rclone config create icloud iclouddrive \
     apple_id "<apple id>" password "<password>"
   rclone config reconnect icloud:   # Apple 2FA; can't be scripted
   ```

3. **Add the units** in a stow package (e.g. `rclone/.config/systemd/user/`):

   `rclone-gdrive.service` (and `rclone-icloud.service` with `icloud:` and
   `%h/iCloudDrive`):

   ```ini
   [Unit]
   Description=Mount Google Drive with rclone
   After=network-online.target
   Wants=network-online.target

   [Service]
   Type=notify
   ExecStartPre=/usr/bin/mkdir -p %h/GoogleDrive
   ExecStart=/usr/bin/rclone mount gdrive: %h/GoogleDrive \
     --vfs-cache-mode full \
     --vfs-cache-max-age 72h \
     --vfs-cache-max-size 10G \
     --dir-cache-time 5m \
     --poll-interval 30s \
     --log-level INFO
   ExecStop=/usr/bin/fusermount3 -u %h/GoogleDrive
   Restart=on-failure
   RestartSec=10

   [Install]
   WantedBy=default.target
   ```

   Optional iCloud re-auth reminder, `rclone-icloud-reauth.timer` (monthly,
   `OnCalendar=*-*-01 09:00:00`, `Persistent=true`) triggering a oneshot
   `rclone-icloud-reauth.service` that runs
   `notify-send -u critical -a rclone "iCloud Drive Re-auth Required" "Run: rclone config reconnect icloud:"`.

4. **Enable** (only after the remote exists, e.g. guarded in `install.sh`
   with `rclone listremotes | grep -q '^gdrive:$'`):

   ```sh
   systemctl --user daemon-reload
   systemctl --user enable --now rclone-gdrive.service
   systemctl --user enable --now rclone-icloud.service      # after 2FA
   systemctl --user enable --now rclone-icloud-reauth.timer
   ```

5. Record the 2FA step in `docs/setup/omarchy.md`.

The original files are in the old chezmoi repo (`~/Work/dotfiles`):
`dotfiles/dot_config/systemd/user/rclone-*.{service,timer}` and
`dotfiles/.chezmoiscripts/run_once_after_{15-setup-rclone-gdrive,31-build-rclone-icloud,32-setup-rclone-icloud}.sh.tmpl`.

**Mac:** Google Drive has its own desktop app and iCloud Drive is built in,
so this is Omarchy-only.
