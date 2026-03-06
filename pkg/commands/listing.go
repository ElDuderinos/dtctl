// Package commands builds a machine-readable catalog of dtctl's command tree.
// It walks the Cobra command hierarchy and produces a structured listing that
// AI agents and MCP servers can use for automated tool registration.
package commands

import (
	"strings"

	"github.com/dynatrace-oss/dtctl/pkg/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// SchemaVersion is incremented on breaking changes to the listing structure.
const SchemaVersion = 1

// Listing is the top-level output of `dtctl commands`.
type Listing struct {
	SchemaVersion int               `json:"schema_version" yaml:"schema_version"`
	Tool          string            `json:"tool" yaml:"tool"`
	Version       string            `json:"version" yaml:"version"`
	Description   string            `json:"description,omitempty" yaml:"description,omitempty"`
	CommandModel  string            `json:"command_model" yaml:"command_model"`
	GlobalFlags   map[string]*Flag  `json:"global_flags,omitempty" yaml:"global_flags,omitempty"`
	Verbs         map[string]*Verb  `json:"verbs" yaml:"verbs"`
	Aliases       map[string]string `json:"resource_aliases,omitempty" yaml:"resource_aliases,omitempty"`
	TimeFormats   *TimeFormats      `json:"time_formats,omitempty" yaml:"time_formats,omitempty"`
	Patterns      []string          `json:"patterns,omitempty" yaml:"patterns,omitempty"`
	Antipatterns  []string          `json:"antipatterns,omitempty" yaml:"antipatterns,omitempty"`
}

// Verb represents a top-level verb (get, describe, apply, ...).
type Verb struct {
	Description  string           `json:"description,omitempty" yaml:"description,omitempty"`
	Mutating     bool             `json:"mutating" yaml:"mutating"`
	SafetyOp     string           `json:"safety_operation,omitempty" yaml:"safety_operation,omitempty"`
	Resources    []string         `json:"resources,omitempty" yaml:"resources,omitempty"`
	Flags        map[string]*Flag `json:"flags,omitempty" yaml:"flags,omitempty"`
	RequiredArgs []string         `json:"required_args,omitempty" yaml:"required_args,omitempty"`
	Subcommands  map[string]*Verb `json:"subcommands,omitempty" yaml:"subcommands,omitempty"`
}

// Flag describes a CLI flag.
type Flag struct {
	Type        string `json:"type" yaml:"type"`
	Default     string `json:"default,omitempty" yaml:"default,omitempty"`
	Required    bool   `json:"required,omitempty" yaml:"required,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// TimeFormats describes the time input formats dtctl accepts.
type TimeFormats struct {
	Relative []string `json:"relative" yaml:"relative"`
	Absolute string   `json:"absolute" yaml:"absolute"`
	Unix     string   `json:"unix" yaml:"unix"`
}

// mutatingVerbs maps verb names to their safety operation type.
// Verbs not listed here are read-only (mutating: false).
var mutatingVerbs = map[string]string{
	"apply":   "OperationCreate",
	"create":  "OperationCreate",
	"edit":    "OperationUpdate",
	"delete":  "OperationDelete",
	"restore": "OperationUpdate",
	"share":   "OperationUpdate",
	"unshare": "OperationUpdate",
	"update":  "OperationUpdate",
	"exec":    "OperationCreate",
}

// resourceAliases are the standard resource aliases built into dtctl.
var resourceAliases = map[string]string{
	"wf":   "workflows",
	"dash": "dashboards",
	"db":   "dashboards",
	"nb":   "notebooks",
	"bkt":  "buckets",
	"ec":   "edgeconnect",
	"fn":   "functions",
	"func": "functions",
}

// hiddenCommands are commands excluded from the listing (internal, utility).
var hiddenCommands = map[string]bool{
	"help":       true,
	"completion": true,
	"commands":   true, // self-referential — not useful in the catalog
	"version":    true, // utility, not an operational command
}

// patterns are recommended usage patterns for AI agents.
var defaultPatterns = []string{
	"Use 'dtctl apply -f' for idempotent resource management",
	"Use 'dtctl diff' before 'dtctl apply' to preview changes",
	"Use 'dtctl query' for ad-hoc DQL queries, not resource-specific flags",
	"Use '--dry-run' to validate apply operations without executing",
	"Use '--agent' for JSON output with operational metadata",
	"Use 'dtctl wait' in CI/CD to poll for conditions",
	"Always specify '--context' in automation scripts",
}

// antipatterns are common mistakes agents should avoid.
var defaultAntipatterns = []string{
	"Don't use 'dtctl create' followed by 'dtctl edit' — use 'dtctl apply -f' instead",
	"Don't parse table output — use '-o json' or '--agent'",
	"Don't hardcode resource IDs — use 'dtctl get' to discover them",
	"Don't skip 'dtctl diff' before 'dtctl apply' in production contexts",
}

var defaultTimeFormats = &TimeFormats{
	Relative: []string{"1h", "30m", "7d", "5min"},
	Absolute: "RFC3339 (e.g., 2024-01-15T10:00:00Z)",
	Unix:     "Unix timestamp (e.g., 1705312800)",
}

// Build walks the Cobra command tree rooted at root and returns a Listing.
func Build(root *cobra.Command) *Listing {
	listing := &Listing{
		SchemaVersion: SchemaVersion,
		Tool:          "dtctl",
		Version:       version.Version,
		Description:   "kubectl-inspired CLI for the Dynatrace platform",
		CommandModel:  "verb-noun",
		GlobalFlags:   buildGlobalFlags(root),
		Verbs:         buildVerbs(root),
		Aliases:       resourceAliases,
		TimeFormats:   defaultTimeFormats,
		Patterns:      defaultPatterns,
		Antipatterns:  defaultAntipatterns,
	}
	return listing
}

// buildGlobalFlags extracts the persistent flags from the root command.
func buildGlobalFlags(root *cobra.Command) map[string]*Flag {
	flags := make(map[string]*Flag)
	root.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		// Skip internal/debug flags
		if f.Hidden {
			return
		}
		key := "--" + f.Name
		flags[key] = &Flag{
			Type:        flagTypeName(f),
			Default:     f.DefValue,
			Description: f.Usage,
		}
	})
	return flags
}

// buildVerbs iterates over the root's direct subcommands and maps them as verbs.
func buildVerbs(root *cobra.Command) map[string]*Verb {
	verbs := make(map[string]*Verb)
	for _, cmd := range root.Commands() {
		name := cmd.Name()
		if hiddenCommands[name] || cmd.Hidden {
			continue
		}

		verb := &Verb{
			Description: cmd.Short,
		}

		// Determine mutating status
		if safetyOp, ok := mutatingVerbs[name]; ok {
			verb.Mutating = true
			verb.SafetyOp = safetyOp
		}

		// Extract resources (subcommands of the verb)
		subs := cmd.Commands()
		if len(subs) > 0 {
			var resources []string
			subcommands := make(map[string]*Verb)
			hasResources := false

			for _, sub := range subs {
				if sub.Hidden || sub.Name() == "help" {
					continue
				}

				subName := sub.Name()

				// Check if this subcommand itself has subcommands (nested, like exec copilot)
				nestedSubs := sub.Commands()
				hasNested := false
				for _, ns := range nestedSubs {
					if !ns.Hidden && ns.Name() != "help" {
						hasNested = true
						break
					}
				}

				if hasNested {
					// Nested subcommand (e.g., exec copilot -> nl2dql, dql2nl, ...)
					subVerb := &Verb{
						Description: sub.Short,
					}
					if safetyOp, ok := mutatingVerbs[name]; ok {
						subVerb.Mutating = true
						subVerb.SafetyOp = safetyOp
					}
					nestedNames := make(map[string]*Verb)
					for _, ns := range nestedSubs {
						if ns.Hidden || ns.Name() == "help" {
							continue
						}
						nestedNames[ns.Name()] = &Verb{
							Description: ns.Short,
						}
					}
					if len(nestedNames) > 0 {
						subVerb.Subcommands = nestedNames
					}

					// Collect flags from the subcommand
					subFlags := collectLocalFlags(sub)
					if len(subFlags) > 0 {
						subVerb.Flags = subFlags
					}

					subcommands[subName] = subVerb
				} else {
					// Treat as a resource
					resources = append(resources, subName)
					hasResources = true
				}
			}

			if hasResources {
				verb.Resources = resources
			}
			if len(subcommands) > 0 {
				verb.Subcommands = subcommands
			}
		}

		// Extract verb-level flags (local flags, not from subcommands)
		verbFlags := collectLocalFlags(cmd)
		if len(verbFlags) > 0 {
			verb.Flags = verbFlags
		}

		// Extract required args from Use string (e.g., "apply -f <file>")
		if args := parseRequiredArgs(cmd.Use); len(args) > 0 {
			verb.RequiredArgs = args
		}

		verbs[name] = verb
	}
	return verbs
}

// collectLocalFlags extracts non-persistent, non-hidden flags from a command.
func collectLocalFlags(cmd *cobra.Command) map[string]*Flag {
	flags := make(map[string]*Flag)
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		// Skip flags that are inherited from parent (persistent flags)
		if cmd.InheritedFlags().Lookup(f.Name) != nil {
			return
		}

		key := "--" + f.Name
		if f.Shorthand != "" {
			key = "-" + f.Shorthand + "/" + key
		}

		fl := &Flag{
			Type:        flagTypeName(f),
			Default:     f.DefValue,
			Description: f.Usage,
		}

		// Mark as required if annotated
		if ann := f.Annotations; ann != nil {
			if _, ok := ann[cobra.BashCompOneRequiredFlag]; ok {
				fl.Required = true
			}
		}

		flags[key] = fl
	})
	return flags
}

// parseRequiredArgs extracts angle-bracket args from a Use string.
// e.g., "apply -f <file>" → ["file"]
func parseRequiredArgs(use string) []string {
	var args []string
	for _, part := range strings.Fields(use) {
		if strings.HasPrefix(part, "<") && strings.HasSuffix(part, ">") {
			args = append(args, strings.Trim(part, "<>"))
		}
	}
	return args
}

// flagTypeName returns a human-readable type name for a flag.
func flagTypeName(f *pflag.Flag) string {
	switch f.Value.Type() {
	case "string":
		return "string"
	case "bool":
		return "boolean"
	case "int", "int32", "int64":
		return "integer"
	case "float32", "float64":
		return "number"
	case "stringSlice", "stringArray":
		return "string[]"
	case "duration":
		return "duration"
	case "count":
		return "integer"
	default:
		return f.Value.Type()
	}
}

// ApplyBrief strips verbose fields from a listing for reduced token count.
// It preserves mutating status since agents always need it.
func ApplyBrief(l *Listing) {
	l.Description = ""
	l.GlobalFlags = nil
	l.TimeFormats = nil
	l.Patterns = nil
	l.Antipatterns = nil

	// Rename resource_aliases to aliases in brief mode
	// (handled by JSON field tag — we just keep it)

	for _, verb := range l.Verbs {
		verb.Description = ""
		verb.SafetyOp = ""
		verb.RequiredArgs = nil

		// Simplify flags: just type, drop description/default
		if verb.Flags != nil {
			for _, f := range verb.Flags {
				desc := ""
				if f.Required {
					desc = " (required)"
				}
				f.Description = ""
				f.Default = ""
				if desc != "" {
					f.Description = strings.TrimSpace(desc)
				}
			}
		}

		// Recurse into subcommands
		for _, sub := range verb.Subcommands {
			sub.Description = ""
			sub.SafetyOp = ""
			sub.RequiredArgs = nil
			for _, f := range sub.Flags {
				f.Description = ""
				f.Default = ""
			}
			for _, nested := range sub.Subcommands {
				nested.Description = ""
				nested.SafetyOp = ""
			}
		}
	}
}

// FilterByResource filters the listing to only include verbs that operate on
// the given resource name. The name is matched against resources, subcommands,
// and aliases. Returns true if any verbs matched.
func FilterByResource(l *Listing, name string) bool {
	// Resolve alias
	resolved := name
	if target, ok := resourceAliases[name]; ok {
		resolved = target
	}

	// Check if it's a verb name first
	if _, ok := l.Verbs[name]; ok {
		// Filter to just this verb
		verb := l.Verbs[name]
		l.Verbs = map[string]*Verb{name: verb}
		return true
	}

	// Filter to verbs that contain the resource
	filtered := make(map[string]*Verb)
	for verbName, verb := range l.Verbs {
		if containsResource(verb, resolved) || containsResource(verb, name) {
			filtered[verbName] = verb
		}
	}

	if len(filtered) == 0 {
		return false
	}

	l.Verbs = filtered
	return true
}

// containsResource checks if a verb or its subcommands reference a resource name.
func containsResource(verb *Verb, name string) bool {
	// Check resources list
	for _, r := range verb.Resources {
		if r == name || r == name+"s" || r+"s" == name {
			return true
		}
	}
	// Check subcommands
	for subName := range verb.Subcommands {
		if subName == name || subName == name+"s" || subName+"s" == name {
			return true
		}
	}
	return false
}

// ResolveAlias resolves a resource alias to its canonical name.
// Returns the original name if no alias exists.
func ResolveAlias(name string) string {
	if target, ok := resourceAliases[name]; ok {
		return target
	}
	return name
}
