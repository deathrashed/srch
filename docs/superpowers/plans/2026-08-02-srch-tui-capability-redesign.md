# SRCH TUI and Search Capability Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the prototype category dashboard with a coherent five-mode TUI backed by one normalized, engine-owned capability model and working keyboard, mouse, Settings, CLI, and cross-platform behavior.

**Architecture:** `domain` owns normalized targets and option values; `catalog` owns engine capabilities, presets, aliases, and confidence; `query` compiles only validated targets; `app` orchestrates side effects; small TUI mode models sit beneath a persistent shell that owns focus, overlays, layout, and mouse hit regions. Existing Reader, download, state, platform, and CLI services remain the implementation spine and are projected into actionable modes rather than duplicated.

**Tech Stack:** Go, Cobra, Bubble Tea v2, Bubbles v2, Lip Gloss v2, Huh v2, Glamour, embedded YAML, Go tests, PTY smoke tests.

---

## File structure

- `internal/domain/types.go`: public request, target, preset, option-group, and Search Set contracts.
- `internal/catalog/catalog.go`: catalog lookup, target resolution, validation, and legacy aliases.
- `internal/catalog/data/engines.yaml`: authoritative engines, targets, presets, option bindings, and confidence.
- `internal/query/parser.go`: order-independent smart-selector parsing.
- `internal/query/builder.go`: provider-neutral compilation from resolved targets.
- `internal/app/app.go`: request normalization and shared Search/Reader/Downloader/API/History commands.
- `internal/config/config.go`: configuration schema, defaults, and versioned migration.
- `internal/tui/model.go`: persistent application shell only.
- `internal/tui/search.go`: Search-mode state, progressive selectors, and actions.
- `internal/tui/modes.go`: Reader, Downloader, API, and History state and updates.
- `internal/tui/settings.go`: transactional Settings draft and focused editors.
- `internal/tui/layout.go`: responsive rendering, styles, header, focus, and hit regions.
- `internal/tui/palette.go`: fuzzy global command palette and command dispatch.
- `internal/*/*_test.go`: behavioral boundaries for each owning package.
- `README.md`, `docs/ARCHITECTURE.md`: user controls, capability tiers, migrations, and platform support.

### Task 1: Normalize engine targets, presets, and option groups

**Files:**
- Modify: `internal/domain/types.go`
- Modify: `internal/catalog/catalog.go`
- Modify: `internal/catalog/data/engines.yaml`
- Test: `internal/catalog/catalog_test.go`

- [ ] **Step 1: Write failing catalog tests**

Add tests that require `PresetsForEngine`, reject a preset belonging to another engine, reject unsupported preset option values, and resolve legacy Metal Archives Band/Album selectors to one canonical engine plus the correct target.

```go
func TestResolveTargetRejectsMismatchedPreset(t *testing.T) {
    catalog := mustLoadCatalog(t)
    _, err := catalog.ResolveTarget(domain.SearchTarget{
        EngineID: "bing-images",
        PresetID: "google-transparent",
    })
    if err == nil {
        t.Fatal("expected an engine/preset mismatch")
    }
}

func TestLegacyMetalAlbumResolvesCanonicalTarget(t *testing.T) {
    catalog := mustLoadCatalog(t)
    resolved, err := catalog.ResolveTarget(domain.SearchTarget{EngineID: "metal-archives-album"})
    if err != nil {
        t.Fatal(err)
    }
    if resolved.Engine.ID != "metal-archives" || resolved.Target.ID != "album" {
        t.Fatalf("unexpected target: %#v", resolved)
    }
}
```

- [ ] **Step 2: Confirm the tests fail**

Run: `go test ./internal/catalog`

Expected: FAIL because normalized targets and target resolution do not exist.

- [ ] **Step 3: Add the normalized contracts**

Introduce `SearchTarget`, `EngineTarget`, `OptionGroup`, `Option`, `Field`, `CapabilityConfidence`, `ModifierBinding`, and `ResolvedTarget`. Remove `Preset.Category`; make `Preset.EngineID` authoritative. Change Search Sets to store `[]SearchTarget`. Keep legacy aliases in catalog data, not in query code.

- [ ] **Step 4: Consolidate catalog data**

Replace duplicate Metal Archives engines with `metal-archives` and Band, Album, and Song targets. Convert Google Images presets to semantic defaults such as `size=large`, `color=transparent`, and `format=svg`. Add documented capability groups for the representative high-confidence engines named in the design; label observed and destination-only behavior explicitly.

