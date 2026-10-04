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
