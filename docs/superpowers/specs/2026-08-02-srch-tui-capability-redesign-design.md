# SRCH TUI and Search Capability Redesign

**Status:** Approved design

**Date:** 2026-08-02

**Scope:** Product navigation, Search composition, engine capabilities, Settings, cross-platform behavior, and migration from the current prototype.

## 1. Purpose

SRCH is a shell-independent Go application for searching, reading, downloading, querying APIs, and revisiting local activity. Its interactive interface must remain fast for a one-line web search while scaling to structured searches such as Google Images filters, Metal Archives targets, MusicBrainz entities, GitHub qualifiers, PubMed fields, and Wikimedia Commons media constraints.

The redesign replaces the current category-tab dashboard with a task-oriented workspace. It removes redundant panels, makes every visible control operable, and gives all five product modes a real workflow.

## 2. Goals and non-goals

### Goals

- Make the current task and focus location obvious without documentation.
- Preserve a short path from launch to search execution.
- Support engine-specific targets, structured fields, and composable filters without duplicating engines.
- Provide keyboard and mouse parity.
- Inherit the user's terminal background and adapt to light, dark, monochrome, narrow, and short terminals.
- Keep CLI argument behavior and the TUI backed by the same catalog and request model.
- Support macOS, Linux, and Windows plus Bash, Zsh, Fish, and PowerShell integration.
- Represent external-search stability and requirements honestly.

### Non-goals

- Recreating a graphical browser inside the terminal.
- Scraping every destination to reproduce its full website UI.
- Promising that undocumented third-party URL parameters are stable.
- Filling wide terminals edge to edge or adding decorative dashboard panels.
- Treating each filter combination as a separate engine.

## 3. Product shell and information architecture

The application has five primary modes:

1. **Search** composes and opens or copies searches.
2. **Reader** fetches a URL and renders readable content.
3. **Downloader** manages direct and delegated media downloads.
4. **API** executes supported structured API searches and presents results.
5. **History** revisits locally stored activity.

Favourites, profiles, Settings, help, and Doctor are support surfaces available through the global command palette rather than primary product tabs.

The centered header persists across all screens. The large form uses the SRCH ASCII mark when the terminal has enough width and height; otherwise it uses a compact wordmark. Its subtitle reflects the active mode, such as `SEARCH · FIND · OPEN`, `READER · FETCH · READ`, or `SETTINGS · PREFERENCES`.

The content column has a readable maximum width and is centered. SRCH renders foregrounds, borders, and selected-control accents but never paints a page-wide background.

## 4. Search workspace

Search presents a progressive sequence of controls:

1. **Category** narrows the catalog, for example Web, Images, AI, Music, Lyrics, Code, Research, Video, Social, Shopping, Extensions, Reference, Privacy, or Health.
2. **Engine** selects one destination within the category.
3. **Target** selects the entity being searched when the engine supports more than one, such as Artist, Album, Track, Band, Release, Song, Image, Audio, or Video.
4. **Filters** exposes compatible option groups declared by the selected engine and target.
5. **Fields** collects structured values needed by the target, such as Artist plus Album or repository plus code query.
6. **Query** contains the primary free-text search.
7. **Actions** open the search, copy its URL, save it as a favourite, or show request details.

A row is omitted when it has no meaningful choices. There are no Search Scope or Request Preview panels. The query is not repeated elsewhere. The generated URL is available through request details or the copy action.

Changing Category resets Engine, Target, filters, and fields to valid defaults. Changing Engine resets engine-owned state. Changing Target preserves only compatible fields and filters. The interface never displays a stale selection that will not be executed.

### Representative compositions

- **Google Images:** Images target with Size, Transparency, Format, Type, and Recency groups. Large, Transparent, SVG, Icon, and HD are presets that supply defaults to these groups.
- **Metal Archives:** one engine with Band, Album, and Song targets instead of duplicate engines.
- **Last.fm:** Artist uses one artist field; Album uses Artist plus Album; Track uses Artist plus Track.
- **MusicBrainz:** Artist, Recording, Release, Release Group, Label, and Work targets with their documented fields.
- **Wikimedia Commons:** Image, Audio, and Video targets with license, file type, size, sort, and structured-data filters where a stable compiler exists.
- **GitHub:** Repositories, Code, Issues, and Users with documented qualifiers and Boolean or regular-expression behavior where supported.
- **PubMed:** structured fields, date ranges, article types, language, species, sex, age, and availability filters.

