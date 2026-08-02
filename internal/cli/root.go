package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"

	searchapi "srch/internal/api"
	"srch/internal/app"
	"srch/internal/domain"
	"srch/internal/download"
	"srch/internal/fetch"
	"srch/internal/reader"
	"srch/internal/state"
	"srch/internal/tui"
)

type searchOptions struct {
	engine    string
	category  string
	preset    string
	searchSet string
	browser   string
	output    string
	exclude   []string
	rawParams []string
	exact     bool
	printURL  bool
	copyURL   bool
	dryRun    bool
	explain   bool
	private   bool
	yes       bool
	stdin     bool
	clipboard bool
	noHistory bool
	site      string
	language  string
	since     string
	size      string
	color     string
	typeValue string
	kind      string
	stars     string
	region    string
	safe      string
}

func New(environment *app.Environment, version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "srch [selectors] [query]",
		Short:         "Search, fetch, read, and download from the terminal",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
	}
	options := bindSearchFlags(root)
	root.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && !hasSearchFlags(cmd) {
			return tui.Run(environment)
		}
		return runSearch(cmd.Context(), environment, args, options)
	}

	search := &cobra.Command{Use: "search [selectors] [query]", Short: "Run an explicit search", Args: cobra.ArbitraryArgs}
	searchOptions := bindSearchFlags(search)
	search.RunE = func(cmd *cobra.Command, args []string) error {
		return runSearch(cmd.Context(), environment, args, searchOptions)
	}
	root.AddCommand(search)
	root.AddCommand(newEnginesCommand(environment))
	root.AddCommand(newCategoriesCommand(environment))
	root.AddCommand(newPresetsCommand(environment))
	root.AddCommand(newSetsCommand(environment))
	root.AddCommand(newHistoryCommand(environment))
	root.AddCommand(newProfilesCommand(environment))
	root.AddCommand(newFavouritesCommand(environment))
	root.AddCommand(newConfigCommand(environment))
	root.AddCommand(newDoctorCommand(environment, version))
	root.AddCommand(newFetchCommand(environment))
	root.AddCommand(newReadCommand(environment))
	root.AddCommand(newDownloadCommand(environment))
	root.AddCommand(newMediaCommand(environment))
	root.AddCommand(newAPICommand(environment))
	root.AddCommand(newCompletionCommand(root))
	root.AddCommand(&cobra.Command{Use: "version", Short: "Print version information", Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "srch %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
	}})
	return root
}

func bindSearchFlags(command *cobra.Command) *searchOptions {
	options := &searchOptions{output: "plain"}
	flags := command.Flags()
	flags.StringVarP(&options.engine, "engine", "e", "", "search engine ID or alias")
	flags.StringVarP(&options.category, "category", "c", "", "search category")
	flags.StringVarP(&options.preset, "preset", "p", "", "search preset")
	flags.StringVar(&options.searchSet, "set", "", "multi-engine Search Set")
	flags.StringVarP(&options.browser, "browser", "b", "", "browser name or executable")
	flags.StringVarP(&options.output, "output", "o", "plain", "output mode: plain, json, jsonl, markdown, tsv")
	flags.StringSliceVarP(&options.exclude, "exclude", "x", nil, "exclude a term")
	flags.StringSliceVar(&options.rawParams, "param", nil, "advanced raw query parameter key=value")
	flags.BoolVar(&options.exact, "exact", false, "quote the complete query")
	flags.BoolVar(&options.printURL, "print-url", false, "print generated URLs instead of opening")
	flags.BoolVar(&options.copyURL, "copy", false, "copy the first generated URL")
	flags.BoolVar(&options.dryRun, "dry-run", false, "show the request and URLs without side effects")
	flags.BoolVar(&options.explain, "explain", false, "show parsed request details")
	flags.BoolVar(&options.private, "private", false, "do not write history")
	flags.BoolVar(&options.noHistory, "no-history", false, "do not write history")
	flags.BoolVarP(&options.yes, "yes", "y", false, "approve bulk actions")
	flags.BoolVar(&options.stdin, "stdin", false, "read query text from standard input")
	flags.BoolVar(&options.clipboard, "clipboard", false, "read query text from the clipboard")
	flags.StringVar(&options.site, "site", "", "restrict to a site")
	flags.StringVar(&options.language, "language", "", "set language")
	flags.StringVar(&options.since, "since", "", "set recency such as y1 or m3")
	flags.StringVar(&options.size, "size", "", "set image size")
	flags.StringVar(&options.color, "color", "", "set image colour")
	flags.StringVar(&options.typeValue, "type", "", "set result type")
	flags.StringVar(&options.kind, "kind", "", "set domain-specific kind")
	flags.StringVar(&options.stars, "stars", "", "set repository star filter")
	flags.StringVar(&options.region, "region", "", "set region")
	flags.StringVar(&options.safe, "safe", "", "set safe-search behavior")
	return options
}

