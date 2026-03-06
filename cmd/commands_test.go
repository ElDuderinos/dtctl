package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtctl/pkg/commands"
	"github.com/stretchr/testify/require"
)

func TestCommandsCmd_OutputsValidJSON(t *testing.T) {
	listing := commands.Build(rootCmd)

	data, err := json.Marshal(listing)
	require.NoError(t, err)
	require.True(t, json.Valid(data), "output should be valid JSON")
}

func TestCommandsCmd_SchemaVersion(t *testing.T) {
	listing := commands.Build(rootCmd)

	require.Equal(t, commands.SchemaVersion, listing.SchemaVersion)
	require.Equal(t, "dtctl", listing.Tool)
	require.Equal(t, "verb-noun", listing.CommandModel)
	require.NotEmpty(t, listing.Version)
}

func TestCommandsCmd_AllVerbsPresent(t *testing.T) {
	listing := commands.Build(rootCmd)

	// Core verbs that must always be present
	expectedVerbs := []string{
		"get", "describe", "apply", "create", "edit", "delete",
		"exec", "diff", "query", "wait", "doctor", "history",
		"restore", "share", "unshare", "logs", "ctx",
	}

	for _, verb := range expectedVerbs {
		require.Contains(t, listing.Verbs, verb, "verb %q should be present", verb)
	}
}

func TestCommandsCmd_ExcludesUtilityCommands(t *testing.T) {
	listing := commands.Build(rootCmd)

	excluded := []string{"commands", "version", "completion", "help"}
	for _, cmd := range excluded {
		require.NotContains(t, listing.Verbs, cmd, "%q should be excluded from listing", cmd)
	}
}

func TestCommandsCmd_MutatingVerbsCorrect(t *testing.T) {
	listing := commands.Build(rootCmd)

	mutatingVerbs := map[string]bool{
		"apply":   true,
		"create":  true,
		"edit":    true,
		"delete":  true,
		"exec":    true,
		"restore": true,
		"share":   true,
		"unshare": true,
		"update":  true,
	}

	readOnlyVerbs := []string{
		"get", "describe", "diff", "query", "wait", "doctor",
		"history", "logs", "ctx", "find", "verify", "open",
	}

	for verb, expected := range mutatingVerbs {
		v, ok := listing.Verbs[verb]
		if !ok {
			continue // verb may not be registered yet
		}
		require.Equal(t, expected, v.Mutating, "verb %q should be mutating=%v", verb, expected)
		if expected {
			require.NotEmpty(t, v.SafetyOp, "mutating verb %q should have safety_operation", verb)
		}
	}

	for _, verb := range readOnlyVerbs {
		v, ok := listing.Verbs[verb]
		if !ok {
			continue
		}
		require.False(t, v.Mutating, "verb %q should not be mutating", verb)
		require.Empty(t, v.SafetyOp, "read-only verb %q should not have safety_operation", verb)
	}
}

func TestCommandsCmd_GetHasResources(t *testing.T) {
	listing := commands.Build(rootCmd)

	getVerb := listing.Verbs["get"]
	require.NotNil(t, getVerb)
	require.NotEmpty(t, getVerb.Resources, "get verb should have resources")

	// Spot-check some expected resources
	require.Contains(t, getVerb.Resources, "workflows")
	require.Contains(t, getVerb.Resources, "dashboards")
	require.Contains(t, getVerb.Resources, "slos")
	require.Contains(t, getVerb.Resources, "settings")
	require.Contains(t, getVerb.Resources, "buckets")
}

func TestCommandsCmd_ResourceAliases(t *testing.T) {
	listing := commands.Build(rootCmd)

	require.NotNil(t, listing.Aliases)
	require.Equal(t, "workflows", listing.Aliases["wf"])
	require.Equal(t, "dashboards", listing.Aliases["dash"])
}

func TestCommandsCmd_TimeFormats(t *testing.T) {
	listing := commands.Build(rootCmd)

	require.NotNil(t, listing.TimeFormats)
	require.NotEmpty(t, listing.TimeFormats.Relative)
	require.NotEmpty(t, listing.TimeFormats.Absolute)
	require.NotEmpty(t, listing.TimeFormats.Unix)
}