## 5. Capability and request model

An engine is the destination family. A target is an entity supported by that engine. A preset is an engine-owned set of defaults. Filters and fields describe refinements available for the active target.

The normalized selection is:

```text
SearchTarget {
    EngineID
    TargetID
    PresetID
}
```

Each engine declares:

- identifiers, display metadata, aliases, categories, and search method;
- supported targets;
- target-specific structured fields;
- option groups and compiler bindings;
- authentication, login, regional, external-tool, or API-key requirements;
- verification metadata and stability classification.

Option groups use explicit behavior:

- **target:** exclusive entity choice;
- **select:** one value replaces another in the same group;
- **multi:** compatible values combine into one request;
- **boolean:** on or off;
- **range:** lower and upper bounds;
- **field:** structured user input;
- **query syntax:** documented operators compiled into a query;
- **requirements:** prerequisites surfaced before execution.

Compatible filters compose. Selecting a conflicting value replaces the previous value in the same exclusivity group or is prevented with a concise explanation. Explicit user selections override preset defaults.

Presets are resolved through `PresetsForEngine(engineID)`. Category is derived from the selected engine rather than duplicated as preset authority. Search Sets store normalized targets so a set can include a specific engine target and preset.

The URL/query builder remains provider-neutral. Provider behavior is catalog data or a small, explicit adapter when a documented API cannot be represented declaratively.

## 6. Capability confidence and external-service policy

Every externally encoded feature has one stability level:

- **Documented:** supported by current official documentation or a stable public API. It is eligible for normal compilation and validation.
- **Observed:** verified against current browser behavior or required by an existing SRCH preset, but not promised as a public contract. It is labelled best-effort.
- **Destination-only:** the destination offers the control, but SRCH has no reliable way to encode it. SRCH opens the destination without pretending to apply the control.

SRCH does not invent query parameters. A catalog verification command validates schemas, required placeholders, conflicts, and known URL output and can optionally smoke-test observed destinations. Runtime availability remains distinct from catalog validity.

High-confidence initial structured families include Wikimedia Commons, Openverse, Flickr, Internet Archive, MusicBrainz, Discogs, Last.fm, Metal Archives, GitHub, GitLab, Sourcegraph, Stack Overflow, PubMed, Crossref, arXiv, Semantic Scholar, Open Library, and MediaWiki. Browser-only or volatile services remain plain text searches until their refinements can be represented honestly.

## 7. Product-mode workflows

### Search

The default mode. Enter opens the current request, `Ctrl+Y` copies its URL, and request details reveal the compiled destination and stability without occupying the normal workspace.

### Reader

Reader accepts a URL or a compatible history item. The user chooses Built-in, Glow, Bat, or Raw rendering when available. Fetching is asynchronous and cancellable. Content is scrollable and preserves the source URL. Missing optional tools are shown before use and do not break Built-in mode.

### Downloader

Downloader supports Direct and Media jobs. Direct jobs expose destination, resume, overwrite, and optional checksum behavior. Media jobs expose video/audio, format, output template, and playlist choices and delegate explicitly to `yt-dlp` when installed. Jobs show progress, final path, cancellation, and recoverable failure details.

### API

API mode lists only engines with an API adapter. It uses the same target, filter, and structured-field definitions as Search. Results have list and detail views, paging where supported, explicit authentication status, rate-limit feedback, and retryable errors.

### History

History is filterable by text, mode, category, engine, and date. Entries can be rerun, copied, favourited, or deleted with confirmation. Clear All requires stronger confirmation. Private requests never enter history.

## 8. Focus, keyboard, and mouse behavior

