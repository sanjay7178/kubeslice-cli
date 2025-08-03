package internal

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/kubeslice/kubeslice-cli/util"
)

const (
	HealthStatusHealthy   = "Healthy"
	HealthStatusUnhealthy = "Unhealthy"
	HealthStatusUnknown   = "Unknown"
)

type ClusterHealth struct {
	ClusterName    string             `json:"cluster_name"`
	ClusterType    string             `json:"cluster_type"`
	OverallStatus  string             `json:"overall_status"`
	NodeStatus     NodeHealthStatus   `json:"node_status"`
	PodStatus      PodHealthStatus    `json:"pod_status"`
	ComponentStatus ComponentHealthStatus `json:"component_status"`
	Timestamp      string             `json:"timestamp"`
}

type NodeHealthStatus struct {
	Status      string `json:"status"`
	ReadyNodes  int    `json:"ready_nodes"`
	TotalNodes  int    `json:"total_nodes"`
	Details     string `json:"details"`
}

type PodHealthStatus struct {
	Status          string `json:"status"`
	RunningPods     int    `json:"running_pods"`
	TotalPods       int    `json:"total_pods"`
	NamespaceStatus map[string]string `json:"namespace_status"`
	Details         string `json:"details"`
}

type ComponentHealthStatus struct {
	Status     string            `json:"status"`
	Components map[string]string `json:"components"`
	Details    string            `json:"details"`
}

type SliceHealth struct {
	SliceName           string                     `json:"slice_name"`
	Namespace           string                     `json:"namespace"`
	OverallStatus       string                     `json:"overall_status"`
	ConfigStatus        SliceConfigStatus          `json:"config_status"`
	DeploymentStatus    SliceGatewayStatus         `json:"deployment_status"`
	WorkerStatus        SliceConnectivityStatus    `json:"worker_status"`
	ParticipatingClusters []string                 `json:"participating_clusters"`
	Timestamp           string                     `json:"timestamp"`
}

type SliceConfigStatus struct {
	Status  string `json:"status"`
	Phase   string `json:"phase"`
	Details string `json:"details"`
}

type SliceGatewayStatus struct {
	Status   string            `json:"status"`
	Clusters map[string]string `json:"clusters"`
	Details  string            `json:"details"`
}

type SliceConnectivityStatus struct {
	Status            string            `json:"status"`
	WorkerClusters    map[string]string `json:"worker_clusters"`
	Details           string            `json:"details"`
}

func ShowClusterHealth(clusterName string, config *ConfigurationSpecs, options *CliOptionsStruct) {
	var cluster *Cluster
	var clusterType string

	// Find the cluster in configuration
	if config.Configuration.ClusterConfiguration.ControllerCluster.Name == clusterName {
		cluster = &config.Configuration.ClusterConfiguration.ControllerCluster
		clusterType = "controller"
	} else {
		for _, worker := range config.Configuration.ClusterConfiguration.WorkerClusters {
			if worker.Name == clusterName {
				cluster = &worker
				clusterType = "worker"
				break
			}
		}
	}

	if cluster == nil {
		util.Fatalf("Cluster '%s' not found in configuration", clusterName)
	}

	util.Printf("\n%s Checking health for cluster: %s (%s)", util.Info, clusterName, clusterType)
	
	health := checkClusterHealth(*cluster, clusterType)
	displayClusterHealth(health, options.OutputFormat)
}

func ShowAllClustersHealth(config *ConfigurationSpecs, options *CliOptionsStruct) {
	util.Printf("\n%s Checking health for all clusters", util.Info)
	
	var allHealth []ClusterHealth

	// Check controller cluster
	controllerHealth := checkClusterHealth(config.Configuration.ClusterConfiguration.ControllerCluster, "controller")
	allHealth = append(allHealth, controllerHealth)

	// Check worker clusters
	for _, worker := range config.Configuration.ClusterConfiguration.WorkerClusters {
		workerHealth := checkClusterHealth(worker, "worker")
		allHealth = append(allHealth, workerHealth)
	}

	displayAllClustersHealth(allHealth, options.OutputFormat)
}

