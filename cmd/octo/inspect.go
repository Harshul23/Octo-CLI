package main

import (
  "encoding/json"
  "fmt"
  "github.com/Octo-sh/Octo-CLI/internal/intelligence"
  "github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:     "inspect [path]",
	Aliases: []string{"ins", "info"},
	Short:   "Inspect how Octo understands a repository",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		explain, _ := cmd.Flags().GetBool("explain")
		if explain {
			return explainCmd.RunE(cmd, args)
		}
		graph, _ := cmd.Flags().GetBool("graph")
		if graph {
			return graphCmd.RunE(cmd, args)
		}

		path := "."
		if len(args) == 1 {
			path = args[0]
		}
		model, err := intelligence.Analyze(path)
		if err != nil {
			return err
		}
		jsonOutput, err := cmd.Flags().GetBool("json")
		if err != nil {
			return err
		}
		if jsonOutput {
			data, err := json.MarshalIndent(model, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}
		fmt.Printf("Repository: %s\n", model.Name)
		fmt.Printf("Language: %s\n", unknown(model.Language))
		fmt.Printf("Framework: %s\n", unknown(model.Framework))
		fmt.Printf("Runtime: %s\n", unknown(model.RuntimeVersion))
		fmt.Printf("Package: %s\n", unknown(model.PackageManager))
		fmt.Printf("Run: %s\n", unknown(model.RunCommand))
		fmt.Printf("Setup: %s\n", unknown(model.SetupCommand))
		fmt.Printf("Monorepo: %t\n", model.Monorepo)
		fmt.Printf("Confidence: %.0f%%\n", model.Confidence*100)
		fmt.Printf("Evidence: %d signals\n", len(model.Evidence))
		if len(model.Components) > 0 {
			fmt.Printf("Components: %d\n", len(model.Components))
			for _, c := range model.Components {
				role := "runnable"
				if c.Role == "library" || c.RunCommand == "" {
					reason := "build/install dependency (no executable entry point)"
					if c.NonRunnableReason != "" {
						reason = c.NonRunnableReason
					}
					role = fmt.Sprintf("library: %s", reason)
				}
				fmt.Printf("  - %s (%s, %s): %s\n", c.Name, c.Path, unknown(c.Language), role)
			}
		}
		if len(model.Services) > 0 {
			fmt.Printf("Services: %d\n", len(model.Services))
			for _, s := range model.Services {
				fmt.Printf("  - %s (image: %s)\n", s.Name, unknown(s.Image))
			}
		}
		if len(model.Diagnostics) > 0 {
			fmt.Printf("Diagnostics: %d\n", len(model.Diagnostics))
			for _, d := range model.Diagnostics {
				fmt.Printf("  - %s\n", d)
			}
		}
		return nil
	},
}

var explainCmd = &cobra.Command{
	Use:     "explain [path]",
	Aliases: []string{"why", "exp"},
	Short:   "Explain the evidence behind Octo decisions",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) == 1 {
			path = args[0]
		}
		model, err := intelligence.Analyze(path)
		if err != nil {
			return err
		}
		planner := intelligence.DeterministicPlanner{}
		plan, err := planner.Plan(cmd.Context(), model)
		if err != nil {
			return err
		}

		jsonOutput, err := cmd.Flags().GetBool("json")
		if err == nil && jsonOutput {
			decisions := intelligence.BuildDecisionTrace(model, plan)
			data, err := json.MarshalIndent(decisions, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		fmt.Printf("Why Octo made these decisions for %s:\n\n", model.Name)
		for _, d := range intelligence.BuildDecisionTrace(model, plan) {
			fmt.Printf("- %s = %s\n  %s (confidence %.0f%%)\n", d.Decision, unknown(d.Value), d.Reason, d.Confidence*100)
			for _, e := range d.Evidence {
				fmt.Printf("  evidence: [%s] %s — %s (strength %.0f%%)\n", e.Kind, e.Detail, e.Path, e.Strength*100)
			}
		}
		fmt.Printf("\nOverall confidence: %.0f%%\n", model.Confidence*100)
		return nil
	},
}

func unknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func init() {
	inspectCmd.GroupID = "main"
	explainCmd.GroupID = "other"

	inspectCmd.Flags().BoolP("json", "j", false, "Output the ProjectModel as JSON")
	inspectCmd.Flags().BoolP("explain", "e", false, "Explain decisions and evidence behind detected facts")
	inspectCmd.Flags().BoolP("graph", "g", false, "Render the repository topology graph")

	explainCmd.Flags().BoolP("json", "j", false, "Output decisions and evidence as JSON")
	rootCmd.AddCommand(inspectCmd)
	rootCmd.AddCommand(explainCmd)
}