# SRCH Product Design

The authoritative redesign specification is
[`docs/superpowers/specs/2026-08-02-srch-tui-capability-redesign-design.md`](docs/superpowers/specs/2026-08-02-srch-tui-capability-redesign-design.md).

## Durable design contract

- The terminal background belongs to the user. SRCH does not paint a page background.
- A centered, persistent SRCH header identifies the active product mode.
- Primary navigation is product-oriented: Search, Reader, Downloader, API, and History.
- Search uses a progressive hierarchy: Category, Engine, Target, Filters, Fields, Query, Actions.
- Controls appear only when they are meaningful for the active engine and target.
- Presets belong to engines and provide defaults; they are not duplicate engines.
- Keyboard and mouse interaction have feature parity.
- Settings use a transactional draft with explicit apply and discard behavior.
- The same Go application supports macOS, Linux, and Windows and integrates with Bash, Zsh, Fish, and PowerShell.

The specification defines the interaction model, capability schema, migrations, resilience requirements, and acceptance criteria.
