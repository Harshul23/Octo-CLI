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
	Short: "Show the detected component dependency graph",
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

		components := append([]intelligence.Component(nil), model.Components...)
		sort.Slice(components, func(i, j int) bool {
			return components[i].Name < components[j].Name
		})

		fmt.Printf("Component graph for %s\n\n", model.Name)
		if len(components) == 0 {
			fmt.Println("No components detected.")
			return nil
		}

		for _, component := range components {
			fmt.Printf("%s [component/%s]\n", component.Name, component.Language)
			printDependencies(component.DependsOn)
		}

		services := append([]intelligence.Service(nil), model.Services...)
		sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })
		if len(services) > 0 {
			fmt.Println("\nInfrastructure services")
			for _, service := range services {
				fmt.Printf("%s [service]\n", service.Name)
				printDependencies(service.DependsOn)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(graphCmd)
}

func printDependencies(deps []string) {
	if len(deps) == 0 {
		fmt.Println("  └─ depends on: none")
		return
	}
	values := append([]string(nil), deps...)
	sort.Strings(values)
	fmt.Printf("  └─ depends on: %s\n", strings.Join(values, ", "))
}
