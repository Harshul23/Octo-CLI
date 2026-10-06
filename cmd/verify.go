package main

import (
	"encoding/json"
	"fmt"

	"github.com/harshul/octo-cli/internal/intelligence"
	"github.com/harshul/octo-cli/internal/ui"
	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify [path]",
	Short: "Verify expected runtime state and lockfile integrity",
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

		checks := intelligence.BuildVerificationChecks(model, plan)
		results, verifyErr := intelligence.Verify(cmd.Context(), checks)

		lock, _ := intelligence.LoadOctoLock(path)
		lockPresent := len(lock.Strategies) > 0
		var strategyStatus intelligence.VerifiedStrategyApplication
		if lockPresent {
			strategyStatus, _ = intelligence.ApplyVerifiedStrategies(path, &model, lock)
		}

		jsonOutput, err := cmd.Flags().GetBool("json")
		if err != nil {
			return err
		}

		if jsonOutput {
			report := map[string]interface{}{
				"project_name":     model.Name,
				"success":          verifyErr == nil,
				"checks":           results,
				"lock_present":     lockPresent,
				"lock_entries":     len(lock.Strategies),
				"lock_reused":      len(strategyStatus.Reused),
				"lock_invalidated": len(strategyStatus.Invalidated),
			}
			if verifyErr != nil {
				report["error"] = verifyErr.Error()
			}
			data, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			if verifyErr != nil {
				return verifyErr
			}
			return nil
		}

		fmt.Printf("Verification report for %s:\n\n", model.Name)
		if len(checks) == 0 {
			fmt.Println(ui.Muted.Render("No active runtime verification checks required for this project."))
		} else {
			for _, res := range results {
				if res.Passed {
					fmt.Println(ui.SuccessLine(fmt.Sprintf("%s: passed (%s)", res.CheckID, res.Reason)))
				} else {
					fmt.Println(ui.ErrorLine(fmt.Sprintf("%s: failed (%s)", res.CheckID, res.Reason)))
				}
			}
		}

		if lockPresent {
			fmt.Printf("\n.octo.lock: %d verified candidate strategy(s) cached (%d active, %d invalidated).\n",
				len(lock.Strategies), len(strategyStatus.Reused), len(strategyStatus.Invalidated))
			for _, s := range strategyStatus.Reused {
				fmt.Printf("  • %s: %s (fingerprint verified)\n", s.Component, s.Command)
			}
			for _, s := range strategyStatus.Invalidated {
				fmt.Printf("  ! %s: %s (invalidated by repository edits)\n", s.Component, s.Command)
			}
		} else {
			fmt.Println("\n.octo.lock: no cached strategies found.")
		}

		if verifyErr != nil {
			return verifyErr
		}
		return nil
	},
}

func init() {
	verifyCmd.Flags().Bool("json", false, "Output verification results as JSON")
	rootCmd.AddCommand(verifyCmd)
}
