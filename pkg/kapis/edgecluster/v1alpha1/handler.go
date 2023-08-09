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
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/emicklei/go-restful"
	"github.com/golang-jwt/jwt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/cli-runtime/pkg/printers"
	k8sinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/edgewize-io/edgewize/pkg/api"
	clusterv1alpha1 "github.com/edgewize-io/edgewize/pkg/api/cluster/v1alpha1"
	infrav1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/infra/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/apiserver/config"
	"github.com/edgewize-io/edgewize/pkg/apiserver/query"
	kubesphere "github.com/edgewize-io/edgewize/pkg/client/clientset/versioned"
	"github.com/edgewize-io/edgewize/pkg/client/informers/externalversions"
	clusterlister "github.com/edgewize-io/edgewize/pkg/client/listers/infra/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/constants"
	resourcev1alpha3 "github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/resource"
	"github.com/edgewize-io/edgewize/pkg/utils/k8sutil"
	"github.com/edgewize-io/edgewize/pkg/version"
)

const (
	defaultAgentImage   = "kubesphere/tower:v1.0"
	defaultTimeout      = 10 * time.Second
	KubeSphereApiServer = "ks-apiserver"
)

var errClusterConnectionIsNotProxy = fmt.Errorf("cluster is not using proxy connection")

type handler struct {
	config                 *config.Config
	k8sclient              kubernetes.Interface
	ksclient               kubesphere.Interface
	serviceLister          v1.ServiceLister
	clusterLister          clusterlister.ClusterLister
	configMapLister        v1.ConfigMapLister
	resourceGetterV1alpha3 *resourcev1alpha3.ResourceGetter

	yamlPrinter *printers.YAMLPrinter
}

func New(config *config.Config, ksclient kubesphere.Interface, k8sclient kubernetes.Interface, k8sInformers k8sinformers.SharedInformerFactory, ksInformers externalversions.SharedInformerFactory, resourceGetterV1alpha3 *resourcev1alpha3.ResourceGetter) *handler {
	return &handler{
		config:                 config,
		ksclient:               ksclient,
		k8sclient:              k8sclient,
		serviceLister:          k8sInformers.Core().V1().Services().Lister(),
		clusterLister:          ksInformers.Infra().V1alpha1().Clusters().Lister(),
		configMapLister:        k8sInformers.Core().V1().ConfigMaps().Lister(),
		resourceGetterV1alpha3: resourceGetterV1alpha3,

		yamlPrinter: &printers.YAMLPrinter{},
	}
}

// ValidateCluster validate cluster kubeconfig and kubesphere apiserver address, check their accessibility
func (h *handler) getConfig(request *restful.Request, response *restful.Response) {
	result := clusterv1alpha1.ConfigResponse{
		Code:   http.StatusOK,
		Status: StatusSucceeded,
		Data: clusterv1alpha1.Config{
			AdvertiseAddress: h.config.EdgeWizeOptions.Gateway.AdvertiseAddress,
			DNSNames:         h.config.EdgeWizeOptions.Gateway.DNSNames,
		},
	}
	response.WriteEntity(result)
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
	if _, ok := cluster.Labels[infrav1alpha1.HostClusterRole]; ok {
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

const (
	ClusterName string = "clustername"
	ClusterType string = "clustertype"
	NodeGroup   string = "nodegroup"
)

func newJoinTokenSecret(nodeGroup, clusterName, clusterType string) (string, error) {
	// set double TokenRefreshDuration as expirationTime, which can guarantee that the validity period
	// of the token obtained at anytime is greater than or equal to TokenRefreshDuration
	expiresAt := time.Now().Add(time.Hour * 24).Unix()

	token := jwt.New(jwt.SigningMethodHS256)

	token.Claims = jwt.StandardClaims{
		ExpiresAt: expiresAt,
	}
	token.Header[ClusterName] = clusterName
	token.Header[ClusterType] = clusterType
	token.Header[NodeGroup] = nodeGroup

	keyPEM, err := getCaKey()
	if err != nil {
		klog.Error(err, "failed to get ca key")
		return "", err
	}
	tokenString, err := token.SignedString(keyPEM)
	if err != nil {
		klog.Error(err, "failed to sign token")
		return "", err
	}

	caHash, err := getCaHash()
	if err != nil {
		klog.Error(err, "failed to get ca hash")
		return "", err
	}
	// combine caHash and tokenString into caHashAndToken
	caHashToken := strings.Join([]string{caHash, tokenString}, ".")
	// save caHashAndToken to secret
	return caHashToken, nil
}

// getCaHash gets ca-hash
func getCaHash() (string, error) {
	caPEM, err := os.ReadFile("/etc/certs/rootCA.crt")
	if err != nil {
		return "", err
	}
	block, _ := pem.Decode(caPEM)
	if block == nil {
		klog.Error("failed to decode PEM block containing public key")
		return "", err
	}

	// 解析 PEM 数据为 x509.Certificate 结构
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		klog.Errorf("failed to parse certificate: %v", err)
		return "", err
	}

	digest := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(digest[:]), nil
}

// getCaKey gets caKey to encrypt token
func getCaKey() ([]byte, error) {
	return os.ReadFile("/etc/certs/rootCA.key")
}
