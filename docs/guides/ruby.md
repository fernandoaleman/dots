# Ruby in Neovim

How Ruby is set up in Neovim (LazyVim's ruby Extra), and how to handle
projects on a Ruby older than 3.0.

> **Status:** the default setup is decided. The legacy-project section
> follows ruby-lsp's documented approach but is **not yet tested**; finish
> and verify it the first time a Ruby 2.x project (the 2.7.8 work app) is
> set up on this machine.

## Default: ruby-lsp (Ruby 3+ projects)

- LazyVim's **ruby** Extra with its default LSP, **`ruby_lsp`**
  (Shopify's Ruby LSP). Nothing else is configured in dots.
- ruby-lsp creates a **composed bundle** in `.ruby-lsp/` inside the project
  (it doesn't need to be in the project's `Gemfile`), and uses the
  project's own RuboCop.
- Ruby comes from **mise** (on PATH via Omarchy's `env-bootstrap`).
- Formatting runs only on demand (`<leader>cf`), because Omarchy sets
  `vim.g.autoformat = false`.
- To check: ruby-lsp's docs warn that installing it **through Mason "may
  cause errors"** across Ruby versions, and LazyVim installs LSPs through
  Mason by default. If that bites, install ruby-lsp with mise instead and
  set `mason = false` for `ruby_lsp`.

## Legacy projects (Ruby < 3.0, e.g. 2.7.8)

ruby-lsp **can't run on Ruby 2.x** (its gemspec says
`required_ruby_version = ">= 3.0"`). ruby-lsp officially supports this
case: *"If you are working on a project using an older version of Ruby not
supported by Ruby LSP, then you may specify a separate Gemfile for
development tools."* The server runs on Ruby 3 from that separate bundle
while you edit the 2.x code.

Documented trade-offs:

- *"gems will not be installed automatically and neither will `ruby-lsp`
  upgrades"*, so update that bundle by hand.
- *"certain functionality may be degraded … since the Ruby LSP will not be
  able to inspect the project's real bundle"*. Features that depend on
  the app's gems (e.g. going to definitions inside gems) are weaker.
- **Formatting must keep using the project's own RuboCop** on the project's
  Ruby (2.7.8). A RuboCop from the Ruby 3 bundle would be newer than the
  app's pinned version and could format differently from CI. Older RuboCop
  versions also need the older CLI flags (`--auto-correct` instead of
  conform's default `--server -a`).

### Setup (to be verified)

1. **A dev-tools bundle on Ruby 3,** outside the project, e.g.
   `~/Work/.ruby-lsp-legacy/`:
   - `.ruby-version` (or `mise.toml`) pinning a Ruby 3.x installed with
     mise
   - `Gemfile` with `gem "ruby-lsp"` (plus any ruby-lsp add-ons needed)
   - `bundle install` to create the lockfile
2. **A project-local `.lazy.lua`** in the legacy repo. lazy.nvim loads it
   automatically (`local_spec = true` by default) after all other specs,
   and nvim asks you to trust it the first time (`vim.secure`). It:
   - starts `ruby_lsp` with `BUNDLE_GEMFILE` pointing at the dev-tools
     `Gemfile`, run under Ruby 3 via mise;
   - points conform's `rubocop` at the project's RuboCop (Ruby 2.7.8 via
     mise) with the older flags.
3. **Keep `.lazy.lua` out of the work repo's git**: add it to
   `.git/info/exclude` (personal, not shared with the team).
4. **Fallback:** if ruby-lsp is too degraded on that project, use
   **solargraph only there** (in the same `.lazy.lua`), which runs on
   Ruby 2.7.8 with the project's own bundle.

Fill in the exact `.lazy.lua` and paths here once tested.

## Sources

- ruby-lsp gemspec: <https://github.com/Shopify/ruby-lsp/blob/main/ruby-lsp.gemspec>
- Custom Gemfile for older Rubies: <https://github.com/Shopify/ruby-lsp/blob/main/vscode/README.md>
- Editor setup (Neovim, Mason caveat): <https://shopify.github.io/ruby-lsp/editors.html>
- Composed bundle / troubleshooting: <https://shopify.github.io/ruby-lsp/troubleshooting.html>
