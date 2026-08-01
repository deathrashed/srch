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

The catalogue describes URL behavior rather than embedding per-engine code. Query/path/fragment/fixed placement, aliases, bangs, targets, option groups, modifier bindings, confidence, and verification metadata are typed and validated at startup. Presets belong to one engine and provide semantic defaults; explicit request values override them. Legacy target aliases preserve older selectors without duplicating engines.

The TUI has one persistent shell with five product modes: Search, Reader, Downloader, API, and History. The shell owns focus, overlays, responsive layout, and mouse hit regions. Search derives Category, Engine, Target, Preset, and adaptive fields from the catalog. Settings mutate an isolated draft and replace live configuration only after successful validation and atomic persistence.

Side effects remain explicit: URL construction is pure, opening is separate, bulk opening is guarded, history can be disabled per request, direct downloads use `.part` files and atomic rename, and media acquisition is delegated only when `yt-dlp` is installed.
