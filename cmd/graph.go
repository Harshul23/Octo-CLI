package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/harshul/octo-cli/internal/intelligence"
	"github.com/spf13/cobra"
)

var graphCmd = &cobra.Command{
	Use:   "graph [path]",
	Short: "Show the detected repository topology graph",
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
		graph, err := intelligence.BuildTopologyGraph(model)
		if err != nil {
			return err
		}

		jsonOutput, err := cmd.Flags().GetBool("json")
		if err != nil {
			return err
		}
		if jsonOutput {
			data, err := json.MarshalIndent(graph, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		fmt.Printf("Repository topology for %s\n\n", model.Name)
		if len(graph.Nodes) == 0 {
			fmt.Println("No topology nodes detected.")
			return nil
		}

		for _, node := range graph.Nodes {
			fmt.Printf("%s [%s]\n", node.ID, node.Kind)
			var dependsOn []string
			var references []string
			seenDep := make(map[string]bool)
			seenRef := make(map[string]bool)
			for _, edge := range graph.Edges {
				if edge.From != node.ID {
					continue
				}
				if edge.Kind == intelligence.RelationshipDependsOn {
					if !seenDep[edge.To] {
						seenDep[edge.To] = true
						dependsOn = append(dependsOn, edge.To)
					}
				} else {
					if !seenRef[edge.To] {
						seenRef[edge.To] = true
						references = append(references, edge.To)
					}
				}
			}
			sort.Strings(dependsOn)
			sort.Strings(references)
			if len(dependsOn) == 0 && len(references) == 0 {
				fmt.Println("  └─ depends on: none")
				continue
			}
			if len(dependsOn) > 0 {
				fmt.Printf("  └─ depends on: %s\n", strings.Join(dependsOn, ", "))
			}
			if len(references) > 0 {
				fmt.Printf("  └─ references: %s\n", strings.Join(references, ", "))
			}
		}

		return nil
	},
}

func init() {
	graphCmd.Flags().Bool("json", false, "Output the TopologyGraph as JSON")
	rootCmd.AddCommand(graphCmd)
}
