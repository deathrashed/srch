package catalog

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"srch/internal/domain"
)

//go:embed data/*.yaml
var builtins embed.FS

type dataFile struct {
	Engines       []domain.Engine       `yaml:"engines"`
	Presets       []domain.Preset       `yaml:"presets"`
	SearchSets    []domain.SearchSet    `yaml:"search_sets"`
	LegacyTargets []domain.LegacyTarget `yaml:"legacy_targets"`
}

type Catalog struct {
	engines       map[string]domain.Engine
	presets       map[string]domain.Preset
	searchSets    map[string]domain.SearchSet
	engineAliases map[string]string
	presetAliases map[string]string
	bangs         map[string]string
	legacyTargets map[string]domain.SearchTarget
}

func Load(userPath string) (*Catalog, error) {
	data, err := builtins.ReadFile("data/engines.yaml")
	if err != nil {
		return nil, fmt.Errorf("read embedded catalogue: %w", err)
	}
	var base dataFile
	if err := yaml.Unmarshal(data, &base); err != nil {
		return nil, fmt.Errorf("parse embedded catalogue: %w", err)
	}
	if userPath != "" {
		userData, readErr := os.ReadFile(userPath)
		if readErr == nil {
			var overrides dataFile
			if err := yaml.Unmarshal(userData, &overrides); err != nil {
				return nil, fmt.Errorf("parse user catalogue: %w", err)
			}
			base.Engines = mergeEngines(base.Engines, overrides.Engines)
			base.Presets = mergePresets(base.Presets, overrides.Presets)
			base.SearchSets = mergeSets(base.SearchSets, overrides.SearchSets)
		} else if !errors.Is(readErr, os.ErrNotExist) {
			return nil, fmt.Errorf("read user catalogue: %w", readErr)
		}
	}
	c := &Catalog{
		engines:       make(map[string]domain.Engine),
		presets:       make(map[string]domain.Preset),
		searchSets:    make(map[string]domain.SearchSet),
		engineAliases: make(map[string]string),
		presetAliases: make(map[string]string),
		bangs:         make(map[string]string),
		legacyTargets: make(map[string]domain.SearchTarget),
	}
	for _, engine := range base.Engines {
		c.engines[engine.ID] = engine
		c.engineAliases[normalize(engine.ID)] = engine.ID
		c.engineAliases[normalize(engine.Name)] = engine.ID
		for _, alias := range engine.Aliases {
			c.engineAliases[normalize(alias)] = engine.ID
		}
		for _, bang := range engine.Bangs {
			c.bangs[strings.TrimPrefix(strings.ToLower(bang), "!")] = engine.ID
		}
	}
	for _, preset := range base.Presets {
		c.presets[preset.ID] = preset
		c.presetAliases[normalize(preset.ID)] = preset.ID
		c.presetAliases[normalize(preset.Name)] = preset.ID
		for _, alias := range preset.Aliases {
			c.presetAliases[normalize(alias)] = preset.ID
		}
	}
	for _, set := range base.SearchSets {
		c.searchSets[set.ID] = set
	}
	for _, legacy := range base.LegacyTargets {
		c.legacyTargets[normalize(legacy.ID)] = legacy.Target
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Catalog) Validate() error {
	for id, engine := range c.engines {
		if id == "" || engine.Name == "" || engine.Category == "" || engine.URL.Base == "" {
			return fmt.Errorf("invalid engine %q: id, name, category and URL are required", id)
		}
		if engine.URL.Placement == "" {
			return fmt.Errorf("invalid engine %q: URL placement is required", id)
		}
	}
	for id, preset := range c.presets {
		if _, ok := c.engines[preset.EngineID]; !ok {
			return fmt.Errorf("preset %q references missing engine %q", id, preset.EngineID)
		}
		engine := c.engines[preset.EngineID]
		if preset.TargetID != "" && !hasTarget(engine, preset.TargetID) {
			return fmt.Errorf("preset %q references missing target %q", id, preset.TargetID)
		}
		for groupID, value := range preset.Modifiers {
			binding, ok := engine.Bindings[groupID]
			if !ok {
				return fmt.Errorf("preset %q references unsupported option group %q", id, groupID)
			}
			if len(binding.Values) > 0 {
				if _, ok := binding.Values[value]; !ok {
					return fmt.Errorf("preset %q uses unsupported %s value %q", id, groupID, value)
				}
			}
		}
	}
	for id, set := range c.searchSets {
		for _, engineID := range set.EngineIDs {
			if _, ok := c.engines[engineID]; !ok {
				return fmt.Errorf("search set %q references missing engine %q", id, engineID)
			}
		}
		for _, target := range set.Targets {
			if _, err := c.ResolveTarget(target); err != nil {
				return fmt.Errorf("search set %q: %w", id, err)
			}
		}
	}
	for id, target := range c.legacyTargets {
		if _, err := c.resolveTarget(target, false); err != nil {
			return fmt.Errorf("legacy target %q: %w", id, err)
		}
	}
	return nil
}

func (c *Catalog) Engine(idOrAlias string) (domain.Engine, bool) {
	if replacement, ok := c.legacyTargets[normalize(idOrAlias)]; ok {
		engine, ok := c.Engine(replacement.EngineID)
		if !ok {
			return domain.Engine{}, false
		}
		for _, target := range engine.Targets {
			if target.ID != replacement.TargetID {
				continue
			}
			if engine.URL.Params == nil {
				engine.URL.Params = make(map[string]string)
			}
			for key, value := range target.Params {
				engine.URL.Params[key] = value
			}
			break
		}
		return engine, true
	}
	id := idOrAlias
	if resolved, ok := c.engineAliases[normalize(idOrAlias)]; ok {
		id = resolved
	}
	engine, ok := c.engines[id]
	return engine, ok && !engine.Disabled && engine.Verification.Status != domain.StatusDisabled && engine.Verification.Status != domain.StatusBroken
}

func (c *Catalog) EngineByBang(bang string) (domain.Engine, bool) {
	id, ok := c.bangs[strings.TrimPrefix(strings.ToLower(bang), "!")]
	if !ok {
		return domain.Engine{}, false
	}
	return c.Engine(id)
}

func (c *Catalog) Preset(idOrAlias string) (domain.Preset, bool) {
	id := idOrAlias
	if resolved, ok := c.presetAliases[normalize(idOrAlias)]; ok {
		id = resolved
	}
	preset, ok := c.presets[id]
	return preset, ok
}

func (c *Catalog) SearchSet(id string) (domain.SearchSet, bool) {
	set, ok := c.searchSets[normalize(id)]
	return set, ok
}

func (c *Catalog) Engines(category string, includeUnavailable bool) []domain.Engine {
	items := make([]domain.Engine, 0, len(c.engines))
	for _, engine := range c.engines {
		if category != "" && engine.Category != category {
			continue
		}
		unavailable := engine.Disabled || engine.Verification.Status == domain.StatusBroken || engine.Verification.Status == domain.StatusDisabled
		if unavailable && !includeUnavailable {
			continue
		}
		items = append(items, engine)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Category == items[j].Category {
			return items[i].Name < items[j].Name
		}
		return items[i].Category < items[j].Category
	})
	return items
}

