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
	"net/http"

	"github.com/emicklei/go-restful"
	restfulspec "github.com/emicklei/go-restful-openapi"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/edgewize-io/edgewize/pkg/apiserver/config"
	"github.com/edgewize-io/edgewize/pkg/informers"
	resourcev1alpha3 "github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/resource"
	"github.com/edgewize-io/edgewize/pkg/version"

	"github.com/edgewize-io/edgewize/pkg/api"
	"github.com/edgewize-io/edgewize/pkg/apiserver/runtime"
	kubesphere "github.com/edgewize-io/edgewize/pkg/client/clientset/versioned"
	"github.com/edgewize-io/edgewize/pkg/constants"
)

const (
	GroupName = "infra.edgewize.io"
)

var GroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1alpha1"}

func AddToContainer(container *restful.Container,
	config *config.Config,
	ksclient kubesphere.Interface,
	k8sclient kubernetes.Interface,
	informerFactory informers.InformerFactory,
	cache cache.Cache,
	k8sDiscovery discovery.DiscoveryInterface) error {
	k8sInformers := informerFactory.KubernetesSharedInformerFactory()
	ksInformers := informerFactory.KubeSphereSharedInformerFactory()

	webservice := runtime.NewWebService(GroupVersion)
	handler := New(config, ksclient, k8sclient, k8sInformers, ksInformers, resourcev1alpha3.NewResourceGetter(informerFactory, cache))

	// returns deployment yaml for cluster agent
	//webservice.Route(webservice.GET("/edgeclusters/{cluster}/agent/deployment").
	//	Doc("Return deployment yaml for cluster agent.").
	//	Param(webservice.PathParameter("cluster", "Name of the cluster.").Required(true)).
	//	To(handler.generateAgentDeployment).
	//	Returns(http.StatusOK, api.StatusOK, nil).
	//	Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeClusterTag}))

	webservice.Route(webservice.GET("/config").
		Doc("").
		To(handler.getConfig).
		Returns(http.StatusOK, api.StatusOK, nil).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeClusterTag}))

	webservice.Route(webservice.POST("/edgeclusters/validation").
		Doc("").
		To(handler.validateEdgeCluster).
		Returns(http.StatusOK, api.StatusOK, nil).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeClusterTag}))

	webservice.Route(webservice.POST("/clusters/validation").
		Doc("").
		To(handler.validateEdgeCluster).
		Returns(http.StatusOK, api.StatusOK, nil).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeClusterTag}))

	webservice.Route(webservice.PUT("/edgeclusters/{cluster}/kubeconfig").
		Doc("Update cluster kubeconfig.").
		Param(webservice.PathParameter("cluster", "Name of the cluster.").Required(true)).
		To(handler.updateKubeConfig).
		Returns(http.StatusOK, api.StatusOK, nil).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeClusterTag}))

	webservice.Route(webservice.PUT("/clusters/{cluster}/kubeconfig").
		Doc("Update cluster kubeconfig.").
		Param(webservice.PathParameter("cluster", "Name of the cluster.").Required(true)).
		To(handler.updateKubeConfig).
		Returns(http.StatusOK, api.StatusOK, nil).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeClusterTag}))

	webservice.Route(webservice.GET("/edgeclusters").
		Doc("").
		To(handler.listEdgeCluster).
		Returns(http.StatusOK, api.StatusOK, nil).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeClusterTag}))

	webservice.Route(webservice.GET("/clusters").
		Doc("").
		To(handler.listEdgeCluster).
		Returns(http.StatusOK, api.StatusOK, nil).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeClusterTag}))

	webservice.Route(webservice.GET("/nodes/join").
		Doc("").
		Param(webservice.BodyParameter("version", "kubeedge version")).
		Param(webservice.BodyParameter("runtime", "edge container runtime, containerd or docker")).
		Param(webservice.BodyParameter("node_name", "edge node name")).
		Param(webservice.BodyParameter("node_group", "node group of edge node")).
		Param(webservice.BodyParameter("image-repository", "private image repository address")).
		Param(webservice.BodyParameter("add_default_taint", "if add default taint")).
		To(handler.joinNode).
		Returns(http.StatusOK, api.StatusOK, nil).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeClusterTag}))

	webservice.Route(webservice.GET("/version").
		To(func(request *restful.Request, response *restful.Response) {
			ksVersion := version.Get()

			if k8sDiscovery != nil {
				k8sVersion, err := k8sDiscovery.ServerVersion()
				if err == nil {
					ksVersion.Kubernetes = k8sVersion
				} else {
					klog.Errorf("Failed to get kubernetes version, error %v", err)
				}
			}

			response.WriteAsJson(ksVersion)
		})).
		Doc("KubeSphere version")

	container.Add(webservice)

	return nil
}