func hasSearchFlags(command *cobra.Command) bool {
	return command.Flags().NFlag() > 0
}

func runSearch(ctx context.Context, environment *app.Environment, args []string, options *searchOptions) error {
	if options.stdin {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read query from stdin: %w", err)
		}
		args = append(args, strings.TrimSpace(string(data)))
	}
	if options.clipboard {
		value, err := readClipboard(environment)
		if err != nil {
			return err
		}
		args = append(args, value)
	}
	selectors := make([]string, 0, 3)
	if options.engine != "" {
		selectors = append(selectors, "@"+options.engine)
	}
	if options.category != "" {
		selectors = append(selectors, "#"+options.category)
	}
	if options.preset != "" {
		selectors = append(selectors, "+"+options.preset)
	}
	request, err := environment.Parse(append(selectors, args...))
	if err != nil {
		return err
	}
	request.Private = options.private || options.noHistory
	request.Modifiers.Exact = options.exact
	request.Modifiers.Exclude = append([]string(nil), options.exclude...)
	values := map[string]string{
		"site": options.site, "language": options.language, "since": options.since,
		"size": options.size, "color": options.color, "type": options.typeValue,
		"kind": options.kind, "stars": options.stars, "region": options.region, "safe": options.safe,
	}
	for name, value := range values {
		if value != "" {
			request.Modifiers.Values[name] = value
		}
	}
	for _, raw := range options.rawParams {
		key, value, ok := strings.Cut(raw, "=")
		if !ok || key == "" {
			return fmt.Errorf("invalid --param %q; expected key=value", raw)
		}
		request.Modifiers.RawQuery[key] = value
	}
	if options.searchSet != "" {
		request.SearchSetID = options.searchSet
		request.EngineIDs = nil
	}
	request.Output = domain.OutputMode(options.output)
	urls, err := environment.URLs(request)
	if err != nil {
		return err
	}
	if options.explain || options.dryRun || options.output == "json" {
		return writeJSON(os.Stdout, map[string]any{"request": request, "urls": urls})
	}
	if options.printURL {
		for _, built := range urls {
			fmt.Fprintln(os.Stdout, built.URL)
		}
		return nil
	}
	if options.copyURL {
		if len(urls) == 0 {
			return fmt.Errorf("no URL generated")
		}
		if err := environment.Platform.Copy(urls[0].URL); err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, "Copied", urls[0].URL)
		return nil
	}
	if len(urls) > 5 && !options.yes {
		return fmt.Errorf("refusing to open %d destinations without --yes", len(urls))
	}
	browser := options.browser
	if browser == "" {
		browser = environment.Config.DefaultBrowser
	}
	for _, built := range urls {
		if err := environment.Platform.OpenURL(built.URL, browser); err != nil {
			return fmt.Errorf("open %s: %w", built.Engine.Name, err)
		}
		fmt.Fprintf(os.Stdout, "Opened %s\n", built.Engine.Name)
	}
	if environment.Config.HistoryEnabled && !request.Private {
		if err := environment.History.Add(request); err != nil {
			return err
		}
	}
	_ = ctx
	return nil
}

func newEnginesCommand(environment *app.Environment) *cobra.Command {
	var category string
	var all bool
	var jsonOutput bool
	command := &cobra.Command{Use: "engines", Short: "List and inspect search engines", RunE: func(cmd *cobra.Command, args []string) error {
		engines := environment.Catalog.Engines(category, all)
		if jsonOutput {
			return writeJSON(cmd.OutOrStdout(), engines)
		}
		for _, engine := range engines {
			fmt.Fprintf(cmd.OutOrStdout(), "%-26s %-12s %-16s %s\n", engine.ID, engine.Category, engine.Verification.Status, engine.Name)
		}
		return nil
	}}
	command.Flags().StringVarP(&category, "category", "c", "", "filter by category")
	command.Flags().BoolVar(&all, "all", false, "include broken and disabled engines")
	command.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return command
}

