package cmd

import (
	"github.com/kubeslice/kubeslice-cli/pkg"
	"github.com/kubeslice/kubeslice-cli/util"
	"github.com/spf13/cobra"
)

var showHealthCmd = &cobra.Command{
	Use:   "show-health",
	Short: "Show health status of Kubeslice resources.",
	Long:  "Display health status and diagnostics for Kubeslice clusters, slices and components.",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var objectName string
		ns, _ := cmd.Flags().GetString("namespace")
		allClusters, _ := cmd.Flags().GetBool("all-clusters")
		
		if len(args) > 1 {
			objectName = args[1]
		}

		// For cluster and slice health checks, we need the configuration file
		if (args[0] == "cluster" || args[0] == "slice") && Config == "" && !allClusters {
			util.Fatalf("Configuration file is required for %s health checks. Use -c flag to specify config file.", args[0])
		}

		pkg.SetCliOptions(pkg.CliParams{Config: Config, Namespace: ns, ObjectName: objectName, ObjectType: args[0], OutputFormat: outputFormat})
		switch args[0] {
		case "cluster":
			if allClusters {
				pkg.ShowHealthAllClusters()
			} else {
				if objectName == "" {
					util.Fatalf("Cluster name is required. Use 'kubeslice-cli show-health cluster <CLUSTER-NAME>' or 'kubeslice-cli show-health cluster -A'")
				}
				pkg.ShowHealthCluster(objectName)
			}
		case "slice":
			if allClusters {
				pkg.ShowHealthAllSlices()
			} else {
				if objectName == "" {
					util.Fatalf("Slice name is required. Use 'kubeslice-cli show-health slice <SLICE-NAME>' or 'kubeslice-cli show-health slice -A'")
				}
				pkg.ShowHealthSlice(objectName)
			}
		default:
			util.Fatalf("Invalid object type. Currently supported: cluster, slice")
		}
	},
}

func init() {
	rootCmd.AddCommand(showHealthCmd)
	showHealthCmd.Flags().StringP("namespace", "n", "", "namespace")
	showHealthCmd.Flags().BoolP("all-clusters", "A", false, "show health for all clusters/slices")
	showHealthCmd.Flags().StringVarP(&outputFormat, "output", "o", "", "supported values json, yaml")
}
