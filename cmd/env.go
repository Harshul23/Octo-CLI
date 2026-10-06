package main

import (
	"encoding/json"
	"fmt"

	"github.com/harshul/octo-cli/internal/intelligence"
	"github.com/harshul/octo-cli/internal/ui"
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env [path]",
	Short: "Inspect required and optional environment variables and detect missing variables",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) == 1 {
			path = args[0]
		}

		model, err := intelligence.Analyze(path)
		if err != nil {
			return err
		}

		status := intelligence.InspectEnvironmentStatus(path, model)

		jsonOutput, err := cmd.Flags().GetBool("json")
		if err != nil {
			return err
		}

		if jsonOutput {
			data, err := json.MarshalIndent(status, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		}

		fmt.Printf("Environment variables for %s:\n\n", model.Name)
		fmt.Printf("Total detected: %d\n", status.TotalVariables)

		if len(status.Present) > 0 {
			fmt.Println("\n" + ui.SuccessLine(fmt.Sprintf("Present (%d):", len(status.Present))))
			for _, v := range status.Present {
				fmt.Printf("  • %s\n", v)
			}
		}

		if len(status.MissingRequired) > 0 {
			fmt.Println("\n" + ui.ErrorLine(fmt.Sprintf("Missing Required (%d):", len(status.MissingRequired))))
			for _, v := range status.MissingRequired {
				fmt.Printf("  • %s\n", v)
			}
		} else {
			fmt.Println("\n" + ui.SuccessLine("No required variables are missing."))
		}

		if len(status.MissingOptional) > 0 {
			fmt.Println("\n" + ui.InfoLine(fmt.Sprintf("Missing Optional (%d):", len(status.MissingOptional))))
			for _, v := range status.MissingOptional {
				fmt.Printf("  • %s\n", v)
			}
		}

		if len(status.MissingRequired) > 0 {
			fmt.Println("\nRun 'octo env template' to view or generate a safe .env template.")
		}

		return nil
	},
}

var envTemplateCmd = &cobra.Command{
	Use:   "template [path]",
	Short: "Generate a safe .env template with non-secret placeholders",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) == 1 {
			path = args[0]
		}

		model, err := intelligence.Analyze(path)
		if err != nil {
			return err
		}

		tpl := intelligence.GenerateEnvTemplate(path, model)
		fmt.Fprint(cmd.OutOrStdout(), tpl)
		return nil
	},
}

func init() {
	envCmd.Flags().Bool("json", false, "Output environment status as JSON")
	envCmd.AddCommand(envTemplateCmd)
	rootCmd.AddCommand(envCmd)
}