func newCategoriesCommand(environment *app.Environment) *cobra.Command {
	return &cobra.Command{Use: "categories", Short: "List search categories and preferred engines", Run: func(cmd *cobra.Command, args []string) {
		for _, category := range environment.Catalog.Categories() {
			preference := environment.Config.Categories[category]
			fmt.Fprintf(cmd.OutOrStdout(), "%-14s preferred=%-24s engines=%d\n", category, preference.DefaultEngine, len(environment.Catalog.Engines(category, false)))
		}
	}}
}

func newPresetsCommand(environment *app.Environment) *cobra.Command {
	return &cobra.Command{Use: "presets", Short: "List search presets", Run: func(cmd *cobra.Command, args []string) {
		for _, preset := range environment.Catalog.Presets("") {
			fmt.Fprintf(cmd.OutOrStdout(), "%-28s %-12s %s\n", preset.ID, preset.Category, preset.Name)
		}
	}}
}

func newSetsCommand(environment *app.Environment) *cobra.Command {
	return &cobra.Command{Use: "sets", Short: "List multi-engine Search Sets", Run: func(cmd *cobra.Command, args []string) {
		for _, set := range environment.Catalog.SearchSets("") {
			fmt.Fprintf(cmd.OutOrStdout(), "%-22s %-12s %s\n", set.ID, set.Category, strings.Join(set.EngineIDs, ", "))
		}
	}}
}

func newHistoryCommand(environment *app.Environment) *cobra.Command {
	command := &cobra.Command{Use: "history", Short: "List local search history", RunE: func(cmd *cobra.Command, args []string) error {
		entries, err := environment.History.List()
		if err != nil {
			return err
		}
		for _, entry := range entries {
			fmt.Fprintf(cmd.OutOrStdout(), "%s  %-12s  %s\n", entry.CreatedAt.Local().Format("2006-01-02 15:04"), strings.Join(entry.EngineIDs, ","), entry.Query)
		}
		return nil
	}}
	command.AddCommand(&cobra.Command{Use: "clear", Short: "Clear local search history", RunE: func(cmd *cobra.Command, args []string) error {
		return environment.History.Clear()
	}})
	return command
}

func newProfilesCommand(environment *app.Environment) *cobra.Command {
	command := &cobra.Command{Use: "profiles", Short: "Manage reusable search profiles", RunE: func(cmd *cobra.Command, args []string) error {
		profiles, err := environment.Library.Profiles()
		if err != nil {
			return err
		}
		for _, profile := range profiles {
			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-24s %s\n", profile.ID, profile.Name, strings.Join(profile.Request.EngineIDs, ","))
		}
		return nil
	}}
	var engine string
	var preset string
	var category string
	var name string
	save := &cobra.Command{Use: "save <id> [query]", Short: "Save or replace a profile", Args: cobra.MinimumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		selectors := make([]string, 0, 3)
		if engine != "" {
			selectors = append(selectors, "@"+engine)
		}
		if category != "" {
			selectors = append(selectors, "#"+category)
		}
		if preset != "" {
			selectors = append(selectors, "+"+preset)
		}
		request, err := environment.Parse(append(selectors, args[1:]...))
		if err != nil {
			return err
		}
		if name == "" {
			name = args[0]
		}
		return environment.Library.SaveProfile(state.Profile{ID: args[0], Name: name, Request: request})
	}}
	save.Flags().StringVar(&engine, "engine", "", "engine ID")
	save.Flags().StringVar(&preset, "preset", "", "preset ID")
	save.Flags().StringVar(&category, "category", "", "category ID")
	save.Flags().StringVar(&name, "name", "", "display name")
	command.AddCommand(save)
	command.AddCommand(&cobra.Command{Use: "delete <id>", Short: "Delete a profile", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return environment.Library.DeleteProfile(args[0])
	}})
	command.AddCommand(&cobra.Command{Use: "run <id> [query]", Short: "Run a saved profile", Args: cobra.MinimumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		profile, ok, err := environment.Library.Profile(args[0])
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("unknown profile %q", args[0])
		}
		request := profile.Request
		if len(args) > 1 {
			request.Query = strings.Join(args[1:], " ")
		}
		urls, err := environment.URLs(request)
		if err != nil {
			return err
		}
		for _, built := range urls {
			if err := environment.Platform.OpenURL(built.URL, environment.Config.DefaultBrowser); err != nil {
				return err
			}
		}
		return nil
	}})
	return command
}

