package commands

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// newTestRoot creates a minimal Cobra command tree for testing.
func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "dtctl"}
	root.PersistentFlags().StringP("output", "o", "table", "output format")
	root.PersistentFlags().BoolP("agent", "A", false, "agent mode")
	root.PersistentFlags().Bool("dry-run", false, "preview")

	// get verb with resources
	get := &cobra.Command{Use: "get", Short: "List resources"}
	get.AddCommand(&cobra.Command{Use: "workflows", Short: "List workflows", Aliases: []string{"wf"}})
	get.AddCommand(&cobra.Command{Use: "dashboards", Short: "List dashboards", Aliases: []string{"dash", "db"}})
	get.AddCommand(&cobra.Command{Use: "notebooks", Short: "List notebooks", Aliases: []string{"nb"}})
	root.AddCommand(get)

	// describe verb
	describe := &cobra.Command{Use: "describe", Short: "Show details"}
	describe.AddCommand(&cobra.Command{Use: "workflow", Short: "Describe workflow"})
	describe.AddCommand(&cobra.Command{Use: "dashboard", Short: "Describe dashboard"})
	root.AddCommand(describe)

	// apply verb (mutating, with flags)
	apply := &cobra.Command{Use: "apply", Short: "Apply configuration"}
	apply.Flags().StringP("file", "f", "", "YAML/JSON file path")
	_ = apply.MarkFlagRequired("file")
	apply.Flags().StringArray("set", nil, "Template variables")
	apply.Flags().Bool("show-diff", false, "Show diff")
	root.AddCommand(apply)

	// delete verb (mutating, with resources)
	del := &cobra.Command{Use: "delete", Short: "Delete resources"}
	del.AddCommand(&cobra.Command{Use: "workflow", Short: "Delete a workflow"})
	del.AddCommand(&cobra.Command{Use: "dashboard", Short: "Delete a dashboard"})
	root.AddCommand(del)

	// exec verb (mutating, with nested subcommands)
	exec := &cobra.Command{Use: "exec", Short: "Execute commands"}
	exec.AddCommand(&cobra.Command{Use: "workflow", Short: "Run a workflow"})
	copilot := &cobra.Command{Use: "copilot", Short: "Chat with copilot"}
	copilot.AddCommand(&cobra.Command{Use: "nl2dql", Short: "NL to DQL"})
	copilot.AddCommand(&cobra.Command{Use: "dql2nl", Short: "DQL to NL"})
	exec.AddCommand(copilot)
	root.AddCommand(exec)

	// doctor (read-only, no resources)
	root.AddCommand(&cobra.Command{Use: "doctor", Short: "Health check"})

	// commands (should be excluded)
	root.AddCommand(&cobra.Command{Use: "commands", Short: "List commands"})

	// version (should be excluded)
	root.AddCommand(&cobra.Command{Use: "version", Short: "Print version"})

	// completion (should be excluded)
	root.AddCommand(&cobra.Command{Use: "completion", Short: "Shell completions"})

	// help is auto-added by Cobra

	return root
}

func TestBuild_SchemaVersion(t *testing.T) {
	root := newTestRoot()
	listing := Build(root)

	require.Equal(t, SchemaVersion, listing.SchemaVersion)
	require.Equal(t, "dtctl", listing.Tool)
	require.Equal(t, "verb-noun", listing.CommandModel)
}

func TestBuild_GlobalFlags(t *testing.T) {
	root := newTestRoot()
	listing := Build(root)

	require.NotNil(t, listing.GlobalFlags)
	require.Contains(t, listing.GlobalFlags, "--output")
	require.Contains(t, listing.GlobalFlags, "--agent")
	require.Contains(t, listing.GlobalFlags, "--dry-run")

	outputFlag := listing.GlobalFlags["--output"]
	require.Equal(t, "string", outputFlag.Type)
	require.Equal(t, "table", outputFlag.Default)
}

func TestBuild_HiddenCommands(t *testing.T) {
	root := newTestRoot()
	listing := Build(root)

	require.NotContains(t, listing.Verbs, "commands", "commands should be excluded from listing")
	require.NotContains(t, listing.Verbs, "version", "version should be excluded from listing")
	require.NotContains(t, listing.Verbs, "completion", "completion should be excluded from listing")
	require.NotContains(t, listing.Verbs, "help", "help should be excluded from listing")
}

