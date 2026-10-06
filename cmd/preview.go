package main

import (
	"encoding/json"
	"fmt"

	"github.com/harshul/octo-cli/internal/intelligence"
	"github.com/harshul/octo-cli/internal/ui"
	"github.com/spf13/cobra"
)

var previewCmd = &cobra.Command{
	Use:   "preview [path]",
	Short: "Preview all machine changes, network ports, and processes before execution",
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

		planner := intelligence.DeterministicPlanner{}
		plan, err := planner.Plan(cmd.Context(), model)
		if err != nil {
			return err
		}

		preview := intelligence.BuildMachinePreview(cmd.Context(), model, plan)

		jsonOutput, err := cmd.Flags().GetBool("json")
		if err != nil {
			return err
		}

		if jsonOutput {
			data, err := json.MarshalIndent(preview, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		}

		fmt.Println(ui.HeadingStyle.Render("Pre-Execution Machine Impact Preview"))
		fmt.Printf("Repository: %s\n\n", model.Name)

		// 1. Runtime Compatibility
		fmt.Println(ui.HeadingStyle.Render("1. Runtime Toolchain & Compatibility"))
		if preview.RuntimeCheck.Compatible {
			ver := preview.RuntimeCheck.DetectedVersion
			if ver == "" {
				ver = "available"
			}
			fmt.Println(ui.SuccessLine(fmt.Sprintf("%s runtime: %s (compatible)", preview.RuntimeCheck.Runtime, ver)))
		} else {
			fmt.Println(ui.WarningLine(fmt.Sprintf("%s runtime: %s", preview.RuntimeCheck.Runtime, preview.RuntimeCheck.Message)))
		}
		if preview.RuntimeCheck.VirtualEnvPath != "" {
			fmt.Println(ui.InfoLine(fmt.Sprintf("Virtual environment: %s", preview.RuntimeCheck.VirtualEnvPath)))
		}

		// 2. Prerequisites
		if len(preview.Prerequisites) > 0 {
			fmt.Println("\n" + ui.HeadingStyle.Render("2. System Prerequisites"))
			for _, req := range preview.Prerequisites {
				fmt.Printf("  • [%s] %s — %s\n", req.Kind, req.Name, req.Explanation)
			}
		}

		// 3. Filesystem Mutations
		fmt.Println("\n" + ui.HeadingStyle.Render("3. Projected Filesystem Changes"))
		for _, m := range preview.Mutations {
			fmt.Printf("  • [%s] %s (%s)\n    %s\n", m.Kind, m.Target, m.Action, ui.Muted.Render(m.Explanation))
		}

		// 4. Network Impact
		if len(preview.Network) > 0 {
			fmt.Println("\n" + ui.HeadingStyle.Render("4. Network Listeners & Port Allocations"))
			for _, net := range preview.Network {
				fmt.Printf("  • Component %s: %s port %d (%s)\n", net.Component, net.Protocol, net.Port, net.Action)
			}
		}

		// 5. Processes
		if len(preview.Processes) > 0 {
			fmt.Println("\n" + ui.HeadingStyle.Render("5. Execution Commands & Processes"))
			for _, p := range preview.Processes {
				daemon := ""
				if p.LongRunning {
					daemon = " [detached/long-running]"
				}
				fmt.Printf("  • [%s] %s%s\n", p.Phase, ui.Command(p.Command), daemon)
			}
		}

		// 6. Environment
		if len(preview.Environment.MissingRequired) > 0 {
			fmt.Println("\n" + ui.ErrorLine(fmt.Sprintf("Missing %d required environment variable(s)!", len(preview.Environment.MissingRequired))))
		}

		return nil
	},
}

func init() {
	previewCmd.Flags().Bool("json", false, "Output machine preview as JSON")
	rootCmd.AddCommand(previewCmd)
}