func TestCommandsCmd_BriefReducesSize(t *testing.T) {
	listing := commands.Build(rootCmd)
	fullData, err := json.Marshal(listing)
	require.NoError(t, err)

	briefListing := commands.Build(rootCmd)
	commands.ApplyBrief(briefListing)
	briefData, err := json.Marshal(briefListing)
	require.NoError(t, err)

	// Brief should be significantly smaller
	reduction := 1.0 - float64(len(briefData))/float64(len(fullData))
	require.Greater(t, reduction, 0.30, "brief mode should reduce size by at least 30%%, got %.0f%%", reduction*100)
}

func TestCommandsCmd_FilterByAlias(t *testing.T) {
	listingByName := commands.Build(rootCmd)
	listingByAlias := commands.Build(rootCmd)

	matchedName := commands.FilterByResource(listingByName, "workflows")
	matchedAlias := commands.FilterByResource(listingByAlias, "wf")

	require.True(t, matchedName)
	require.True(t, matchedAlias)

	// Same verbs should match
	var nameVerbs, aliasVerbs []string
	for v := range listingByName.Verbs {
		nameVerbs = append(nameVerbs, v)
	}
	for v := range listingByAlias.Verbs {
		aliasVerbs = append(aliasVerbs, v)
	}
	require.ElementsMatch(t, nameVerbs, aliasVerbs)
}

func TestCommandsCmd_FilterByVerb(t *testing.T) {
	listing := commands.Build(rootCmd)
	matched := commands.FilterByResource(listing, "get")

	require.True(t, matched)
	require.Len(t, listing.Verbs, 1)
	require.Contains(t, listing.Verbs, "get")
}

func TestCommandsCmd_FilterNoMatch(t *testing.T) {
	listing := commands.Build(rootCmd)
	matched := commands.FilterByResource(listing, "nonexistent-resource")

	require.False(t, matched)
}

func TestCommandsCmd_GlobalFlags(t *testing.T) {
	listing := commands.Build(rootCmd)

	require.NotNil(t, listing.GlobalFlags)
	require.Contains(t, listing.GlobalFlags, "--output")
	require.Contains(t, listing.GlobalFlags, "--agent")
	require.Contains(t, listing.GlobalFlags, "--dry-run")
	require.Contains(t, listing.GlobalFlags, "--context")
	require.Contains(t, listing.GlobalFlags, "--plain")
	require.Contains(t, listing.GlobalFlags, "--chunk-size")
}

func TestCommandsCmd_ExecHasSubcommands(t *testing.T) {
	listing := commands.Build(rootCmd)

	execVerb := listing.Verbs["exec"]
	require.NotNil(t, execVerb)
	require.NotEmpty(t, execVerb.Resources, "exec should have resources")

	// copilot should be a subcommand (has nested commands)
	require.NotNil(t, execVerb.Subcommands, "exec should have subcommands")
	require.Contains(t, execVerb.Subcommands, "copilot")
}

func TestCommandsCmd_Howto(t *testing.T) {
	listing := commands.Build(rootCmd)

	var buf bytes.Buffer
	err := commands.GenerateHowto(&buf, listing)
	require.NoError(t, err)

	output := buf.String()
	require.Contains(t, output, "# dtctl Quick Reference")
	require.Contains(t, output, "## Common Workflows")
	require.Contains(t, output, "## Safety Levels")

	// Verify all verbs appear in howto
	for verb := range listing.Verbs {
		require.True(t, strings.Contains(output, verb),
			"howto should mention verb %q", verb)
	}
}

func TestCommandsCmd_PatternsAndAntipatterns(t *testing.T) {
	listing := commands.Build(rootCmd)

	require.NotEmpty(t, listing.Patterns)
	require.NotEmpty(t, listing.Antipatterns)

	// Check for key content
	found := false
	for _, p := range listing.Patterns {
		if strings.Contains(p, "apply") {
			found = true
			break
		}
	}
	require.True(t, found, "patterns should mention apply")

	found = false
	for _, p := range listing.Antipatterns {
		if strings.Contains(p, "table output") {
			found = true
			break
		}
	}
	require.True(t, found, "antipatterns should warn about table output parsing")
}
