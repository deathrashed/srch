package domain

import "time"

type Action string

const (
	ActionOpen     Action = "open"
	ActionPrint    Action = "print"
	ActionCopy     Action = "copy"
	ActionFetch    Action = "fetch"
	ActionRead     Action = "read"
	ActionDownload Action = "download"
)

type OutputMode string

const (
	OutputPlain    OutputMode = "plain"
	OutputJSON     OutputMode = "json"
	OutputJSONL    OutputMode = "jsonl"
	OutputMarkdown OutputMode = "markdown"
	OutputTSV      OutputMode = "tsv"
)

type Placement string

const (
	PlacementQuery    Placement = "query"
	PlacementPath     Placement = "path"
	PlacementFragment Placement = "fragment"
	PlacementFixed    Placement = "fixed"
)

type VerificationStatus string

const (
	StatusVerified      VerificationStatus = "verified"
	StatusReachable     VerificationStatus = "reachable"
	StatusBestEffort    VerificationStatus = "best_effort"
	StatusRequiresLogin VerificationStatus = "requires_login"
	StatusRegional      VerificationStatus = "regional"
	StatusAPI           VerificationStatus = "api"
	StatusBroken        VerificationStatus = "broken"
	StatusDisabled      VerificationStatus = "disabled"
)

type Verification struct {
	Status     VerificationStatus `yaml:"status" json:"status"`
	VerifiedAt string             `yaml:"verified_at,omitempty" json:"verified_at,omitempty"`
	Note       string             `yaml:"note,omitempty" json:"note,omitempty"`
}

type URLSpec struct {
	Base       string            `yaml:"base" json:"base"`
	Placement  Placement         `yaml:"placement" json:"placement"`
	QueryParam string            `yaml:"query_param,omitempty" json:"query_param,omitempty"`
	Template   string            `yaml:"template,omitempty" json:"template,omitempty"`
	Params     map[string]string `yaml:"params,omitempty" json:"params,omitempty"`
}

type CapabilityConfidence string

const (
	ConfidenceDocumented      CapabilityConfidence = "documented"
	ConfidenceObserved        CapabilityConfidence = "observed"
	ConfidenceDestinationOnly CapabilityConfidence = "destination_only"
)

type OptionKind string

const (
	OptionSelect  OptionKind = "select"
	OptionMulti   OptionKind = "multi"
	OptionBoolean OptionKind = "boolean"
	OptionRange   OptionKind = "range"
	OptionField   OptionKind = "field"
)

type Option struct {
	ID    string `yaml:"id" json:"id"`
	Name  string `yaml:"name" json:"name"`
	Value string `yaml:"value,omitempty" json:"value,omitempty"`
}

type OptionGroup struct {
	ID         string               `yaml:"id" json:"id"`
	Name       string               `yaml:"name" json:"name"`
	Kind       OptionKind           `yaml:"kind" json:"kind"`
	Options    []Option             `yaml:"options,omitempty" json:"options,omitempty"`
	Confidence CapabilityConfidence `yaml:"confidence,omitempty" json:"confidence,omitempty"`
}

type Field struct {
	ID          string `yaml:"id" json:"id"`
	Name        string `yaml:"name" json:"name"`
	Placeholder string `yaml:"placeholder,omitempty" json:"placeholder,omitempty"`
	Required    bool   `yaml:"required,omitempty" json:"required,omitempty"`
}

type EngineTarget struct {
	ID           string            `yaml:"id" json:"id"`
	Name         string            `yaml:"name" json:"name"`
	QueryParam   string            `yaml:"query_param,omitempty" json:"query_param,omitempty"`
	Params       map[string]string `yaml:"params,omitempty" json:"params,omitempty"`
	Fields       []Field           `yaml:"fields,omitempty" json:"fields,omitempty"`
	OptionGroups []OptionGroup     `yaml:"option_groups,omitempty" json:"option_groups,omitempty"`
}

type ModifierBinding struct {
	Param    string            `yaml:"param,omitempty" json:"param,omitempty"`
	Template string            `yaml:"template,omitempty" json:"template,omitempty"`
	Join     string            `yaml:"join,omitempty" json:"join,omitempty"`
	Values   map[string]string `yaml:"values,omitempty" json:"values,omitempty"`
}