- [ ] **Step 5: Validate and commit**

Run: `go test ./internal/catalog`

Expected: PASS with catalog schema, legacy target, and preset ownership tests green.

Commit: `feat: normalize search engine capabilities`

### Task 2: Compile normalized targets without provider leakage

**Files:**
- Modify: `internal/query/parser.go`
- Modify: `internal/query/builder.go`
- Modify: `internal/app/app.go`
- Test: `internal/query/query_test.go`
- Test: `internal/app/app_test.go`

- [ ] **Step 1: Write failing parser and builder tests**

Cover selector-order independence, explicit values overriding preset defaults, compatible image filters composing, conflicting selections replacing one another, Metal Archives Band/Album construction, and Bing never receiving Google `tbs` values.

```go
func TestSelectorsAreOrderIndependent(t *testing.T) {
    left := mustParse(t, `@metal-archives +metal-album "Black Sabbath"`)
    right := mustParse(t, `+metal-album @metal-archives "Black Sabbath"`)
    if diff := cmp.Diff(left.Targets, right.Targets); diff != "" {
        t.Fatalf("targets differ (-left +right):\n%s", diff)
    }
}

func TestExplicitFilterOverridesPreset(t *testing.T) {
    request := requestFor("google-images", "google-large", map[string][]string{"size": {"medium"}})
    url := mustBuild(t, request)
    if strings.Contains(url, "isz:l") || !strings.Contains(url, "isz:m") {
        t.Fatalf("explicit size did not win: %s", url)
    }
}
```

- [ ] **Step 2: Confirm the tests fail**

Run: `go test ./internal/query ./internal/app`

Expected: FAIL on provider-specific builder behavior and global preset selection.

- [ ] **Step 3: Normalize once in the application layer**

Resolve every request to validated `SearchTarget` values before building URLs. Reject mismatches rather than silently dropping presets. Make selector order irrelevant by collecting selectors before resolution.

- [ ] **Step 4: Make the builder generic**

Compile parameter, path, fragment, and query-syntax bindings declared by the selected engine. Merge preset defaults first and explicit option values second. Remove hard-coded Google and Metal Archives branches.

- [ ] **Step 5: Validate and commit**

Run: `go test ./internal/query ./internal/app`

Expected: PASS for normalized Search, Search Sets, and representative engine output.

Commit: `feat: compile engine-owned search targets`