func TestBuild_VerbsPresent(t *testing.T) {
	root := newTestRoot()
	listing := Build(root)

	expectedVerbs := []string{"get", "describe", "apply", "delete", "exec", "doctor"}
	for _, v := range expectedVerbs {
		require.Contains(t, listing.Verbs, v, "verb %q should be present", v)
	}
}

func TestBuild_MutatingVerbs(t *testing.T) {
	root := newTestRoot()
	listing := Build(root)

	tests := []struct {
		verb     string
		mutating bool
		safetyOp string
	}{
		{"get", false, ""},
		{"describe", false, ""},
		{"doctor", false, ""},
		{"apply", true, "OperationCreate"},
		{"delete", true, "OperationDelete"},
		{"exec", true, "OperationCreate"},
	}

	for _, tt := range tests {
		t.Run(tt.verb, func(t *testing.T) {
			verb, ok := listing.Verbs[tt.verb]
			require.True(t, ok, "verb %q should exist", tt.verb)
			require.Equal(t, tt.mutating, verb.Mutating, "verb %q mutating", tt.verb)
			require.Equal(t, tt.safetyOp, verb.SafetyOp, "verb %q safety_operation", tt.verb)
		})
	}
}

func TestBuild_Resources(t *testing.T) {
	root := newTestRoot()
	listing := Build(root)

	getVerb := listing.Verbs["get"]
	require.ElementsMatch(t, []string{"workflows", "dashboards", "notebooks"}, getVerb.Resources)

	deleteVerb := listing.Verbs["delete"]
	require.ElementsMatch(t, []string{"workflow", "dashboard"}, deleteVerb.Resources)
}

func TestBuild_VerbFlags(t *testing.T) {
	root := newTestRoot()
	listing := Build(root)

	applyVerb := listing.Verbs["apply"]
	require.NotNil(t, applyVerb.Flags)
	require.Contains(t, applyVerb.Flags, "-f/--file")
	require.Contains(t, applyVerb.Flags, "--show-diff")
	require.Contains(t, applyVerb.Flags, "--set")

	fileFlag := applyVerb.Flags["-f/--file"]
	require.Equal(t, "string", fileFlag.Type)
}

func TestBuild_NestedSubcommands(t *testing.T) {
	root := newTestRoot()
	listing := Build(root)

	execVerb := listing.Verbs["exec"]
	require.Contains(t, execVerb.Resources, "workflow")
	require.NotNil(t, execVerb.Subcommands)
	require.Contains(t, execVerb.Subcommands, "copilot")

	copilot := execVerb.Subcommands["copilot"]
	require.NotNil(t, copilot.Subcommands)
	require.Contains(t, copilot.Subcommands, "nl2dql")
	require.Contains(t, copilot.Subcommands, "dql2nl")
}

func TestBuild_ResourceAliases(t *testing.T) {
	root := newTestRoot()
	listing := Build(root)

	require.NotNil(t, listing.Aliases)
	require.Equal(t, "workflows", listing.Aliases["wf"])
	require.Equal(t, "dashboards", listing.Aliases["dash"])
	require.Equal(t, "dashboards", listing.Aliases["db"])
	require.Equal(t, "notebooks", listing.Aliases["nb"])
}

func TestBuild_TimeFormats(t *testing.T) {
	root := newTestRoot()
	listing := Build(root)

	require.NotNil(t, listing.TimeFormats)
	require.NotEmpty(t, listing.TimeFormats.Relative)
	require.NotEmpty(t, listing.TimeFormats.Absolute)
	require.NotEmpty(t, listing.TimeFormats.Unix)
}

func TestBuild_PatternsAndAntipatterns(t *testing.T) {
	root := newTestRoot()
	listing := Build(root)

	require.NotEmpty(t, listing.Patterns)
	require.NotEmpty(t, listing.Antipatterns)
}

