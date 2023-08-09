package edgecluster

import (
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
)


const (
	CurrentNamespace             = "edgewize-system"
	controllerName               = "edgecluster-controller"
	DefaultDistro                = "k3s"
	EdgeWizeValuesConfigName     = "edgewize-values-config"
	WhizardGatewayServiceName    = "gateway-whizard-operated"
	MonitorNamespace             = "kubesphere-monitoring-system"
	EdgeWizeServers              = "edgewize-servers.yaml"
	EdgeDeploySecret             = "edge-deploy-secret"
	MonitorPromServiceName       = "prometheus-k8s"
	WhizardEdgeGatewayConfigName = "whizard-edge-gateway-configmap"
)

type ServiceMap map[string]corev1.ServiceSpec

// Reconciler reconciles a Workspace object
type Reconciler struct {
	client.Client
	Logger                  logr.Logger
	Recorder                record.EventRecorder
	MaxConcurrentReconciles int
	Instances               sync.Map
}