### Task 3: Migrate configuration and persisted targets safely

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/state/state.go`
- Test: `internal/config/config_test.go`
- Test: `internal/state/state_test.go`

- [ ] **Step 1: Write failing migration tests**

Use temporary directories to prove that version-1 Metal Archives defaults and Search Sets migrate to normalized targets, a backup is created, invalid legacy entries are reported, and loading twice is idempotent.

- [ ] **Step 2: Confirm the tests fail**

Run: `go test ./internal/config ./internal/state`

Expected: FAIL because versioned target migration is absent.

- [ ] **Step 3: Add versioned, atomic migration**

Add a schema version, migrate into a draft, validate it, write a timestamped backup, and atomically replace the configuration only after success. Preserve unresolved legacy data in the backup and migration report.

- [ ] **Step 4: Validate and commit**

Run: `go test ./internal/config ./internal/state`

Expected: PASS including repeated-load and partial-failure cases.

Commit: `feat: migrate normalized search targets`

### Task 4: Build the persistent TUI shell and responsive layout

**Files:**
- Rewrite: `internal/tui/model.go`
- Create: `internal/tui/layout.go`
- Test: `internal/tui/model_test.go`
- Test: `internal/tui/layout_test.go`

- [ ] **Step 1: Write failing shell tests**

Assert five product modes, centered tabs, persistent header/subtitle, terminal-native background, compact header breakpoints, focus traversal, mode switching that preserves Search state, and no rejected labels.

```go
func TestSearchViewUsesProductNavigation(t *testing.T) {
    model := newTestModel(t, 110, 34)
    view := model.View().Content
    for _, label := range []string{"Search", "Reader", "Downloader", "API", "History"} {
        if !strings.Contains(view, label) { t.Fatalf("missing %s", label) }
    }
    for _, rejected := range []string{"More:Video", "Search Scope", "Request Preview", "★ Preferred"} {
        if strings.Contains(view, rejected) { t.Fatalf("unexpected %q", rejected) }
    }
}
```

- [ ] **Step 2: Confirm the tests fail**

Run: `go test ./internal/tui`

Expected: FAIL on the current category dashboard.

- [ ] **Step 3: Introduce shell-owned mode and focus state**

Define `ModeSearch`, `ModeReader`, `ModeDownloader`, `ModeAPI`, and `ModeHistory`. The shell owns mode switching, overlays, background detection, focus restoration, and status messages. Child modes own only local task state.

- [ ] **Step 4: Render from one layout tree**

Use Lip Gloss placement and measured widths to center a bounded column. Render the header in full or compact form based on both width and height. Return mouse hit regions alongside content. Set Bubble Tea v2 mouse mode declaratively from `View()`.

- [ ] **Step 5: Validate and commit**

Run: `go test ./internal/tui`

Expected: PASS for wide, narrow, short, light, dark, and monochrome render cases.

Commit: `feat: add persistent product-mode tui shell`

### Task 5: Implement the progressive Search workspace

**Files:**
- Create: `internal/tui/search.go`
- Modify: `internal/tui/model.go`
- Test: `internal/tui/search_test.go`

- [ ] **Step 1: Write failing Search-state tests**

Cover Category to Engine to Target to Filters to Fields to Query focus order, omitted empty rows, engine-owned presets, downstream reset rules, adaptive Album/Track fields, URL copy, open, private mode, and generated request matching displayed selections.

- [ ] **Step 2: Confirm the tests fail**

Run: `go test ./internal/tui -run 'TestSearch'`

Expected: FAIL because Search still uses category-wide presets and static panels.

- [ ] **Step 3: Implement Search state and rendering**

Build visible controls from the current resolved engine target. Use one focused row at a time. Keep text cursor keys inside fields. Display preset defaults as removable selected options, not a separate dashboard summary.

- [ ] **Step 4: Wire actions**

Enter opens, `Ctrl+Y` copies, `Ctrl+S` favourites, `/` focuses the primary query, and request details appear as an overlay. Preserve the query on validation or opener errors.

- [ ] **Step 5: Validate and commit**

Run: `go test ./internal/tui -run 'TestSearch'`

Expected: PASS for keyboard state, engine transitions, adaptive fields, and action dispatch.

Commit: `feat: add progressive search workspace`

### Task 6: Implement palette and transactional Settings

**Files:**
- Create: `internal/tui/palette.go`
- Create: `internal/tui/settings.go`
- Modify: `internal/tui/model.go`
- Test: `internal/tui/palette_test.go`
- Test: `internal/tui/settings_test.go`

- [ ] **Step 1: Write failing overlay tests**

Cover `Ctrl+P` from every mode, fuzzy filtering, keyboard and mouse command dispatch, unavailable requirement text, focus restoration, Settings navigation, focused editors only, apply, discard, persistence, and invalid draft recovery.

- [ ] **Step 2: Confirm the tests fail**

Run: `go test ./internal/tui -run 'TestPalette|TestSettings'`

Expected: FAIL because the current drawer is not a command dispatcher and Huh mutates live config.

- [ ] **Step 3: Implement command dispatch and Settings drafts**

Create typed commands shared by keyboard and mouse. Clone configuration on Settings entry. Use Huh only for the focused value editor or confirmation. Apply the validated draft atomically on `Ctrl+S`; Esc discards it and restores the previous mode and focus.

- [ ] **Step 4: Validate and commit**

Run: `go test ./internal/tui -run 'TestPalette|TestSettings'`

Expected: PASS for overlays, focus restoration, and transactional behavior.

Commit: `feat: add command palette and transactional settings`

### Task 7: Make Reader, Downloader, API, and History actionable

**Files:**
- Create: `internal/tui/modes.go`
- Modify: `internal/app/app.go`
- Modify: `internal/reader/reader.go`
- Modify: `internal/download/download.go`
- Modify: `internal/state/history.go`
- Test: `internal/tui/modes_test.go`
- Test: `internal/app/app_test.go`

- [ ] **Step 1: Write failing mode tests**

Use fake platform and network adapters to cover Reader fetch/cancel/render, direct and media download validation/progress/cancel, API target/results/paging/errors, and History filter/rerun/copy/delete/clear/private exclusion.

- [ ] **Step 2: Confirm the tests fail**

Run: `go test ./internal/tui ./internal/app ./internal/reader ./internal/download ./internal/state`

Expected: FAIL because the prototype modes are static copy.

- [ ] **Step 3: Add typed application commands**

Expose cancellable commands and result messages for each workflow. Keep effects in existing owning services. Preserve inputs after failure and keep credentials out of requests, history, and rendered details.

- [ ] **Step 4: Implement each mode model**

Reader offers Built-in, Glow, Bat, and Raw based on availability. Downloader offers Direct and Media and validates `yt-dlp` before dispatch. API lists only adapter-backed engines. History exposes filterable rows and confirmations.

- [ ] **Step 5: Validate and commit**

Run: `go test ./internal/tui ./internal/app ./internal/reader ./internal/download ./internal/state`

Expected: PASS for success, cancellation, unavailable, timeout, and retry paths.

Commit: `feat: add actionable utility modes`

### Task 8: Complete mouse parity and accessibility

**Files:**
- Modify: `internal/tui/layout.go`
- Modify: `internal/tui/model.go`
- Test: `internal/tui/mouse_test.go`
- Test: `internal/tui/accessibility_test.go`

- [ ] **Step 1: Write failing interaction tests**

Render at two sizes, click every control class using its recorded hit region, resize, and prove stale coordinates do not dispatch. Verify no-color focus text, reduced motion, non-TTY rendering, and Huh accessible mode wiring.

- [ ] **Step 2: Confirm the tests fail**

Run: `go test ./internal/tui -run 'TestMouse|TestAccessibility'`

Expected: FAIL until every component registers hit regions through the layout tree.

- [ ] **Step 3: Route mouse and accessibility through the shell**

Register semantic actions during render and dispatch clicks through the same typed command path as keys. Use cell-motion mode, text focus markers, terminal color downsampling, and preference-controlled motion.

- [ ] **Step 4: Validate and commit**

Run: `go test ./internal/tui -run 'TestMouse|TestAccessibility'`

Expected: PASS at all tested dimensions and color modes.

Commit: `feat: complete tui mouse and accessibility support`

### Task 9: Align CLI arguments, completion, and documentation

**Files:**
- Modify: `internal/cli/root.go`
- Modify: `README.md`
- Modify: `docs/ARCHITECTURE.md`
- Test: `internal/cli/root_test.go`

- [ ] **Step 1: Write failing CLI tests**

Cover engine, target, preset, filters, structured fields, stdin, clipboard, literal `--`, private mode, preview, copy-only, browser, guarded Search Sets, and completion output containing target and filter candidates.

- [ ] **Step 2: Confirm the tests fail**

Run: `go test ./internal/cli`

Expected: FAIL for normalized target flags and dynamic completion.

- [ ] **Step 3: Wire normalized request flags and completions**

Keep shell scripts thin. Generate Bash, Zsh, Fish, and PowerShell completions from catalog lookups and validate incompatible flag combinations before side effects.

- [ ] **Step 4: Update user and architecture documentation**

Document product modes, controls, capability confidence, representative structured searches, optional tools, configuration paths, migration behavior, and cross-platform installation.

- [ ] **Step 5: Validate and commit**

Run: `go test ./internal/cli && go run ./cmd/srch completion bash >/dev/null && go run ./cmd/srch completion zsh >/dev/null && go run ./cmd/srch completion fish >/dev/null && go run ./cmd/srch completion powershell >/dev/null`

Expected: PASS and four non-empty completion scripts.

Commit: `docs: align cli and tui workflows`

### Task 10: Full verification and real-terminal QA

**Files:**
- Modify only if verification exposes a defect in the planned behavior.

- [ ] **Step 1: Run static and automated verification**

Run: `gofmt -w cmd internal && go test -race ./... && go vet ./... && git diff --check`

Expected: all commands exit zero.

- [ ] **Step 2: Build every supported target**

Run the Makefile cross-build targets for macOS arm64/amd64, Linux arm64/amd64, and Windows amd64.

Expected: every target builds successfully without executing foreign binaries.

- [ ] **Step 3: Exercise the real TUI in a PTY**

Verify wide, narrow, and short layouts; Search composition for Google Images, Metal Archives, Last.fm, Wikimedia Commons, GitHub, and PubMed; mouse tabs/selectors/palette/Settings; Settings apply/discard; Reader success/failure; Downloader missing-tool and direct-job paths; API results/errors; History rerun/delete; and clean exit.

Expected: the observable interface matches the approved design and retains terminal background.

- [ ] **Step 4: Reconcile documentation and commit final fixes**

Run: `git status --short && git diff --check`

Expected: only intentional files remain changed; no generated binaries or temporary QA artifacts are staged.

Commit: `feat: complete srch interactive redesign`