func TestApplyBrief(t *testing.T) {
	root := newTestRoot()
	listing := Build(root)

	ApplyBrief(listing)

	// Stripped fields
	require.Empty(t, listing.Description)
	require.Nil(t, listing.GlobalFlags)
	require.Nil(t, listing.TimeFormats)
	require.Nil(t, listing.Patterns)
	require.Nil(t, listing.Antipatterns)

	// Preserved fields
	require.Equal(t, SchemaVersion, listing.SchemaVersion)
	require.Equal(t, "dtctl", listing.Tool)
	require.NotEmpty(t, listing.Verbs)
	require.NotNil(t, listing.Aliases)

	// Verb descriptions stripped but mutating preserved
	applyVerb := listing.Verbs["apply"]
	require.Empty(t, applyVerb.Description)
	require.True(t, applyVerb.Mutating, "mutating should be preserved in brief mode")
	require.Empty(t, applyVerb.SafetyOp, "safety_operation should be stripped in brief mode")

	getVerb := listing.Verbs["get"]
	require.False(t, getVerb.Mutating, "read-only verb should remain false")
}

func TestFilterByResource(t *testing.T) {
	tests := []struct {
		name        string
		filter      string
		expectMatch bool
		expectVerbs []string
	}{
		{
			name:        "filter by resource name",
			filter:      "workflows",
			expectMatch: true,
			// Matches "workflows" in get, and "workflow" (singular match) in describe/delete/exec
			expectVerbs: []string{"get", "describe", "delete", "exec"},
		},
		{
			name:        "filter by singular resource",
			filter:      "workflow",
			expectMatch: true,
			// Matches "workflow" in describe/delete/exec, and "workflows" (plural match) in get
			expectVerbs: []string{"get", "delete", "describe", "exec"},
		},
		{
			name:        "filter by alias",
			filter:      "wf",
			expectMatch: true,
			// Alias resolves to "workflows", same result as filtering by "workflows"
			expectVerbs: []string{"get", "describe", "delete", "exec"},
		},
		{
			name:        "filter by verb name",
			filter:      "get",
			expectMatch: true,
			expectVerbs: []string{"get"},
		},
		{
			name:        "filter by nonexistent resource",
			filter:      "nonexistent",
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := newTestRoot()
			listing := Build(root)

			matched := FilterByResource(listing, tt.filter)
			require.Equal(t, tt.expectMatch, matched)

			if tt.expectMatch {
				var verbNames []string
				for name := range listing.Verbs {
					verbNames = append(verbNames, name)
				}
				require.ElementsMatch(t, tt.expectVerbs, verbNames)
			}
		})
	}
}

func TestResolveAlias(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"wf", "workflows"},
		{"dash", "dashboards"},
		{"db", "dashboards"},
		{"nb", "notebooks"},
		{"bkt", "buckets"},
		{"ec", "edgeconnect"},
		{"fn", "functions"},
		{"func", "functions"},
		{"workflows", "workflows"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			require.Equal(t, tt.expected, ResolveAlias(tt.input))
		})
	}
}

func TestFlagTypeName(t *testing.T) {
	root := &cobra.Command{Use: "test"}
	root.Flags().String("s", "", "string flag")
	root.Flags().Bool("b", false, "bool flag")
	root.Flags().Int("i", 0, "int flag")
	root.Flags().StringSlice("ss", nil, "string slice flag")
	root.Flags().Duration("d", 0, "duration flag")

	tests := []struct {
		flagName string
		expected string
	}{
		{"s", "string"},
		{"b", "boolean"},
		{"i", "integer"},
		{"ss", "string[]"},
		{"d", "duration"},
	}

	for _, tt := range tests {
		t.Run(tt.flagName, func(t *testing.T) {
			f := root.Flags().Lookup(tt.flagName)
			require.NotNil(t, f)
			require.Equal(t, tt.expected, flagTypeName(f))
		})
	}
}

func TestParseRequiredArgs(t *testing.T) {
	tests := []struct {
		use      string
		expected []string
	}{
		{"apply -f <file>", []string{"file"}},
		{"describe <resource> <id>", []string{"resource", "id"}},
		{"get", nil},
		{"query [dql-string]", nil}, // brackets, not angles
	}

	for _, tt := range tests {
		t.Run(tt.use, func(t *testing.T) {
			result := parseRequiredArgs(tt.use)
			if tt.expected == nil {
				require.Nil(t, result)
			} else {
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestBuild_HiddenCobraCommand(t *testing.T) {
	root := &cobra.Command{Use: "dtctl"}
	root.AddCommand(&cobra.Command{Use: "visible", Short: "Visible"})
	hidden := &cobra.Command{Use: "secret", Short: "Secret", Hidden: true}
	root.AddCommand(hidden)

	listing := Build(root)
	require.Contains(t, listing.Verbs, "visible")
	require.NotContains(t, listing.Verbs, "secret")
}