The parent Bubble Tea model owns routing, focus, overlays, and mouse hit testing. Components receive input only while focused. Huh is used for a focused editor or confirmation flow, not as an always-active full-screen Settings model.

Global controls:

- `Alt+Left` / `Alt+Right`: previous or next product mode.
- Number keys: direct product-mode jump when a text editor is not consuming input.
- `Tab` / `Shift+Tab`: next or previous visible control.
- Arrow keys: move within the focused row or list.
- `/`: focus the primary query or filter input.
- `Ctrl+P`: open the searchable command palette everywhere.
- `Ctrl+S`: apply Settings; elsewhere save or favourite when applicable.
- `Esc`: close the top overlay, cancel editing, or move back one level.
- `?`: contextual help for the current mode and focus.

Text inputs retain normal cursor navigation; category switching never steals Left or Right from non-empty input. Each visible keyboard-selectable element has an equivalent mouse action. Click hit regions are rebuilt from the rendered layout on every frame so responsive movement does not leave stale coordinates.

Focus uses a strong border or accent. Selected-but-unfocused values use a quieter selected style. A selector never displays the focus cursor when another control is focused. Text labels accompany symbols so meaning does not depend on color or glyphs.

## 9. Command palette

`Ctrl+P` opens a fuzzy-searchable overlay containing:

- product-mode navigation;
- Search actions and recent engines;
- Reader, Downloader, API, and History actions;
- favourites and profiles;
- Settings categories;
- help and Doctor commands.

Entries are commands, not static destinations. Each enabled entry performs its action. Unavailable entries state the missing requirement. Keyboard selection and mouse click share the same command dispatch path. Closing the palette restores the previous focus.

## 10. Settings

Settings is a compact, transactional list divided into Search, Appearance, Behavior, Integrations, Paths, and Accessibility.

Each row displays its name and current draft value. Arrow keys navigate rows. Enter opens only the focused value editor. `Ctrl+S` validates and atomically applies the complete draft. Esc closes an editor or, from the settings list, discards the draft and returns to the previous mode. Unsaved edits never mutate live configuration.

Settings include:

- default category, engine, target, and category preferences;
- browser choice and platform opener behavior;
- query input position and header size;
- layout density, hints, theme, monochrome mode, and motion;
- history and private-search behavior;
- mouse support and key-map preferences;
- Reader renderer and fetch limits;
- download directory, collision policy, and `yt-dlp` integration;
- API credentials by reference, never rendered in plain text;
- config, data, cache, and download paths;
- accessibility preferences including reduced motion and no-color behavior.

Unavailable integration choices remain discoverable with their requirements. Settings do not show a selector cursor until that selector owns focus.

## 11. Responsive and visual behavior

- The terminal's background is always preserved.
- The workspace is centered with a maximum readable width.
- Product tabs and short selector rows are centered within the workspace.
- Wide terminals gain breathing room rather than extra dashboard panels.
- Narrow terminals stack fields and actions while preserving their order.
- Short terminals use the compact header and a scrollable body.
- No essential state relies on color alone.
- Light, dark, ANSI-16, ANSI-256, true-color, monochrome, and non-TTY output degrade gracefully.
- Spinners and animation appear only during real work and honor reduced-motion settings.

## 12. Side effects, errors, and recovery

URL construction and request validation are pure. Browser opening, clipboard writes, network fetches, downloads, API calls, and persistence are separate commands with visible results.

- Unsupported controls are omitted; unavailable integrations state why they cannot run.
- Observed filters are labelled best-effort. If their compiler fails validation, SRCH offers the unfiltered destination and identifies which refinements were not applied.
- Network work exposes loading, cancellation, timeout, rate-limit, retry, and terminal error states.
- Bulk opening requires confirmation above a configurable threshold.
- Reader, Downloader, and API errors preserve user input for correction or retry.
- Direct downloads use partial files and atomic rename; collision and overwrite behavior is explicit.
- Configuration and state writes are atomic.
- Invalid Settings drafts cannot replace a valid configuration.
- Credentials are referenced through environment or platform-appropriate secret providers and are not written into history or rendered request details.

