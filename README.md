# srch

`srch` is a cross-platform search, fetch, reader, and download command for macOS, Linux, and Windows. It keeps the fast argument style of `ggl.fish`, but moves the shared behavior into one Go binary so Fish, Zsh, Bash, and PowerShell all receive the same parser and feature set.

Running `srch` without arguments opens a Bubble Tea interface. Running it with a query opens the preferred search engine.

```text
╭──────────────────────────────────────────────╮
│                  SRCH                        │
│       SEARCH  •  FETCH  •  DISCOVER         │
╰──────────────────────────────────────────────╯
```

## Install

Go 1.24 or newer is required to build from source.

```sh
make build
install -m 0755 bin/srch /usr/local/bin/srch
```

On Windows, build with `go build -o bin\\srch.exe .\\cmd\\srch` and put `bin` on `PATH`.

Optional integrations are discovered at runtime: `defuddle`, `glow`, `bat`, `jq`, `curl`, `aria2c`, `yt-dlp`, and `ffmpeg`. The core search, native fetcher, Glamour renderer, and direct downloader do not require them. Run `srch doctor` for the local report.

## Smart search syntax

```sh
srch "bubble tea layouts"                    # preferred web engine
srch @brave "private search"                 # engine selector
srch '!gh' language:go "bubble tea"          # bang and modifier
srch '#images' +transparent "black metal logo"
srch '#images' +svg "terminal icon"
srch '#music' kind:album "transilvanian hunger"
srch --set developer-search "lipgloss table"
srch --print-url @google "quoted safely"
srch --dry-run --set artwork-search "album cover"
printf '%s' 'query from a pipe' | srch --stdin @kagi
```

Selectors are recognized only in selector positions. Use `--` to treat everything after it literally. Shell quoting remains authoritative. Supported modifiers include `site:`, `language:`, `since:`, `size:`, `color:`, `type:`, `kind:`, `stars:`, `region:`, and `safe:`; unsupported engine capabilities fail clearly instead of being silently ignored. `--param key=value` is the advanced escape hatch.

Large multi-engine sets require `--yes` before opening more than five destinations. `--private` and `--no-history` suppress history writes.

## Interactive mode

The search input is at the bottom by default so the selected category, engine, preset, and generated request remain visible while typing. Change it to top or automatic in Settings.

| Key | Action |
|---|---|
| `Tab`, `Shift+Tab`, or `Left` / `Right` | change primary category |
| `Alt+1` … `Alt+8` | jump to a category tab |
| `Ctrl+E` | cycle engines |
| `Ctrl+R` | cycle category presets |
| `Ctrl+M` | cycle Video, Social, Shopping, Extensions, Reference, Privacy, and Health under More |
| `Ctrl+P` / `Ctrl+K` | command palette and utilities |
| `Ctrl+S` / `Ctrl+,` | open Settings directly |
| `Ctrl+Y` | copy the generated URL |
| `Ctrl+H` | toggle the large header |
| `?` | manual |
| `Esc` | close the current surface |

The utility drawer exposes history, favourites/profiles, reader, API, downloads/media, settings, help, and dependency diagnostics. Settings use Huh controls and persist atomically.

## Commands

```text
search       explicit search command and all smart flags
engines      list the typed engine catalogue
categories   show category defaults
presets      list reusable transformations
sets         list multi-engine search sets
history      inspect or clear local history
profiles     save, run, and delete reusable requests
favourites   manage favourite engines (also: favorites)
fetch        bounded native/curl HTTP fetch with formatted output
read         defuddle extraction plus built-in Glamour rendering
download     resumable atomic direct downloads with SHA-256 checking
media        explicit yt-dlp delegation
api          structured-source adapters
config       initialize, inspect, validate, or run guided setup
doctor       report paths, platform, catalogue, and optional tools
completion   generate Fish, Zsh, Bash, or PowerShell completion
```

Examples:

```sh
srch api musicbrainz "Darkthrone"
srch fetch https://example.com --output auto
srch fetch https://example.com --copy-curl
srch read https://example.com/article --viewer glow
srch download https://example.com/file.zip --resume --sha256 DIGEST
srch media https://example.com/video --audio --dry-run
srch completion zsh > _srch
```

## Catalogue and configuration

The built-in catalogue contains more than 100 engines across web, images, AI, music, lyrics, code, research, video, social, shopping, browser extensions, reference, privacy, and health. User catalogue entries are merged by stable ID from `engines.yaml` in the config directory.

Configuration locations follow the operating system:

- macOS/Linux: `$XDG_CONFIG_HOME/srch`, `$XDG_DATA_HOME/srch`, and `$XDG_CACHE_HOME/srch`, with standard home-directory fallbacks.
- Windows: native user config and cache directories.
- Downloads: `~/Downloads/srch` unless overridden.

Run `srch config init` to write the effective defaults, `srch config setup` for the guided form, and `srch config validate` after editing.

## Development

```sh
make check
make cross-build
```

The catalogue and argument behavior are data-driven and covered by targeted tests. Network endpoints can change independently, so catalogue verification status is metadata rather than a promise that a third-party service is continuously available.
