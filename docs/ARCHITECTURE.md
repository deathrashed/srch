# Architecture

`srch` deliberately keeps shell integration thin. `cmd/srch` starts a shared Go application used by every shell and operating system.

- `internal/domain`: stable request, engine, preset, and search-set types.
- `internal/catalog`: embedded catalogue, user override merge, and validation.
- `internal/query`: smart argument parsing and safe URL construction.
- `internal/app`: orchestration boundary shared by CLI and TUI.
- `internal/cli`: Cobra commands, shell completion, and explicit side-effect flags.
- `internal/tui`: Bubble Tea state machine with Bubbles inputs, Lip Gloss layout, and Huh settings.
- `internal/platform`: browser, clipboard, and optional-tool adapters for macOS, Linux, and Windows.
- `internal/state`: atomic history, favourite, and profile persistence.
- `internal/fetch`, `internal/reader`, `internal/download`: bounded HTTP, Markdown reading, and atomic downloads.

The catalogue describes URL behavior rather than embedding per-engine code. Query/path/fragment/fixed placement, aliases, bangs, capabilities, and verification metadata are typed and validated at startup. Presets transform a request without duplicating engines.

Side effects remain explicit: URL construction is pure, opening is separate, bulk opening is guarded, history can be disabled per request, direct downloads use `.part` files and atomic rename, and media acquisition is delegated only when `yt-dlp` is installed.
