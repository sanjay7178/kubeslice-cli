package internal

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/kubeslice/kubeslice-cli/util"
)

func InstallCalico(clusterConfig *ClusterConfiguration) {
	util.Printf("\nInstalling Calico Networking...")

	clusters := getAllClusters(clusterConfig)
	for _, cluster := range clusters {
		if !calicoAlreadyInstalled(cluster) {
			util.Printf("Installing on Cluster %s", cluster.Name)
			installCalicoOperatorPrerequisites(cluster)
			util.Printf("%s Successfully applied Calico Operator Prerequisites on Cluster %s", util.Tick, cluster.Name)
			
			util.Printf("%s Waiting for Calico CRDs to be ready on Cluster %s...", util.Wait, cluster.Name)
			waitForCalicoCRDs(cluster)
			util.Printf("%s Calico CRDs are ready on Cluster %s", util.Tick, cluster.Name)

			createCalicoOperator(cluster)
			util.Printf("%s Successfully installed Calico Operator on Cluster %s", util.Tick, cluster.Name)
			time.Sleep(200 * time.Millisecond)

			util.Printf("%s Waiting for Calico Pods to be Healthy on Cluster %s...", util.Wait, cluster.Name)
			PodVerification("Waiting for Calico Pods to be Healthy", *cluster, "calico-system")
		}
	}

	util.Printf("%s Successfully installed Calico Networking", util.Tick)
}

func calicoAlreadyInstalled(cluster *Cluster) bool {
	var outB, errB bytes.Buffer
	err := util.RunCommandCustomIO("kubectl", &outB, &errB, true, "--context="+cluster.ContextName, "--kubeconfig="+cluster.KubeConfigPath, "get", "namespace", "calico-system")
	if err != nil {
		if strings.Contains(errB.String(), "NotFound") {
			return false
		}
	}
	PodVerification("Waiting for Calico Pods to be Healthy", *cluster, "calico-system")
	util.Printf("%s Calico Networking already present on cluster %s", util.Tick, cluster.Name)
	return true
}

func installCalicoOperatorPrerequisites(cluster *Cluster) {
	err := util.RunCommand("kubectl", "--context="+cluster.ContextName, "--kubeconfig="+cluster.KubeConfigPath, "create", "-f", "https://raw.githubusercontent.com/projectcalico/calico/v3.30.0/manifests/tigera-operator.yaml")
	if err != nil {
		log.Fatalf("Process failed %v", err)
	}
}

func createCalicoOperator(cluster *Cluster) {
	err := util.RunCommand("kubectl", "--context="+cluster.ContextName, "--kubeconfig="+cluster.KubeConfigPath, "create", "-f", "https://raw.githubusercontent.com/projectcalico/calico/v3.30.0/manifests/custom-resources.yaml")
	if err != nil {
		log.Fatalf("Process failed %v", err)
	}
}

// waitForCalicoCRDs waits for the Calico CRDs to be ready before applying custom resources
func waitForCalicoCRDs(cluster *Cluster) {
	err := Retry(10, 5*time.Second, func() error {
		return checkCalicoCRDs(cluster)
	})
	if err != nil {
		log.Fatalf("Failed to wait for Calico CRDs: %v", err)
	}
}

// checkCalicoCRDs checks if the required Calico CRDs are available
func checkCalicoCRDs(cluster *Cluster) error {
	requiredCRDs := []string{
		"installations.operator.tigera.io",
		"apiservers.operator.tigera.io",
	}
	
	for _, crd := range requiredCRDs {
		var outB, errB bytes.Buffer
		err := util.RunCommandCustomIO("kubectl", &outB, &errB, true, 
			"--context="+cluster.ContextName, 
			"--kubeconfig="+cluster.KubeConfigPath, 
			"get", "crd", crd)
		if err != nil {
			return fmt.Errorf("CRD %s not ready: %v", crd, err)
		}
	}
	return nil
}
