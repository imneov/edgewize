/*
Copyright 2020 KubeSphere Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/edgewize-io/edgewize/pkg/api"
	clusterv1alpha1 "github.com/edgewize-io/edgewize/pkg/api/cluster/v1alpha1"
	infrav1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/infra/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/apiserver/query"
	kubesphere "github.com/edgewize-io/edgewize/pkg/client/clientset/versioned"
	"github.com/edgewize-io/edgewize/pkg/client/informers/externalversions"
	clusterlister "github.com/edgewize-io/edgewize/pkg/client/listers/infra/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/constants"
	resourcev1alpha3 "github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/resource"
	"github.com/edgewize-io/edgewize/pkg/utils/k8sutil"
	"github.com/edgewize-io/edgewize/pkg/version"
	"github.com/emicklei/go-restful"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/cli-runtime/pkg/printers"
	k8sinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

const (
	defaultAgentImage   = "kubesphere/tower:v1.0"
	defaultTimeout      = 10 * time.Second
	KubeSphereApiServer = "ks-apiserver"
)

var errClusterConnectionIsNotProxy = fmt.Errorf("cluster is not using proxy connection")

type handler struct {
	k8sclient              kubernetes.Interface
	ksclient               kubesphere.Interface
	serviceLister          v1.ServiceLister
	clusterLister          clusterlister.ClusterLister
	configMapLister        v1.ConfigMapLister
	resourceGetterV1alpha3 *resourcev1alpha3.ResourceGetter

	yamlPrinter *printers.YAMLPrinter
}

func New(ksclient kubesphere.Interface, k8sclient kubernetes.Interface, k8sInformers k8sinformers.SharedInformerFactory, ksInformers externalversions.SharedInformerFactory, resourceGetterV1alpha3 *resourcev1alpha3.ResourceGetter) *handler {
	return &handler{
		ksclient:               ksclient,
		k8sclient:              k8sclient,
		serviceLister:          k8sInformers.Core().V1().Services().Lister(),
		clusterLister:          ksInformers.Infra().V1alpha1().Clusters().Lister(),
		configMapLister:        k8sInformers.Core().V1().ConfigMaps().Lister(),
		resourceGetterV1alpha3: resourceGetterV1alpha3,

		yamlPrinter: &printers.YAMLPrinter{},
	}
}

// updateKubeConfig updates the kubeconfig of the specific cluster, this API is used to update expired kubeconfig.
func (h *handler) updateKubeConfig(request *restful.Request, response *restful.Response) {
	var req clusterv1alpha1.UpdateClusterRequest
	if err := request.ReadEntity(&req); err != nil {
		api.HandleBadRequest(response, request, err)
		return
	}

	clusterName := request.PathParameter("cluster")
	obj, err := h.clusterLister.Get(clusterName)
	if err != nil {
		api.HandleBadRequest(response, request, err)
		return
	}
	cluster := obj.DeepCopy()
	if _, ok := cluster.Labels[infrav1alpha1.HostCluster]; ok {
		api.HandleBadRequest(response, request, fmt.Errorf("update kubeconfig of the host cluster is not allowed"))
		return
	}
	// For member clusters that use proxy mode, we don't need to update the kubeconfig,
	// if the certs expired, just restart the tower component in the host cluster, it will renew the cert.
	if cluster.Spec.Connection.Type == infrav1alpha1.ConnectionTypeProxy {
		api.HandleBadRequest(response, request, fmt.Errorf(
			"update kubeconfig of member clusters which using proxy mode is not allowed, their certs are managed and will be renewed by tower",
		))
		return
	}

	if len(req.KubeConfig) == 0 {
		api.HandleBadRequest(response, request, fmt.Errorf("cluster kubeconfig MUST NOT be empty"))
		return
	}
	config, err := k8sutil.LoadKubeConfigFromBytes(req.KubeConfig)
	if err != nil {
		api.HandleBadRequest(response, request, err)
		return
	}
	config.Timeout = defaultTimeout
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		api.HandleBadRequest(response, request, err)
		return
	}
	if _, err = clientSet.Discovery().ServerVersion(); err != nil {
		api.HandleBadRequest(response, request, err)
		return
	}

	_, err = validateKubeSphereAPIServer(config)
	if err != nil {
		api.HandleBadRequest(response, request, fmt.Errorf("unable validate kubesphere endpoint, %v", err))
		return
	}

	// Check if the cluster is the same
	kubeSystem, err := clientSet.CoreV1().Namespaces().Get(context.TODO(), metav1.NamespaceSystem, metav1.GetOptions{})
	if err != nil {
		api.HandleBadRequest(response, request, err)
		return
	}
	if kubeSystem.UID != cluster.Status.UID {
		api.HandleBadRequest(
			response, request, fmt.Errorf(
				"this kubeconfig corresponds to a different cluster than the previous one, you need to make sure that kubeconfig is not from another cluster",
			))
		return
	}

	cluster.Spec.Connection.KubeConfig = req.KubeConfig
	if _, err = h.ksclient.InfraV1alpha1().Clusters().Update(context.TODO(), cluster, metav1.UpdateOptions{}); err != nil {
		api.HandleBadRequest(response, request, err)
		return
	}
	response.WriteHeader(http.StatusOK)
}

// ValidateCluster validate cluster kubeconfig and kubesphere apiserver address, check their accessibility
func (h *handler) listEdgeCluster(request *restful.Request, response *restful.Response) {
	query := query.ParseQueryParameter(request)
	resourceType := "clusters"
	namespace := request.PathParameter("namespace")

	result, err := h.resourceGetterV1alpha3.List(resourceType, namespace, query)
	if err == nil {
		response.WriteEntity(result)
		return
	}

	if err != resourcev1alpha3.ErrResourceNotSupported {
		klog.Error(err, resourceType)
		api.HandleInternalError(response, request, err)
		return
	}
	response.WriteEntity(result)
}

// ValidateCluster validate cluster kubeconfig and kubesphere apiserver address, check their accessibility
func (h *handler) validateEdgeCluster(request *restful.Request, response *restful.Response) {
	var cluster infrav1alpha1.Cluster

	err := request.ReadEntity(&cluster)
	if err != nil {
		api.HandleBadRequest(response, request, err)
		return
	}

	if cluster.Spec.Connection.Type != infrav1alpha1.ConnectionTypeDirect {
		api.HandleBadRequest(response, request, fmt.Errorf("cluster connection type MUST be direct"))
		return
	}

	if len(cluster.Spec.Connection.KubeConfig) == 0 {
		api.HandleBadRequest(response, request, fmt.Errorf("cluster kubeconfig MUST NOT be empty"))
		return
	}

	config, err := k8sutil.LoadKubeConfigFromBytes(cluster.Spec.Connection.KubeConfig)
	if err != nil {
		api.HandleBadRequest(response, request, err)
		return
	}
	config.Timeout = defaultTimeout
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		api.HandleBadRequest(response, request, err)
		return
	}

	if err = h.validateKubeConfig(cluster.Name, clientSet); err != nil {
		api.HandleBadRequest(response, request, err)
		return
	}

	if _, err = validateKubeSphereAPIServer(config); err != nil {
		api.HandleBadRequest(response, request, fmt.Errorf("unable validate kubesphere endpoint, %v", err))
		return
	}

	response.WriteHeader(http.StatusOK)
}

// validateKubeConfig takes base64 encoded kubeconfig and check its validity
func (h *handler) validateKubeConfig(clusterName string, clientSet kubernetes.Interface) error {
	kubeSystem, err := clientSet.CoreV1().Namespaces().Get(context.TODO(), metav1.NamespaceSystem, metav1.GetOptions{})
	if err != nil {
		return err
	}

	clusters, err := h.clusterLister.List(labels.Everything())
	if err != nil {
		return err
	}

	// clusters with the exactly same kube-system namespace UID considered to be one
	// MUST not import the same cluster twice
	for _, existedCluster := range clusters {
		if existedCluster.Status.UID == kubeSystem.UID {
			return fmt.Errorf("cluster %s already exists (%s), MUST not import the same cluster twice", clusterName, existedCluster.Name)
		}
	}

	_, err = clientSet.Discovery().ServerVersion()
	return err
}

// validateKubeSphereAPIServer uses version api to check the accessibility
func validateKubeSphereAPIServer(config *rest.Config) (*version.Info, error) {
	transport, err := rest.TransportFor(config)
	if err != nil {
		return nil, err
	}
	client := http.Client{
		Timeout:   defaultTimeout,
		Transport: transport,
	}

	response, err := client.Get(fmt.Sprintf("%s/api/v1/namespaces/%s/services/:%s:/proxy/kapis/version", config.Host, constants.KubeSphereNamespace, KubeSphereApiServer))
	if err != nil {
		return nil, err
	}

	responseBytes, _ := ioutil.ReadAll(response.Body)
	responseBody := string(responseBytes)

	response.Body = ioutil.NopCloser(bytes.NewBuffer(responseBytes))

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid response: %s , please make sure %s.%s.svc of member cluster is up and running", KubeSphereApiServer, constants.KubeSphereNamespace, responseBody)
	}

	ver := version.Info{}
	err = json.NewDecoder(response.Body).Decode(&ver)
	if err != nil {
		return nil, fmt.Errorf("invalid response: %s , please make sure %s.%s.svc of member cluster is up and running", KubeSphereApiServer, constants.KubeSphereNamespace, responseBody)
	}

	return &ver, nil
}