func newFavouritesCommand(environment *app.Environment) *cobra.Command {
	command := &cobra.Command{Use: "favourites", Aliases: []string{"favorites"}, Short: "Manage favourite engines", RunE: func(cmd *cobra.Command, args []string) error {
		values, err := environment.Library.Favourites()
		if err != nil {
			return err
		}
		for _, id := range values {
			if engine, ok := environment.Catalog.Engine(id); ok {
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s %s\n", id, engine.Name)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), id)
			}
		}
		return nil
	}}
	command.AddCommand(&cobra.Command{Use: "add <engine>", Short: "Favourite an engine", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		engine, ok := environment.Catalog.Engine(args[0])
		if !ok {
			return fmt.Errorf("unknown engine %q", args[0])
		}
		return environment.Library.AddFavourite(engine.ID)
	}})
	command.AddCommand(&cobra.Command{Use: "remove <engine>", Short: "Remove a favourite", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		engine, ok := environment.Catalog.Engine(args[0])
		if !ok {
			return fmt.Errorf("unknown engine %q", args[0])
		}
		return environment.Library.RemoveFavourite(engine.ID)
	}})
	return command
}

func newConfigCommand(environment *app.Environment) *cobra.Command {
	command := &cobra.Command{Use: "config", Short: "Inspect and edit SRCH configuration", Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(cmd.OutOrStdout(), filepath.Join(environment.Paths.ConfigDir, "config.toml"))
	}}
	command.AddCommand(&cobra.Command{Use: "init", Short: "Write the default configuration", RunE: func(cmd *cobra.Command, args []string) error {
		return environment.SaveConfig()
	}})
	command.AddCommand(&cobra.Command{Use: "show", Short: "Print effective configuration", RunE: func(cmd *cobra.Command, args []string) error {
		return writeJSON(cmd.OutOrStdout(), environment.Config)
	}})
	command.AddCommand(&cobra.Command{Use: "validate", Short: "Validate configuration and catalogue", RunE: func(cmd *cobra.Command, args []string) error {
		if err := environment.Catalog.Validate(); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "configuration and catalogue are valid")
		return nil
	}})
	command.AddCommand(&cobra.Command{Use: "setup", Short: "Run guided settings", RunE: func(cmd *cobra.Command, args []string) error {
		cfg := &environment.Config
		form := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().Title("Default category").Options(
				huh.NewOption("Web", "web"), huh.NewOption("Images", "images"), huh.NewOption("AI", "ai"), huh.NewOption("Music", "music"), huh.NewOption("Code", "code"), huh.NewOption("Research", "research"),
			).Value(&cfg.DefaultCategory),
			huh.NewSelect[string]().Title("Search input position").Options(
				huh.NewOption("Bottom", "bottom"), huh.NewOption("Top", "top"), huh.NewOption("Automatic", "auto"),
			).Value(&cfg.InputPosition),
			huh.NewSelect[string]().Title("Theme").Options(
				huh.NewOption("Automatic", "auto"), huh.NewOption("Dark", "dark"), huh.NewOption("Light", "light"), huh.NewOption("Monochrome", "monochrome"),
			).Value(&cfg.Theme),
			huh.NewConfirm().Title("Store local search history?").Value(&cfg.HistoryEnabled),
		))
		if err := form.Run(); err != nil {
			return err
		}
		return environment.SaveConfig()
	}})
	return command
}

func newDoctorCommand(environment *app.Environment, version string) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{Use: "doctor", Short: "Diagnose platform and optional integrations", RunE: func(cmd *cobra.Command, args []string) error {
		tools := make(map[string]bool)
		for _, tool := range []string{"defuddle", "glow", "bat", "jq", "curl", "aria2c", "yt-dlp", "ffmpeg"} {
			tools[tool] = environment.Platform.Available(tool)
		}
		report := map[string]any{
			"version": version, "os": runtime.GOOS, "arch": runtime.GOARCH,
			"config_dir": environment.Paths.ConfigDir, "data_dir": environment.Paths.DataDir,
			"cache_dir": environment.Paths.CacheDir, "download_dir": environment.Config.DownloadDir,
			"engines": len(environment.Catalog.Engines("", true)), "tools": tools,
		}
		if jsonOutput {
			return writeJSON(cmd.OutOrStdout(), report)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "srch %s %s/%s\n", version, runtime.GOOS, runtime.GOARCH)
		fmt.Fprintf(cmd.OutOrStdout(), "config    %s\ndata      %s\ncache     %s\ndownloads %s\nengines   %d\n", environment.Paths.ConfigDir, environment.Paths.DataDir, environment.Paths.CacheDir, environment.Config.DownloadDir, report["engines"])
		names := make([]string, 0, len(tools))
		for name := range tools {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			state := "missing"
			if tools[name] {
				state = "available"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-10s %s\n", name, state)
		}
		return nil
	}}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return command
}

