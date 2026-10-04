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

### AWS VPN Omarchy plugin

Install the user's own AWS VPN client plugin (outside dots; see
[desktop.md](decisions/desktop.md)) and review it for submission to the
Omarchy plugin catalog.

### Retire the old repos (Mac phase)

Archive `fernandoaleman/nvim` once the Mac uses dots.

## When it happens

### Ruby 2.7.8 work app on this machine

Set up ruby-lsp through a project-local `.lazy.lua` and finish the parts
of the [Ruby guide](guides/ruby.md) marked "to be verified".