type Engine struct {
	ID           string                     `yaml:"id" json:"id"`
	Name         string                     `yaml:"name" json:"name"`
	Aliases      []string                   `yaml:"aliases,omitempty" json:"aliases,omitempty"`
	Bangs        []string                   `yaml:"bangs,omitempty" json:"bangs,omitempty"`
	Category     string                     `yaml:"category" json:"category"`
	Tags         []string                   `yaml:"tags,omitempty" json:"tags,omitempty"`
	InputKind    string                     `yaml:"input_kind,omitempty" json:"input_kind,omitempty"`
	URL          URLSpec                    `yaml:"url" json:"url"`
	Capabilities []string                   `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
	Targets      []EngineTarget             `yaml:"targets,omitempty" json:"targets,omitempty"`
	Bindings     map[string]ModifierBinding `yaml:"bindings,omitempty" json:"bindings,omitempty"`
	Confidence   CapabilityConfidence       `yaml:"confidence,omitempty" json:"confidence,omitempty"`
	Verification Verification               `yaml:"verification" json:"verification"`
	Disabled     bool                       `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	Meta         map[string]string          `yaml:"meta,omitempty" json:"meta,omitempty"`
}

type Preset struct {
	ID          string            `yaml:"id" json:"id"`
	Name        string            `yaml:"name" json:"name"`
	Category    string            `yaml:"category,omitempty" json:"category,omitempty"` // legacy input; category is derived from EngineID
	EngineID    string            `yaml:"engine" json:"engine"`
	TargetID    string            `yaml:"target,omitempty" json:"target,omitempty"`
	Aliases     []string          `yaml:"aliases,omitempty" json:"aliases,omitempty"`
	QuerySuffix string            `yaml:"query_suffix,omitempty" json:"query_suffix,omitempty"`
	Params      map[string]string `yaml:"params,omitempty" json:"params,omitempty"`
	Modifiers   map[string]string `yaml:"modifiers,omitempty" json:"modifiers,omitempty"`
}

type SearchTarget struct {
	EngineID string `yaml:"engine" json:"engine"`
	TargetID string `yaml:"target,omitempty" json:"target,omitempty"`
	PresetID string `yaml:"preset,omitempty" json:"preset,omitempty"`
}

type LegacyTarget struct {
	ID     string       `yaml:"id" json:"id"`
	Target SearchTarget `yaml:"target" json:"target"`
}

type ResolvedTarget struct {
	Engine Engine
	Target EngineTarget
	Preset *Preset
}

type SearchSet struct {
	ID        string         `yaml:"id" json:"id"`
	Name      string         `yaml:"name" json:"name"`
	Category  string         `yaml:"category" json:"category"`
	EngineIDs []string       `yaml:"engines" json:"engines"`
	Targets   []SearchTarget `yaml:"targets,omitempty" json:"targets,omitempty"`
}

type Modifiers struct {
	Exact    bool              `json:"exact,omitempty"`
	Exclude  []string          `json:"exclude,omitempty"`
	Values   map[string]string `json:"values,omitempty"`
	RawQuery map[string]string `json:"raw_query,omitempty"`
}

type SearchRequest struct {
	Query       string         `json:"query"`
	CategoryID  string         `json:"category,omitempty"`
	EngineIDs   []string       `json:"engines,omitempty"`
	Targets     []SearchTarget `json:"targets,omitempty"`
	PresetID    string         `json:"preset,omitempty"`
	SearchSetID string         `json:"search_set,omitempty"`
	Modifiers   Modifiers      `json:"modifiers,omitempty"`
	Action      Action         `json:"action"`
	Output      OutputMode     `json:"output"`
	Private     bool           `json:"private,omitempty"`
}

type Result struct {
	Title       string            `json:"title"`
	URL         string            `json:"url,omitempty"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Raw         []byte            `json:"-"`
}

type HistoryEntry struct {
	ID        string    `json:"id"`
	Query     string    `json:"query"`
	EngineIDs []string  `json:"engines"`
	PresetID  string    `json:"preset,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