func newFetchCommand(environment *app.Environment) *cobra.Command {
	var backend string
	var output string
	var timeout time.Duration
	var maxBytes int64
	var copyCurl bool
	command := &cobra.Command{Use: "fetch <url>", Short: "Fetch a URL and display its content", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if copyCurl {
			line := shellJoin(fetch.CurlCommand(args[0], nil))
			if err := environment.Platform.Copy(line); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), line)
			return nil
		}
		response, err := fetch.Get(cmd.Context(), args[0], fetch.Options{Backend: backend, Timeout: timeout, MaxBytes: maxBytes})
		if err != nil {
			return err
		}
		body, err := fetch.Format(response, output)
		if err != nil {
			return err
		}
		_, err = cmd.OutOrStdout().Write(append(body, '\n'))
		return err
	}}
	command.Flags().StringVar(&backend, "backend", "native", "fetch backend: native or curl")
	command.Flags().StringVarP(&output, "output", "o", "auto", "output mode")
	command.Flags().DurationVar(&timeout, "timeout", 20*time.Second, "request timeout")
	command.Flags().Int64Var(&maxBytes, "max-bytes", 10<<20, "maximum response size")
	command.Flags().BoolVar(&copyCurl, "copy-curl", false, "copy a reproducible Curl command")
	return command
}

func newReadCommand(environment *app.Environment) *cobra.Command {
	var raw bool
	var external string
	var width int
	command := &cobra.Command{Use: "read <url>", Short: "Extract and render readable Markdown", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		markdown, err := reader.Extract(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		if external != "" {
			if !environment.Platform.Available(external) {
				return fmt.Errorf("viewer %q is not installed", external)
			}
			viewer := exec.CommandContext(cmd.Context(), external, "-")
			viewer.Stdin = strings.NewReader(string(markdown))
			viewer.Stdout = cmd.OutOrStdout()
			viewer.Stderr = cmd.ErrOrStderr()
			return viewer.Run()
		}
		if raw {
			_, err = cmd.OutOrStdout().Write(markdown)
			return err
		}
		rendered, err := reader.Render(markdown, width, environment.Config.Theme)
		if err != nil {
			return err
		}
		fmt.Fprint(cmd.OutOrStdout(), rendered)
		return nil
	}}
	command.Flags().BoolVar(&raw, "raw", false, "print extracted Markdown without rendering")
	command.Flags().StringVar(&external, "viewer", "", "external viewer such as glow or bat")
	command.Flags().IntVarP(&width, "width", "w", 100, "render width")
	return command
}

func newDownloadCommand(environment *app.Environment) *cobra.Command {
	var destination string
	var overwrite bool
	var resume bool
	var expectedSHA string
	var dryRun bool
	command := &cobra.Command{Use: "download <url>", Short: "Download a direct HTTP resource", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if destination == "" {
			u, err := url.Parse(args[0])
			if err != nil {
				return err
			}
			name := filepath.Base(u.Path)
			if name == "." || name == "/" || name == "" {
				name = "download"
			}
			destination = filepath.Join(environment.Config.DownloadDir, name)
		}
		if dryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "source: %s\ndestination: %s\nresume: %t\noverwrite: %t\n", args[0], destination, resume, overwrite)
			return nil
		}
		result, err := download.Direct(cmd.Context(), args[0], download.Options{Destination: destination, Overwrite: overwrite, Resume: resume, ExpectedSHA: expectedSHA})
		if err != nil {
			return err
		}
		return writeJSON(cmd.OutOrStdout(), result)
	}}
	command.Flags().StringVarP(&destination, "destination", "d", "", "destination path")
	command.Flags().BoolVar(&overwrite, "overwrite", false, "replace an existing file")
	command.Flags().BoolVar(&resume, "resume", true, "resume a partial download")
	command.Flags().StringVar(&expectedSHA, "sha256", "", "expected SHA-256 checksum")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "show the download plan")
	return command
}

