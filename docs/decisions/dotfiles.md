# Small dotfiles

Started 2026-10-04 (in progress). Old source: the top-level files of the
chezmoi repo (`dot_gemrc`, `dot_default-gems`, `dot_rspec`,
`dot_hushlogin`, `dot_confluence-cli/`).

## `.gemrc`: one line, `ruby` package

Omarchy has no `~/.gemrc` and installs no Ruby (projects get Ruby from
`.ruby-version` via mise; [Ruby guide](../guides/ruby.md)). Checked against
RubyGems' source (`lib/rubygems/config_file.rb`,
`install_update_options.rb`):

| Old setting | Effect | Kept |
|---|---|---|
| `gem/install/update: --no-document` | RubyGems otherwise builds `ri` docs on every `gem install` (`document: %w[ri]`) | **yes** (`gem: --no-document` covers install and update) |
| `:backtrace: false` | hides backtraces; the default is `true` | no |
| `:sources`, `:update_sources: true`, `:bulk_threshold: 1000` | RubyGems' defaults | no |
| `:benchmark`, `:plugins` | not read by RubyGems | no |

`ruby/.gemrc` (new `ruby` stow package). Bundler never builds docs, so this
only affects direct `gem install`. **Mac:** same file.

## `.default-gems`: dropped

mise installed the listed gems (`pry`, `neovim`) into every Ruby it
installed. **Dropped** (2026-10-04):

- mise is retiring the file: *"Default package files are deprecated. They
  are still supported for now, but mise will start warning in `2026.11.0`
  and support will be removed in `2027.11.0`"* (mise docs, Ruby). Its
  replacements: a `gem:` tool, or a `postinstall` hook on a ruby entry.
- `neovim` (Ruby provider) only serves Neovim plugins written in Ruby;
  LazyVim uses none.
- `pry`: Ruby 3's `irb` (highlighting, completion, multi-line editing)
  covers most of it; projects that want pry list it in their `Gemfile`.

Re-enable snippets in [troubleshooting](../guides/troubleshooting.md).
**Mac:** same.

## `.rspec`: dropped

Global RSpec defaults (`--color`, `--format documentation`, `--backtrace`,
`--profile 10`, `--order random`, `--seed 12345`). **Dropped**
(2026-10-04), checked against rspec-core's `configuration_options.rb`:

- Only full-line comments are stripped (`/\A\s*#/`); each line is then
  shell-split, so the old inline comments became extra arguments, which
  RSpec treats as files to run.
- A fixed `--seed` makes `--order random` the same order every run.
- RSpec colors terminal output by default.
- A global `~/.rspec` mixes into every project's own `.rspec`, so runs
  differ from CI and teammates.

Projects keep their own `.rspec`; pass flags directly for one-offs.
**Mac:** same.
