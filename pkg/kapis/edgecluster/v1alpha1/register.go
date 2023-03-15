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

	"github.com/edgewize-io/edgewize/pkg/informers"
	resourcev1alpha3 "github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/resource"
	"github.com/emicklei/go-restful"
	restfulspec "github.com/emicklei/go-restful-openapi"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/cache"

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
	ksclient kubesphere.Interface,
	informerFactory informers.InformerFactory,
	cache cache.Cache,
	proxyService string,
	proxyAddress string,
	agentImage string) error {
	k8sInformers := informerFactory.KubernetesSharedInformerFactory()
	ksInformers := informerFactory.KubeSphereSharedInformerFactory()

	webservice := runtime.NewWebService(GroupVersion)
	handler := New(ksclient, k8sInformers, ksInformers, proxyService, proxyAddress, agentImage, resourcev1alpha3.NewResourceGetter(informerFactory, cache))

	// returns deployment yaml for cluster agent
	webservice.Route(webservice.GET("/edgeclusters/{cluster}/agent/deployment").
		Doc("Return deployment yaml for cluster agent.").
		Param(webservice.PathParameter("cluster", "Name of the cluster.").Required(true)).
		To(handler.generateAgentDeployment).
		Returns(http.StatusOK, api.StatusOK, nil).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeClusterTag}))

	webservice.Route(webservice.POST("/edgeclusters/validation").
		Doc("").
		Param(webservice.BodyParameter("cluster", "cluster specification")).
		To(handler.validateEdgeCluster).
		Returns(http.StatusOK, api.StatusOK, nil).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeClusterTag}))

	webservice.Route(webservice.PUT("/edgeclusters/{cluster}/kubeconfig").
		Doc("Update cluster kubeconfig.").
		Param(webservice.PathParameter("cluster", "Name of the cluster.").Required(true)).
		To(handler.updateKubeConfig).
		Returns(http.StatusOK, api.StatusOK, nil).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeClusterTag}))

	webservice.Route(webservice.GET("/edgeclusters").
		Doc("").
		Param(webservice.BodyParameter("cluster", "cluster specification")).
		To(handler.listEdgeCluster).
		Returns(http.StatusOK, api.StatusOK, nil).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeClusterTag}))

	container.Add(webservice)

	return nil
}