func newMediaCommand(environment *app.Environment) *cobra.Command {
	var audioOnly bool
	var format string
	var output string
	var dryRun bool
	command := &cobra.Command{Use: "media <url>", Short: "Download media through yt-dlp", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !environment.Platform.Available("yt-dlp") {
			return fmt.Errorf("yt-dlp is not installed")
		}
		if output == "" {
			output = filepath.Join(environment.Config.DownloadDir, "media", "%(title)s [%(id)s].%(ext)s")
		}
		ytArgs := []string{"--no-playlist", "--output", output}
		if audioOnly {
			ytArgs = append(ytArgs, "--extract-audio", "--audio-format", "best")
		}
		if format != "" {
			ytArgs = append(ytArgs, "--format", format)
		}
		ytArgs = append(ytArgs, args[0])
		if dryRun {
			fmt.Fprintln(cmd.OutOrStdout(), shellJoin(append([]string{"yt-dlp"}, ytArgs...)))
			return nil
		}
		process := exec.CommandContext(cmd.Context(), "yt-dlp", ytArgs...)
		process.Stdout = cmd.OutOrStdout()
		process.Stderr = cmd.ErrOrStderr()
		process.Stdin = os.Stdin
		return process.Run()
	}}
	command.Flags().BoolVarP(&audioOnly, "audio", "a", false, "extract audio")
	command.Flags().StringVarP(&format, "format", "f", "", "yt-dlp format selector")
	command.Flags().StringVarP(&output, "output", "o", "", "yt-dlp output template")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "show the yt-dlp invocation")
	return command
}

func newAPICommand(environment *app.Environment) *cobra.Command {
	var searchType string
	command := &cobra.Command{Use: "api <adapter> <query>", Short: "Query a supported structured API", Args: cobra.MinimumNArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		adapter, ok := searchapi.Lookup(args[0])
		if !ok {
			return fmt.Errorf("unknown API adapter %q", args[0])
		}
		if searchType == "" {
			searchType = adapter.Types[0].ID
		}
		result, err := searchapi.Search(cmd.Context(), adapter.ID, searchType, strings.Join(args[1:], " "))
		if err != nil {
			return err
		}
		_, err = cmd.OutOrStdout().Write(append(result.Raw, '\n'))
		return err
	}}
	for _, adapter := range searchapi.Adapters() {
		command.ValidArgs = append(command.ValidArgs, adapter.ID)
	}
	command.Flags().StringVar(&searchType, "type", "", "adapter-specific search type (defaults to the first supported type)")
	return command
}

func newCompletionCommand(root *cobra.Command) *cobra.Command {
	return &cobra.Command{Use: "completion <fish|zsh|bash|powershell>", Short: "Generate shell completion", Args: cobra.ExactArgs(1), ValidArgs: []string{"fish", "zsh", "bash", "powershell"}, RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "fish":
			return root.GenFishCompletion(cmd.OutOrStdout(), true)
		case "zsh":
			return root.GenZshCompletion(cmd.OutOrStdout())
		case "bash":
			return root.GenBashCompletionV2(cmd.OutOrStdout(), true)
		case "powershell":
			return root.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
		default:
			return fmt.Errorf("unsupported shell %q", args[0])
		}
	}}
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func readClipboard(environment *app.Environment) (string, error) {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("pbpaste")
	case "windows":
		command = exec.Command("powershell", "-NoProfile", "-Command", "Get-Clipboard")
	default:
		if environment.Platform.Available("wl-paste") {
			command = exec.Command("wl-paste", "--no-newline")
		} else if environment.Platform.Available("xclip") {
			command = exec.Command("xclip", "-selection", "clipboard", "-o")
		} else if environment.Platform.Available("xsel") {
			command = exec.Command("xsel", "--clipboard", "--output")
		}
	}
	if command == nil {
		return "", fmt.Errorf("no clipboard reader found")
	}
	data, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("read clipboard: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func shellJoin(args []string) string {
	quoted := make([]string, len(args))
	for index, arg := range args {
		if arg == "" || strings.ContainsAny(arg, " \t\n\"'\\$`;&|<>()") {
			quoted[index] = "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
		} else {
			quoted[index] = arg
		}
	}
	return strings.Join(quoted, " ")
}
