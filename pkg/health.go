package pkg

import (
	"github.com/kubeslice/kubeslice-cli/pkg/internal"
)

func ShowHealthCluster(clusterName string) {
	internal.ShowClusterHealth(clusterName, ApplicationConfiguration, CliOptions)
}

func ShowHealthAllClusters() {
	internal.ShowAllClustersHealth(ApplicationConfiguration, CliOptions)
}

func ShowHealthSlice(sliceName string) {
	internal.ShowSliceHealth(sliceName, ApplicationConfiguration, CliOptions)
}

func ShowHealthAllSlices() {
	internal.ShowAllSlicesHealth(ApplicationConfiguration, CliOptions)
}