## 13. CLI and shell behavior

The CLI and TUI resolve the same normalized request. Smart arguments include engine aliases and bangs, category selection, engine and target selection, preset selection, composable filters, structured fields, stdin, clipboard, literal `--`, private mode, URL preview, copy-only, browser selection, and guarded multi-engine Search Sets.

Completion generators expose valid categories, engines, targets, presets, filters, and commands for Bash, Zsh, Fish, and PowerShell. Shell wrappers remain thin and optional. Path resolution uses native configuration, data, cache, and download locations on macOS, Linux/XDG, and Windows.

## 14. Migration

Configuration migration is versioned, creates a recoverable backup, and reports changed keys.

- Duplicate engines such as Metal Archives Band and Metal Archives Album become one engine with Band, Album, and Song targets.
- Legacy engine identifiers resolve through explicit aliases.
- Category-scoped presets migrate to engine-owned presets.
- Existing Search Sets migrate from engine IDs to normalized engine, target, and preset selections.
- Current Google image presets retain equivalent output and are marked observed/best-effort when they depend on undocumented parameters.
- Invalid or ambiguous legacy entries are preserved in the backup and reported rather than silently discarded.

## 15. Component boundaries

- **Domain:** normalized requests, targets, fields, filters, presets, Search Sets, and validation rules.
- **Catalog:** embedded definitions, user overrides, capability confidence, migration aliases, and schema verification.
- **Query:** smart argument parsing and provider-neutral request compilation.
- **Application:** orchestration shared by CLI and TUI.
- **TUI shell:** product navigation, persistent header, focus graph, overlays, responsive layout, and status reporting.
- **Mode models:** Search, Reader, Downloader, API, History, Settings, palette, and help, each owning its local state.
- **Platform:** browser, clipboard, filesystem paths, and optional-tool discovery for macOS, Linux, and Windows.
- **State:** atomic history, favourites, profiles, configuration, and migrations.
- **Network services:** bounded fetch, Reader extraction, downloads, API adapters, and explicit `yt-dlp` delegation.

Mode models communicate through typed application commands rather than reading or mutating each other's state. Layout rendering returns both visual output and mouse hit regions from the same component tree.

## 16. Verification and acceptance criteria

The redesign is accepted when all of the following are demonstrated:

- Search, Reader, Downloader, API, and History are reachable and perform their documented primary workflows.
- The persistent header, contextual subtitle, centered workspace, terminal-native background, and compact breakpoints render correctly in wide, narrow, and short live PTYs.
- Every visible control works by keyboard and mouse.
- `Ctrl+P`, Settings, help, and Escape behavior work from every product mode.
- Category or engine changes rebuild downstream target, preset, filter, and field state without stale selections.
- Compatible filters compose, conflicting filters cannot produce contradictory requests, and explicit choices override preset defaults.
- Representative Google Images, Metal Archives, Last.fm, MusicBrainz, Wikimedia Commons, GitHub, and PubMed requests compile as specified.
- Settings support edit, focused selection, apply, discard, persistence, invalid-input recovery, and accurate integration availability.
- Private activity is excluded from history and credentials are excluded from rendered or persisted request data.
- Existing configuration and Search Sets migrate with backups and explicit reports.
- Targeted unit and integration tests cover catalog validation, parsing, compilation, migrations, mode transitions, focus, mouse dispatch, and transactional Settings.
- CLI completion generation succeeds for Bash, Zsh, Fish, and PowerShell.
- Builds succeed for macOS arm64 and amd64, Linux arm64 and amd64, and Windows amd64.
- Manual QA in a real terminal exercises representative searches, command palette, mouse navigation, Settings, Reader, Downloader, API results, History, cancellation, missing integrations, and network failures.

## 17. Delivery boundaries

This specification is delivered in dependency order: normalize the domain and catalog first, migrate query and CLI behavior, build the reusable TUI shell and Search mode, then complete the other product modes and cross-platform verification. The implementation plan divides that sequence into independently verifiable milestones. No milestone introduces duplicate engine definitions or a second request model.
