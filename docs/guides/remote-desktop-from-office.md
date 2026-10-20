# Remote desktop from the office: what to test

Last time (2026-10-14), from the work Mac at the office: the Moonlight
stream to Omarchy and Screen Sharing to the Mac Studio were both slow, text
was a little fuzzy, and the window froze after a while untouched (fixed by
closing and reconnecting). Office ~500 Mbps, home ~900. Measure first,
change one thing at a time, and write down each result
([TODO](../TODO.md#office-remote-desktop-slow-fuzzy-freezes-when-idle)).

All commands run on the work Mac (Terminal), at the office.

## 1. Is Tailscale direct or relayed?

The CLI is inside the app:

```bash
alias ts=/Applications/Tailscale.app/Contents/MacOS/Tailscale
ts netcheck               # UDP: true/false, nearest DERP relay and its latency
ts ping omarchy           # "via <ip>:<port>" = direct; "via DERP(xxx)" = relayed
ts ping mac-studio
ts status | grep -E 'omarchy|mac-studio'   # "direct" or "relay" per peer
```

Run them twice: with the AWS VPN **off**, then **on**. A relay (DERP) means
the office network blocks the UDP that Tailscale needs for a direct path:
traffic then goes through Tailscale's relay servers, with more latency and
less bandwidth, which would explain both slow streams.

## 2. Moonlight's numbers

Start the Omarchy stream, then press **Ctrl+Option+Shift+S** to show the
stats overlay (again to hide; Moonlight's `input.cpp`). Note:

- resolution / frame rate, and the codec (H.264, HEVC or AV1)
- **network latency** and **frames dropped by your network connection**
- decode time and the bitrate

Move a window around for ~30 s and read them again. Lots of dropped frames
means the network; high decode time means the Mac's decoder.

## 3. The idle freeze

Likely cause on Omarchy (not proven): after **2.5 min** idle Omarchy starts
the screensaver, at **5 min** it locks, and ~5 s later it **turns the
displays off** ([desktop.md](../decisions/desktop.md#idle-and-lock-omarchys)).
Sunshine streams the middle monitor (`DP-2`), so a monitor that's off would
leave nothing to stream.

1. Stream, don't touch anything for **6 minutes**, note when (if) it freezes.
2. Turn on Omarchy's Stay Awake (the same toggle as its menu), over SSH:

   ```bash
   ssh omarchy omarchy-toggle-idle stay-awake
   ```

   Repeat the 6 minutes. If it no longer freezes, that was it. Turn idle
   back on afterwards: `ssh omarchy omarchy-toggle-idle allow-idle`.
3. Screen Sharing to the Mac Studio, same 6 minutes. Its display sleep:
   `ssh mac-studio pmset -g | grep -E 'displaysleep|sleep'`.

Leaving Stay Awake on at home means the unattended desktop doesn't lock, so
that trade-off is a decision for after the test.

## 4. Send the results

Paste the outputs and numbers into Claude Code on the work Mac (this
project's sessions sync to it). What happens next depends on them:

- **Relayed:** ask office IT whether outbound UDP can be allowed for
  Tailscale (port 41641 is its default), or look at Tailscale's relay
  options; nothing on our machines fixes a blocked path.
- **Direct but slow, or fuzzy:** Moonlight's bitrate, resolution (the
  stream is 16:9, the MacBook is 16:10) and codec are the knobs to try, one
  at a time.
- **Freeze is the idle timer:** pick between Stay Awake while away, longer
  `idle.lock` in `shell.json`, or a streaming-only exception.