func (c *Catalog) Presets(category string) []domain.Preset {
	items := make([]domain.Preset, 0, len(c.presets))
	for _, preset := range c.presets {
		engine := c.engines[preset.EngineID]
		if category == "" || engine.Category == category {
			items = append(items, preset)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

func (c *Catalog) PresetsForEngine(engineID string) []domain.Preset {
	engine, ok := c.Engine(engineID)
	if !ok {
		return nil
	}
	items := make([]domain.Preset, 0)
	for _, preset := range c.presets {
		if preset.EngineID == engine.ID {
			items = append(items, preset)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

func (c *Catalog) ResolveTarget(target domain.SearchTarget) (domain.ResolvedTarget, error) {
	return c.resolveTarget(target, true)
}

func (c *Catalog) resolveTarget(target domain.SearchTarget, allowLegacy bool) (domain.ResolvedTarget, error) {
	if allowLegacy {
		if replacement, ok := c.legacyTargets[normalize(target.EngineID)]; ok {
			if target.TargetID != "" {
				replacement.TargetID = target.TargetID
			}
			if target.PresetID != "" {
				replacement.PresetID = target.PresetID
			}
			target = replacement
		}
	}
	engine, ok := c.Engine(target.EngineID)
	if !ok {
		return domain.ResolvedTarget{}, fmt.Errorf("unknown engine %q", target.EngineID)
	}
	var preset *domain.Preset
	if target.PresetID != "" {
		value, ok := c.Preset(target.PresetID)
		if !ok {
			return domain.ResolvedTarget{}, fmt.Errorf("unknown preset %q", target.PresetID)
		}
		if value.EngineID != engine.ID {
			return domain.ResolvedTarget{}, fmt.Errorf("preset %q belongs to engine %q, not %q", value.ID, value.EngineID, engine.ID)
		}
		preset = &value
		if target.TargetID == "" {
			target.TargetID = value.TargetID
		}
	}
	var selected domain.EngineTarget
	if target.TargetID != "" {
		for _, candidate := range engine.Targets {
			if candidate.ID == target.TargetID {
				selected = candidate
				break
			}
		}
		if selected.ID == "" {
			return domain.ResolvedTarget{}, fmt.Errorf("engine %q has no target %q", engine.ID, target.TargetID)
		}
	} else if len(engine.Targets) > 0 {
		selected = engine.Targets[0]
	}
	return domain.ResolvedTarget{Engine: engine, Target: selected, Preset: preset}, nil
}

func hasTarget(engine domain.Engine, targetID string) bool {
	for _, target := range engine.Targets {
		if target.ID == targetID {
			return true
		}
	}
	return false
}

func (c *Catalog) SearchSets(category string) []domain.SearchSet {
	items := make([]domain.SearchSet, 0, len(c.searchSets))
	for _, set := range c.searchSets {
		if category == "" || set.Category == category {
			items = append(items, set)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

func (c *Catalog) Categories() []string {
	seen := make(map[string]struct{})
	for _, engine := range c.engines {
		if !engine.Disabled {
			seen[engine.Category] = struct{}{}
		}
	}
	items := make([]string, 0, len(seen))
	for category := range seen {
		items = append(items, category)
	}
	sort.Strings(items)
	return items
}

func normalize(value string) string {
	return strings.Trim(strings.ToLower(strings.ReplaceAll(value, " ", "-")), "!+@#")
}

func mergeEngines(base, overrides []domain.Engine) []domain.Engine {
	byID := make(map[string]domain.Engine, len(base)+len(overrides))
	order := make([]string, 0, len(base)+len(overrides))
	for _, item := range base {
		byID[item.ID] = item
		order = append(order, item.ID)
	}
	for _, item := range overrides {
		if _, exists := byID[item.ID]; !exists {
			order = append(order, item.ID)
		}
		byID[item.ID] = item
	}
	result := make([]domain.Engine, 0, len(order))
	for _, id := range order {
		result = append(result, byID[id])
	}
	return result
}

func mergePresets(base, overrides []domain.Preset) []domain.Preset {
	byID := make(map[string]domain.Preset, len(base)+len(overrides))
	for _, item := range base {
		byID[item.ID] = item
	}
	for _, item := range overrides {
		byID[item.ID] = item
	}
	result := make([]domain.Preset, 0, len(byID))
	for _, item := range byID {
		result = append(result, item)
	}
	return result
}

func mergeSets(base, overrides []domain.SearchSet) []domain.SearchSet {
	byID := make(map[string]domain.SearchSet, len(base)+len(overrides))
	for _, item := range base {
		byID[item.ID] = item
	}
	for _, item := range overrides {
		byID[item.ID] = item
	}
	result := make([]domain.SearchSet, 0, len(byID))
	for _, item := range byID {
		result = append(result, item)
	}
	return result
}
