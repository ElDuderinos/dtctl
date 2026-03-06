package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dynatrace-oss/dtctl/pkg/commands"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var briefMode bool

// commandsCmd outputs a machine-readable listing of all dtctl commands.
var commandsCmd = &cobra.Command{
	Use:   "commands [resource-or-verb]",
	Short: "List all commands as structured JSON for AI agents",
	Long: `Output a machine-readable catalog of dtctl's command tree.

The listing includes all verbs, resources, flags, mutating status, safety
operations, and resource aliases. It is designed for automated consumption
by AI coding agents and MCP servers.

Examples:
  # Full JSON listing
  dtctl commands

  # Brief listing (reduced token count)
  dtctl commands --brief

  # Commands for a specific resource
  dtctl commands workflows
  dtctl commands wf           # alias

  # Commands for a specific verb
  dtctl commands get

  # YAML output
  dtctl commands -o yaml

  # LLM-optimized markdown guide
  dtctl commands howto`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCommandsListing,
}

// howtoCmd outputs an LLM-optimized markdown reference guide.
var howtoCmd = &cobra.Command{
	Use:   "howto",
	Short: "Output an LLM-optimized usage guide in markdown",
	Long: `Output a markdown document optimized for LLM context windows.

The guide includes common workflows, safety levels, time formats, output
formats, patterns, and antipatterns. It is designed to be injected into
an AI agent's system prompt or context.

Examples:
  dtctl commands howto
  dtctl commands howto | pbcopy    # Copy to clipboard on macOS`,
	RunE: runHowto,
}

func runCommandsListing(cmd *cobra.Command, args []string) error {
	listing := commands.Build(rootCmd)

	// Apply resource/verb filter if a positional arg is provided
	if len(args) > 0 {
		if !commands.FilterByResource(listing, args[0]) {
			return fmt.Errorf("no commands found for %q", args[0])
		}
	}

	// Apply brief mode
	if briefMode {
		commands.ApplyBrief(listing)
	}

	// Output in requested format (reuse the global -o flag)
	switch outputFormat {
	case "yaml", "yml":
		enc := yaml.NewEncoder(os.Stdout)
		enc.SetIndent(2)
		if err := enc.Encode(listing); err != nil {
			return fmt.Errorf("encoding YAML: %w", err)
		}
		return enc.Close()
	default:
		// Default to JSON (even for table/wide/etc — JSON is the only
		// meaningful format for structured command metadata)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(listing)
	}
}

func runHowto(cmd *cobra.Command, args []string) error {
	listing := commands.Build(rootCmd)
	return commands.GenerateHowto(os.Stdout, listing)
}

func init() {
	commandsCmd.Flags().BoolVar(&briefMode, "brief", false, "minimal output (reduced token count for AI agents)")
	commandsCmd.AddCommand(howtoCmd)
	rootCmd.AddCommand(commandsCmd)
}
