package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/harshul/octo-cli/internal/benchmark"
	"github.com/harshul/octo-cli/internal/ui"
	"github.com/spf13/cobra"
)

var benchmarkCmd = &cobra.Command{
	Use:     "benchmark [path]",
	Aliases: []string{"eval"},
	Short:   "Benchmark autonomous execution reliability and token efficiency against raw shell baselines",
	Long: `Benchmark autonomous agent execution against raw shell interaction.
Measures Pass@1 reliability, hallucination rate, token consumption, and readiness verification.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		suiteFlag, err := cmd.Flags().GetBool("suite")
		if err != nil {
			return err
		}

		jsonOutput, err := cmd.Flags().GetBool("json")
		if err != nil {
			return err
		}

		if suiteFlag {
			report, err := benchmark.RunBuiltinSuite(cmd.Context())
			if err != nil {
				return fmt.Errorf("benchmark suite failed: %w", err)
			}

			if jsonOutput {
				data, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			renderSuiteReport(cmd, report)
			return nil
		}

		path := "."
		if len(args) == 1 {
			path = args[0]
		}

		result, err := benchmark.EvaluateRepository(cmd.Context(), path, "")
		if err != nil {
			return fmt.Errorf("benchmark evaluation failed: %w", err)
		}

		if jsonOutput {
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		}

		renderSingleResult(cmd, result)
		return nil
	},
}

func renderSuiteReport(cmd *cobra.Command, report benchmark.BenchmarkSuiteReport) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\n📊 %s\n", ui.HeadingStyle.Render(report.SuiteName))
	fmt.Fprintln(out, strings.Repeat("═", 72))
	fmt.Fprintf(out, "Total Test Archetypes:  %d\n", report.TotalCases)
	fmt.Fprintf(out, "Octo Pass@1 Rate:       %s\n", ui.SuccessStyle.Render(fmt.Sprintf("%.1f%%", report.OctoPassRate)))
	fmt.Fprintf(out, "Baseline Pass Rate:     %s\n", ui.WarningStyle.Render(fmt.Sprintf("%.1f%%", report.BaselinePassRate)))
	fmt.Fprintf(out, "Avg Context Reduction:  %s\n", ui.SuccessStyle.Render(fmt.Sprintf("%.1f%%", report.AvgTokenSavingsPct)))
	fmt.Fprintf(out, "Avg Engine Latency:     %d ms\n", report.AvgLatencyMs)
	if report.ZeroHallucinationConfirmed {
		fmt.Fprintf(out, "Zero Hallucinations:    %s\n", ui.SuccessStyle.Render("Confirmed (0.0% error rate across all targets)"))
	}
	fmt.Fprintln(out, strings.Repeat("─", 72))

	fmt.Fprintln(out, "\n📋 Archetype Breakdown:")
	for _, res := range report.Results {
		status := ui.SuccessStyle.Render("✓ PASS")
		if !res.OctoScore.OneShotSuccess {
			status = ui.ErrorStyle.Render("✗ FAIL")
		}
		fmt.Fprintf(out, "  %s %-32s | %s | %4.1f%% tokens saved | %s\n",
			status,
			res.Archetype,
			fmt.Sprintf("%2dms", res.OctoScore.LatencyMs),
			res.Comparison.TokenReductionPct,
			res.Comparison.SafetyAdvantage,
		)
	}
	fmt.Fprintln(out)
}

func renderSingleResult(cmd *cobra.Command, res benchmark.BenchmarkResult) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\n📊 %s\n", ui.HeadingStyle.Render("Octo AI Agent Execution Benchmark"))
	fmt.Fprintln(out, strings.Repeat("═", 65))
	fmt.Fprintf(out, "📁 Target Repository: %s\n", ui.Path(res.Repository))
	if res.Archetype != "" {
		fmt.Fprintf(out, "🏷️  Archetype:         %s\n", res.Archetype)
	}
	fmt.Fprintln(out, strings.Repeat("─", 65))

	fmt.Fprintln(out, "\n🤖 Comparison with Raw LLM Shell Baseline:")
	fmt.Fprintf(out, "  %-24s %-20s %-20s\n", "Metric", "Octo (Deterministic)", "Raw Agent Shell")
	fmt.Fprintln(out, "  "+strings.Repeat("-", 60))

	octoPass := "PASS"
	if !res.OctoScore.OneShotSuccess {
		octoPass = "FAIL"
	}
	basePass := "PASS"
	if !res.BaselineScore.OneShotSuccess {
		basePass = "RETRY / FAIL"
	}
	fmt.Fprintf(out, "  %-24s %-20s %-20s\n", "Pass@1 Success", ui.SuccessStyle.Render(octoPass), ui.WarningStyle.Render(basePass))
	fmt.Fprintf(out, "  %-24s %-20s %-20s\n", "Hallucination Rate",
		ui.SuccessStyle.Render(fmt.Sprintf("%.1f%%", res.OctoScore.HallucinationRate*100)),
		ui.ErrorStyle.Render(fmt.Sprintf("%.1f%%", res.BaselineScore.HallucinationRate*100)))
	fmt.Fprintf(out, "  %-24s %-20s %-20s\n", "Context Tokens",
		fmt.Sprintf("~%d tokens", res.OctoScore.EstimatedTokens),
		fmt.Sprintf("~%d tokens", res.BaselineScore.EstimatedTokens))
	fmt.Fprintf(out, "  %-24s %-20s %-20s\n", "Verification Method",
		"Deterministic TCP/HTTP", "Blind Backgrounding (&)")
	fmt.Fprintf(out, "  %-24s %-20s %-20s\n", "Planning Latency",
		fmt.Sprintf("%d ms", res.OctoScore.LatencyMs),
		fmt.Sprintf("%d ms", res.BaselineScore.LatencyMs))

	fmt.Fprintln(out, "\n💡 Key Advantages:")
	fmt.Fprintf(out, "  • Context Token Reduction: %s\n", ui.SuccessStyle.Render(fmt.Sprintf("%.1f%%", res.Comparison.TokenReductionPct)))
	fmt.Fprintf(out, "  • Speedup Multiplier:      %s\n", ui.Accent.Render(fmt.Sprintf("%.1fx", res.Comparison.LatencySpeedupX)))
	fmt.Fprintf(out, "  • Safety Advantage:        %s\n", ui.InfoStyle.Render(res.Comparison.SafetyAdvantage))
	fmt.Fprintln(out)
}

func init() {
	benchmarkCmd.Flags().Bool("suite", false, "Run the canonical SWE-bench archetype benchmark suite")
	benchmarkCmd.Flags().Bool("json", false, "Output benchmark results as JSON")
	rootCmd.AddCommand(benchmarkCmd)
}
