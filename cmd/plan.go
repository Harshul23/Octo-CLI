package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/harshul/octo-cli/internal/intelligence"
	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan [path]",
	Short: "Show the deterministic execution plan for a repository",
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

		plan, err := (intelligence.DeterministicPlanner{}).Plan(cmd.Context(), model)
		if err != nil {
			return err
		}

		jsonOutput, err := cmd.Flags().GetBool("json")
		if err != nil {
			return err
		}
		if jsonOutput {
			data, err := json.MarshalIndent(plan, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		order, err := intelligence.TopologicalOrder(plan)
		if err != nil {
			return err
		}

		fmt.Printf("Execution plan for %s\n\n", plan.ProjectName)
		for i, id := range order {
			step := findStep(plan, id)
			deps := "none"
			if len(step.DependsOn) > 0 {
				deps = strings.Join(step.DependsOn, ", ")
			}
			fmt.Printf("%d. [%s] %s\n", i+1, step.Phase, step.ID)
			if step.Command != "" {
				fmt.Printf("   command: %s\n", step.Command)
			}
			if step.WorkDir != "" {
				fmt.Printf("   workdir: %s\n", step.WorkDir)
			}
			fmt.Printf("   depends_on: %s\n", deps)
			fmt.Printf("   why: %s\n\n", step.Explanation)
		}
		return nil
	},
}

func findStep(plan intelligence.ExecutionPlan, id string) intelligence.ExecutionStep {
	for _, step := range plan.Steps {
		if step.ID == id {
			return step
		}
	}
	return intelligence.ExecutionStep{}
}

func init() {
	planCmd.Flags().Bool("json", false, "Output the ExecutionPlan as JSON")
	rootCmd.AddCommand(planCmd)
}
