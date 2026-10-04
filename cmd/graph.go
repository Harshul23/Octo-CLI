package main

import (
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

		fmt.Printf("Repository topology for %s\n\n", model.Name)
		if len(graph.Nodes) == 0 {
			fmt.Println("No topology nodes detected.")
			return nil
		}

		edgesByFrom := make(map[string][]string)
		for _, edge := range graph.Edges {
			edgesByFrom[edge.From] = append(edgesByFrom[edge.From], edge.To)
		}

		for _, node := range graph.Nodes {
			fmt.Printf("%s [%s]\n", node.ID, node.Kind)
			deps := append([]string(nil), edgesByFrom[node.ID]...)
			sort.Strings(deps)
			if len(deps) == 0 {
				fmt.Println("  └─ depends on: none")
				continue
			}
			fmt.Printf("  └─ depends on: %s\n", strings.Join(deps, ", "))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(graphCmd)
}