func ShowSliceHealth(sliceName string, config *ConfigurationSpecs, options *CliOptionsStruct) {
	util.Printf("\n%s Checking health for slice: %s", util.Info, sliceName)
	
	health := checkSliceHealth(sliceName, config, options)
	displaySliceHealth(health, options.OutputFormat)
}

func ShowAllSlicesHealth(config *ConfigurationSpecs, options *CliOptionsStruct) {
	util.Printf("\n%s Checking health for all slices", util.Info)
	
	var allHealth []SliceHealth
	
	// Get all slice configs from the controller cluster
	controllerCluster := &config.Configuration.ClusterConfiguration.ControllerCluster
	projectNamespace := "kubeslice-" + config.Configuration.KubeSliceConfiguration.ProjectName
	if options.Namespace != "" {
		projectNamespace = options.Namespace
	}
	
	sliceNames := getAllSliceNames(controllerCluster, projectNamespace)
	
	for _, sliceName := range sliceNames {
		sliceHealth := checkSliceHealth(sliceName, config, options)
		allHealth = append(allHealth, sliceHealth)
	}
	
	displayAllSlicesHealth(allHealth, options.OutputFormat)
}

func checkClusterHealth(cluster Cluster, clusterType string) ClusterHealth {
	health := ClusterHealth{
		ClusterName: cluster.Name,
		ClusterType: clusterType,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	// Check node status
	health.NodeStatus = checkNodeHealth(cluster)
	
	// Check pod status
	health.PodStatus = checkPodHealth(cluster)
	
	// Check component status
	health.ComponentStatus = checkComponentHealth(cluster, clusterType)

	// Determine overall status
	health.OverallStatus = determineOverallStatus(health.NodeStatus.Status, health.PodStatus.Status, health.ComponentStatus.Status)

	return health
}

func checkNodeHealth(cluster Cluster) NodeHealthStatus {
	var outB, errB bytes.Buffer
	status := NodeHealthStatus{
		Status: HealthStatusUnknown,
	}

	// Get node status
	err := util.RunCommandCustomIO("kubectl", &outB, &errB, true, 
		"--context="+cluster.ContextName, 
		"--kubeconfig="+cluster.KubeConfigPath, 
		"get", "nodes", 
		"-o", "jsonpath={.items[*].status.conditions[?(@.type==\"Ready\")].status}")
	
	if err != nil {
		status.Details = fmt.Sprintf("Failed to get node status: %v", err)
		status.Status = HealthStatusUnhealthy
		return status
	}

	readyStatuses := strings.Fields(outB.String())
	status.TotalNodes = len(readyStatuses)
	
	for _, readyStatus := range readyStatuses {
		if readyStatus == "True" {
			status.ReadyNodes++
		}
	}

	if status.ReadyNodes == status.TotalNodes && status.TotalNodes > 0 {
		status.Status = HealthStatusHealthy
		status.Details = fmt.Sprintf("All %d nodes are ready", status.TotalNodes)
	} else if status.ReadyNodes > 0 {
		status.Status = HealthStatusUnhealthy
		status.Details = fmt.Sprintf("%d out of %d nodes are ready", status.ReadyNodes, status.TotalNodes)
	} else {
		status.Status = HealthStatusUnhealthy
		status.Details = "No nodes are ready"
	}

	return status
}

func checkPodHealth(cluster Cluster) PodHealthStatus {
	status := PodHealthStatus{
		Status:          HealthStatusUnknown,
		NamespaceStatus: make(map[string]string),
	}

	// Check pods in kubeslice-related namespaces
	namespaces := []string{"kubeslice-controller", "kubeslice-system"}
	
	totalRunning := 0
	totalPods := 0
	allHealthy := true

	for _, namespace := range namespaces {
		nsStatus := checkNamespacePods(cluster, namespace)
		status.NamespaceStatus[namespace] = nsStatus.Status
		
		if nsStatus.Status != HealthStatusHealthy {
			allHealthy = false
		}
		
		totalRunning += nsStatus.RunningPods
		totalPods += nsStatus.TotalPods
	}

	status.RunningPods = totalRunning
	status.TotalPods = totalPods

	if allHealthy && totalPods > 0 {
		status.Status = HealthStatusHealthy
		status.Details = fmt.Sprintf("All %d pods are running across kubeslice namespaces", totalRunning)
	} else if totalRunning > 0 {
		status.Status = HealthStatusUnhealthy
		status.Details = fmt.Sprintf("%d out of %d pods are running", totalRunning, totalPods)
	} else {
		status.Status = HealthStatusUnhealthy
		status.Details = "No pods are running in kubeslice namespaces"
	}

	return status
}

func checkNamespacePods(cluster Cluster, namespace string) PodHealthStatus {
	var outB, errB bytes.Buffer
	status := PodHealthStatus{
		Status: HealthStatusUnknown,
	}

	// Check if namespace exists first
	err := util.RunCommandCustomIO("kubectl", &outB, &errB, true,
		"--context="+cluster.ContextName,
		"--kubeconfig="+cluster.KubeConfigPath,
		"get", "namespace", namespace)
	
	if err != nil {
		// Namespace doesn't exist, which might be normal for some clusters
		status.Status = HealthStatusHealthy
		status.Details = fmt.Sprintf("Namespace %s does not exist", namespace)
		return status
	}

	// Get pod status in the namespace
	outB.Reset()
	errB.Reset()
	err = util.RunCommandCustomIO("kubectl", &outB, &errB, true,
		"--context="+cluster.ContextName,
		"--kubeconfig="+cluster.KubeConfigPath,
		"get", "pods", "-n", namespace,
		"-o", "jsonpath={.items[*].status.phase}")
	
	if err != nil {
		status.Details = fmt.Sprintf("Failed to get pod status in namespace %s: %v", namespace, err)
		status.Status = HealthStatusUnhealthy
		return status
	}

	if outB.String() == "" {
		// No pods in namespace
		status.Status = HealthStatusHealthy
		status.Details = fmt.Sprintf("No pods in namespace %s", namespace)
		return status
	}

	phases := strings.Fields(outB.String())
	status.TotalPods = len(phases)
	
	for _, phase := range phases {
		if phase == "Running" {
			status.RunningPods++
		}
	}

	if status.RunningPods == status.TotalPods {
		status.Status = HealthStatusHealthy
	} else {
		status.Status = HealthStatusUnhealthy
	}

	return status
}

func checkComponentHealth(cluster Cluster, clusterType string) ComponentHealthStatus {
	status := ComponentHealthStatus{
		Status:     HealthStatusUnknown,
		Components: make(map[string]string),
	}

	var components []string
	if clusterType == "controller" {
		components = []string{"kubeslice-controller", "cert-manager"}
	} else {
		components = []string{"kubeslice-operator", "spire-server", "spire-agent"}
	}

	allHealthy := true
	for _, component := range components {
		componentStatus := checkDeploymentStatus(cluster, component)
		status.Components[component] = componentStatus
		if componentStatus != HealthStatusHealthy {
			allHealthy = false
		}
	}

	if allHealthy {
		status.Status = HealthStatusHealthy
		status.Details = "All key components are healthy"
	} else {
		status.Status = HealthStatusUnhealthy
		status.Details = "Some components are not healthy"
	}

	return status
}

func checkDeploymentStatus(cluster Cluster, deploymentName string) string {
	var outB, errB bytes.Buffer
	
	// Try different namespaces where the deployment might exist
	namespaces := []string{"kubeslice-controller", "kubeslice-system", "cert-manager"}
	
	for _, namespace := range namespaces {
		err := util.RunCommandCustomIO("kubectl", &outB, &errB, true,
			"--context="+cluster.ContextName,
			"--kubeconfig="+cluster.KubeConfigPath,
			"get", "deployment", deploymentName, "-n", namespace,
			"-o", "jsonpath={.status.readyReplicas}/{.status.replicas}")
		
		if err == nil && outB.String() != "" {
			replicas := outB.String()
			if strings.Contains(replicas, "/") {
				parts := strings.Split(replicas, "/")
				if len(parts) == 2 && parts[0] == parts[1] && parts[0] != "0" {
					return HealthStatusHealthy
				}
			}
			return HealthStatusUnhealthy
		}
		outB.Reset()
		errB.Reset()
	}
	
	return HealthStatusUnknown
}

func determineOverallStatus(nodeStatus, podStatus, componentStatus string) string {
	if nodeStatus == HealthStatusHealthy && podStatus == HealthStatusHealthy && componentStatus == HealthStatusHealthy {
		return HealthStatusHealthy
	} else if nodeStatus == HealthStatusUnhealthy || podStatus == HealthStatusUnhealthy || componentStatus == HealthStatusUnhealthy {
		return HealthStatusUnhealthy
	}
	return HealthStatusUnknown
}

func displayClusterHealth(health ClusterHealth, outputFormat string) {
	if outputFormat == "json" {
		util.PrintJSON(health)
		return
	}
	if outputFormat == "yaml" {
		util.PrintYAML(health)
		return
	}

	// Default table format
	util.Printf("\n=== Cluster Health Report ===")
	util.Printf("Cluster: %s (%s)", health.ClusterName, health.ClusterType)
	util.Printf("Overall Status: %s", getStatusWithIcon(health.OverallStatus))
	util.Printf("Timestamp: %s", health.Timestamp)
	util.Printf("")

	util.Printf("Node Status: %s", getStatusWithIcon(health.NodeStatus.Status))
	util.Printf("  %s", health.NodeStatus.Details)
	util.Printf("")

	util.Printf("Pod Status: %s", getStatusWithIcon(health.PodStatus.Status))
	util.Printf("  %s", health.PodStatus.Details)
	for ns, status := range health.PodStatus.NamespaceStatus {
		util.Printf("  - %s: %s", ns, getStatusWithIcon(status))
	}
	util.Printf("")

	util.Printf("Component Status: %s", getStatusWithIcon(health.ComponentStatus.Status))
	util.Printf("  %s", health.ComponentStatus.Details)
	for component, status := range health.ComponentStatus.Components {
		util.Printf("  - %s: %s", component, getStatusWithIcon(status))
	}
}

func displayAllClustersHealth(allHealth []ClusterHealth, outputFormat string) {
	if outputFormat == "json" {
		util.PrintJSON(allHealth)
		return
	}
	if outputFormat == "yaml" {
		util.PrintYAML(allHealth)
		return
	}

	// Default table format
	util.Printf("\n=== All Clusters Health Report ===")
	util.Printf("%-20s %-12s %-12s %-12s %-12s %-12s", "CLUSTER", "TYPE", "OVERALL", "NODES", "PODS", "COMPONENTS")
	util.Printf("%-20s %-12s %-12s %-12s %-12s %-12s", 
		strings.Repeat("-", 20), 
		strings.Repeat("-", 12), 
		strings.Repeat("-", 12), 
		strings.Repeat("-", 12), 
		strings.Repeat("-", 12), 
		strings.Repeat("-", 12))

	for _, health := range allHealth {
		util.Printf("%-20s %-12s %-12s %-12s %-12s %-12s",
			health.ClusterName,
			health.ClusterType,
			getStatusForTable(health.OverallStatus),
			getStatusForTable(health.NodeStatus.Status),
			getStatusForTable(health.PodStatus.Status),
			getStatusForTable(health.ComponentStatus.Status))
	}
	util.Printf("")
}

func getStatusWithIcon(status string) string {
	switch status {
	case HealthStatusHealthy:
		return fmt.Sprintf("%s %s", util.Check, status)
	case HealthStatusUnhealthy:
		return fmt.Sprintf("%s %s", util.Cross, status)
	default:
		return fmt.Sprintf("%s %s", util.Wait, status)
	}
}

func getStatusForTable(status string) string {
	switch status {
	case HealthStatusHealthy:
		return "✓ Healthy"
	case HealthStatusUnhealthy:
		return "✗ Unhealthy"
	default:
		return "? Unknown"
	}
}

func getAllSliceNames(controllerCluster *Cluster, namespace string) []string {
	var outB, errB bytes.Buffer
	var sliceNames []string
	
	err := util.RunCommandCustomIO("kubectl", &outB, &errB, true,
		"--context="+controllerCluster.ContextName,
		"--kubeconfig="+controllerCluster.KubeConfigPath,
		"get", SliceConfigObject, "-n", namespace,
		"-o", "jsonpath={.items[*].metadata.name}")
	
	if err != nil {
		util.Printf("%s Failed to get slice configurations: %v", util.Cross, err)
		return sliceNames
	}
	
	if outB.String() != "" {
		sliceNames = strings.Fields(outB.String())
	}
	
	return sliceNames
}

func checkSliceHealth(sliceName string, config *ConfigurationSpecs, options *CliOptionsStruct) SliceHealth {
	controllerCluster := &config.Configuration.ClusterConfiguration.ControllerCluster
	projectNamespace := "kubeslice-" + config.Configuration.KubeSliceConfiguration.ProjectName
	if options.Namespace != "" {
		projectNamespace = options.Namespace
	}
	
	health := SliceHealth{
		SliceName: sliceName,
		Namespace: projectNamespace,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	
	// Check slice configuration status
	health.ConfigStatus = checkSliceConfigStatus(controllerCluster, sliceName, projectNamespace)
	
	// Get participating clusters from slice config
	health.ParticipatingClusters = getSliceParticipatingClusters(controllerCluster, sliceName, projectNamespace)
	
	// Check slice deployment status on worker clusters
	health.DeploymentStatus = checkSliceDeploymentStatus(config, sliceName, projectNamespace, health.ParticipatingClusters)
	
	// Check connectivity between clusters by verifying slice objects exist on workers
	health.WorkerStatus = checkSlicePresenceOnWorkers(config, sliceName, health.ParticipatingClusters)
	
	// Determine overall status
	health.OverallStatus = determineSliceOverallStatus(health.ConfigStatus.Status, health.DeploymentStatus.Status, health.WorkerStatus.Status)
	
	return health
}

func checkSliceConfigStatus(controllerCluster *Cluster, sliceName string, namespace string) SliceConfigStatus {
	var outB, errB bytes.Buffer
	status := SliceConfigStatus{
		Status: HealthStatusUnknown,
	}
	
	// Get slice config status
	err := util.RunCommandCustomIO("kubectl", &outB, &errB, true,
		"--context="+controllerCluster.ContextName,
		"--kubeconfig="+controllerCluster.KubeConfigPath,
		"get", SliceConfigObject, sliceName, "-n", namespace,
		"-o", "jsonpath={.status.phase}")
	
	if err != nil {
		status.Details = fmt.Sprintf("Failed to get slice config: %v", err)
		status.Status = HealthStatusUnhealthy
		return status
	}
	
	phase := strings.TrimSpace(outB.String())
	status.Phase = phase
	
	switch phase {
	case "Ready":
		status.Status = HealthStatusHealthy
		status.Details = "Slice configuration is ready"
	case "Pending":
		status.Status = HealthStatusUnhealthy
		status.Details = "Slice configuration is pending"
	case "Failed":
		status.Status = HealthStatusUnhealthy
		status.Details = "Slice configuration has failed"
	default:
		if phase == "" {
			status.Status = HealthStatusUnhealthy
			status.Details = "Slice configuration status not available"
		} else {
			status.Status = HealthStatusUnknown
			status.Details = fmt.Sprintf("Unknown slice phase: %s", phase)
		}
	}
	
	return status
}

func getSliceParticipatingClusters(controllerCluster *Cluster, sliceName string, namespace string) []string {
	var outB, errB bytes.Buffer
	var clusters []string
	
	err := util.RunCommandCustomIO("kubectl", &outB, &errB, true,
		"--context="+controllerCluster.ContextName,
		"--kubeconfig="+controllerCluster.KubeConfigPath,
		"get", SliceConfigObject, sliceName, "-n", namespace,
		"-o", "jsonpath={.spec.clusters}")
	
	if err == nil && outB.String() != "" {
		clusterList := strings.Trim(outB.String(), "[]")
		if clusterList != "" {
			clusters = strings.Split(clusterList, ",")
			for i := range clusters {
				clusters[i] = strings.TrimSpace(clusters[i])
			}
		}
	}
	
	return clusters
}

func checkSliceDeploymentStatus(config *ConfigurationSpecs, sliceName string, namespace string, participatingClusters []string) SliceGatewayStatus {
	status := SliceGatewayStatus{
		Status:   HealthStatusUnknown,
		Clusters: make(map[string]string),
	}
	
	allHealthy := true
	totalClusters := 0
	healthyClusters := 0
	
	// Check slice deployment on each participating cluster
	for _, clusterName := range participatingClusters {
		// Find the worker cluster configuration
		var workerCluster *Cluster
		for _, worker := range config.Configuration.ClusterConfiguration.WorkerClusters {
			if worker.Name == clusterName {
				workerCluster = &worker
				break
			}
		}
		
		if workerCluster == nil {
			status.Clusters[clusterName] = HealthStatusUnknown
			allHealthy = false
			totalClusters++
			continue
		}
		
		clusterStatus := checkSliceOnWorkerCluster(workerCluster, sliceName)
		status.Clusters[clusterName] = clusterStatus
		totalClusters++
		
		if clusterStatus == HealthStatusHealthy {
			healthyClusters++
		} else {
			allHealthy = false
		}
	}
	
	if totalClusters == 0 {
		status.Status = HealthStatusUnknown
		status.Details = "No participating clusters found"
	} else if allHealthy {
		status.Status = HealthStatusHealthy
		status.Details = fmt.Sprintf("Slice is deployed on all %d participating clusters", totalClusters)
	} else {
		status.Status = HealthStatusUnhealthy
		status.Details = fmt.Sprintf("Slice is deployed on %d out of %d participating clusters", healthyClusters, totalClusters)
	}
	
	return status
}

func checkSliceOnWorkerCluster(workerCluster *Cluster, sliceName string) string {
	var outB, errB bytes.Buffer
	
	// Check if slice exists on worker cluster in kubeslice-system namespace
	err := util.RunCommandCustomIO("kubectl", &outB, &errB, true,
		"--context="+workerCluster.ContextName,
		"--kubeconfig="+workerCluster.KubeConfigPath,
		"get", "slice", sliceName, "-n", "kubeslice-system",
		"-o", "jsonpath={.metadata.name}")
	
	if err != nil {
		return HealthStatusUnhealthy
	}
	
	sliceExists := strings.TrimSpace(outB.String())
	if sliceExists == sliceName {
		return HealthStatusHealthy
	}
	
	return HealthStatusUnhealthy
}

func checkSlicePresenceOnWorkers(config *ConfigurationSpecs, sliceName string, participatingClusters []string) SliceConnectivityStatus {
	status := SliceConnectivityStatus{
		Status:            HealthStatusUnknown,
		WorkerClusters:    make(map[string]string),
	}
	
	if len(participatingClusters) < 2 {
		status.Status = HealthStatusHealthy
		status.Details = "Single cluster slice - no inter-cluster connectivity required"
		return status
	}
	
	allHealthy := true
	totalClusters := 0
	healthyClusters := 0
	
	// Check if slice is present on all worker clusters
	for _, clusterName := range participatingClusters {
		// Find the worker cluster configuration
		var workerCluster *Cluster
		for _, worker := range config.Configuration.ClusterConfiguration.WorkerClusters {
			if worker.Name == clusterName {
				workerCluster = &worker
				break
			}
		}
		
		if workerCluster == nil {
			linkStatus := HealthStatusUnknown
			status.WorkerClusters[clusterName] = linkStatus
			allHealthy = false
			totalClusters++
			continue
		}
		
		linkStatus := checkSliceOnWorkerCluster(workerCluster, sliceName)
		status.WorkerClusters[clusterName] = linkStatus
		totalClusters++
		
		if linkStatus == HealthStatusHealthy {
			healthyClusters++
		} else {
			allHealthy = false
		}
	}
	
	if totalClusters == 0 {
		status.Status = HealthStatusHealthy
		status.Details = "No worker clusters to check"
	} else if allHealthy {
		status.Status = HealthStatusHealthy
		status.Details = fmt.Sprintf("Slice is present on all %d worker clusters", totalClusters)
	} else {
		status.Status = HealthStatusUnhealthy
		status.Details = fmt.Sprintf("Slice is present on %d out of %d worker clusters", healthyClusters, totalClusters)
	}
	
	return status
}



func determineSliceOverallStatus(configStatus, deploymentStatus, workerStatus string) string {
	if configStatus == HealthStatusHealthy && deploymentStatus == HealthStatusHealthy && workerStatus == HealthStatusHealthy {
		return HealthStatusHealthy
	} else if configStatus == HealthStatusUnhealthy || deploymentStatus == HealthStatusUnhealthy || workerStatus == HealthStatusUnhealthy {
		return HealthStatusUnhealthy
	}
	return HealthStatusUnknown
}

func displaySliceHealth(health SliceHealth, outputFormat string) {
	if outputFormat == "json" {
		util.PrintJSON(health)
		return
	}
	if outputFormat == "yaml" {
		util.PrintYAML(health)
		return
	}

	// Default table format
	util.Printf("\n=== Slice Health Report ===")
	util.Printf("Slice: %s", health.SliceName)
	util.Printf("Namespace: %s", health.Namespace)
	util.Printf("Overall Status: %s", getStatusWithIcon(health.OverallStatus))
	util.Printf("Timestamp: %s", health.Timestamp)
	util.Printf("")

	util.Printf("Configuration Status: %s", getStatusWithIcon(health.ConfigStatus.Status))
	util.Printf("  Phase: %s", health.ConfigStatus.Phase)
	util.Printf("  %s", health.ConfigStatus.Details)
	util.Printf("")

	util.Printf("Deployment Status: %s", getStatusWithIcon(health.DeploymentStatus.Status))
	util.Printf("  %s", health.DeploymentStatus.Details)
	for cluster, status := range health.DeploymentStatus.Clusters {
		util.Printf("  - %s: %s", cluster, getStatusWithIcon(status))
	}
	util.Printf("")

	util.Printf("Worker Cluster Status: %s", getStatusWithIcon(health.WorkerStatus.Status))
	util.Printf("  %s", health.WorkerStatus.Details)
	for cluster, status := range health.WorkerStatus.WorkerClusters {
		util.Printf("  - %s: %s", cluster, getStatusWithIcon(status))
	}
	util.Printf("")

	if len(health.ParticipatingClusters) > 0 {
		util.Printf("Participating Clusters: %s", strings.Join(health.ParticipatingClusters, ", "))
	}
}

func displayAllSlicesHealth(allHealth []SliceHealth, outputFormat string) {
	if outputFormat == "json" {
		util.PrintJSON(allHealth)
		return
	}
	if outputFormat == "yaml" {
		util.PrintYAML(allHealth)
		return
	}

	// Default table format
	util.Printf("\n=== All Slices Health Report ===")
	util.Printf("%-20s %-15s %-12s %-12s %-12s %-12s", "SLICE", "NAMESPACE", "OVERALL", "CONFIG", "DEPLOYMENT", "WORKERS")
	util.Printf("%-20s %-15s %-12s %-12s %-12s %-12s", 
		strings.Repeat("-", 20), 
		strings.Repeat("-", 15), 
		strings.Repeat("-", 12), 
		strings.Repeat("-", 12), 
		strings.Repeat("-", 12), 
		strings.Repeat("-", 12))

	for _, health := range allHealth {
		util.Printf("%-20s %-15s %-12s %-12s %-12s %-12s",
			health.SliceName,
			health.Namespace,
			getStatusForTable(health.OverallStatus),
			getStatusForTable(health.ConfigStatus.Status),
			getStatusForTable(health.DeploymentStatus.Status),
			getStatusForTable(health.WorkerStatus.Status))
	}
	util.Printf("")
}