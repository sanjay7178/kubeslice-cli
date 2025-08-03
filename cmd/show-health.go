package cmd

import (
	"github.com/kubeslice/kubeslice-cli/pkg"
	"github.com/kubeslice/kubeslice-cli/util"
	"github.com/spf13/cobra"
)

var showHealthCmd = &cobra.Command{
	Use:   "show-health",
	Short: "Show health status of Kubeslice resources.",
	Long:  "Show health status of Kubeslice resources like slices.",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var objectName string
		ns, _ := cmd.Flags().GetString("namespace")
		allNamespaces, _ := cmd.Flags().GetBool("all-namespaces")
		
		// For 'slice' resource type, we have specific handling
		if args[0] == "slice" {
			if allNamespaces {
				// kubeslice-cli show-health slice -A
				pkg.SetCliOptions(pkg.CliParams{Config: Config, Namespace: "", ObjectName: "", ObjectType: args[0], OutputFormat: outputFormat})
				pkg.ShowSliceHealthAll()
				return
			}
			
			if len(args) > 1 {
				// kubeslice-cli show-health slice <SLICE-NAME> -n <namespace>
				objectName = args[1]
				if ns == "" {
					util.Fatalf("Namespace is required when specifying a slice name (use -n <namespace>)")
				}
			} else {
				// kubeslice-cli show-health slice -n <namespace> (show all slices in namespace)
				if ns == "" {
					util.Fatalf("Namespace is required (use -n <namespace> or -A for all namespaces)")
				}
			}
			
			pkg.SetCliOptions(pkg.CliParams{Config: Config, Namespace: ns, ObjectName: objectName, ObjectType: args[0], OutputFormat: outputFormat})
			pkg.ShowSliceHealth()
			return
		}
		
		// For other resource types
		if !allNamespaces && ns == "" {
			util.Fatalf("Namespace is required (use -n <namespace> or -A for all namespaces)")
		}

		if len(args) > 1 {
			objectName = args[1]
		}

		pkg.SetCliOptions(pkg.CliParams{Config: Config, Namespace: ns, ObjectName: objectName, ObjectType: args[0], OutputFormat: outputFormat})
		switch args[0] {
		default:
			util.Fatalf("Invalid object type. Supported: slice")
		}
	},
}

func init() {
	rootCmd.AddCommand(showHealthCmd)
	showHealthCmd.Flags().StringP("namespace", "n", "", "namespace")
	showHealthCmd.Flags().BoolP("all-namespaces", "A", false, "show health for all namespaces")
	showHealthCmd.Flags().StringVarP(&outputFormat, "output", "o", "", "supported values json, yaml")
}