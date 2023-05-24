/*
Copyright 2019 The KubeSphere Authors.

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

package v1alpha3

import (
	"github.com/edgewize-io/edgewize/pkg/constants"
	"github.com/emicklei/go-restful"
	restfulspec "github.com/emicklei/go-restful-openapi"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/edgewize-io/edgewize/pkg/api"
	"github.com/edgewize-io/edgewize/pkg/api/resource/v1alpha2"
	"github.com/edgewize-io/edgewize/pkg/apiserver/query"
	"github.com/edgewize-io/edgewize/pkg/apiserver/runtime"
	"github.com/edgewize-io/edgewize/pkg/informers"
	"github.com/edgewize-io/edgewize/pkg/models/components"
	v2 "github.com/edgewize-io/edgewize/pkg/models/registries/v2"
	resourcev1alpha3 "github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/resource"

	"net/http"
)

const (
	GroupName = "resources.kubesphere.io"

	tagClusteredResource  = "Clustered Resource"
	tagComponentStatus    = "Component Status"
	tagNamespacedResource = "Namespaced Resource"

	ok = "OK"
)

var GroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1alpha3"}

func Resource(resource string) schema.GroupResource {
	return GroupVersion.WithResource(resource).GroupResource()
}

func AddToContainer(c *restful.Container, informerFactory informers.InformerFactory, cache cache.Cache) error {

	webservice := runtime.NewWebService(GroupVersion)
	handler := New(resourcev1alpha3.NewResourceGetter(informerFactory, cache), components.NewComponentsGetter(informerFactory.KubernetesSharedInformerFactory()), informerFactory)

	webservice.Route(webservice.GET("/quotas").
		To(handler.handleGetClusterQuotas).
		Doc("get whole cluster's resource usage").
		Returns(http.StatusOK, api.StatusOK, api.ResourceQuota{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.ClusterResourcesTag}))

	webservice.Route(webservice.GET("/namespaces/{namespace}/quotas").
		Doc("get specified namespace's resource quota and usage").
		Param(webservice.PathParameter("namespace", "the name of the project")).
		Returns(http.StatusOK, api.StatusOK, api.ResourceQuota{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.NamespaceResourcesTag}).
		To(handler.handleGetNamespaceQuotas))

	webservice.Route(webservice.GET("/{resources}").
		To(handler.handleListResources).
		Metadata(restfulspec.KeyOpenAPITags, []string{tagClusteredResource}).
		Doc("Cluster level resources").
		Param(webservice.PathParameter("resources", "cluster level resource type, e.g. pods,jobs,configmaps,services.")).
		Param(webservice.QueryParameter(query.ParameterName, "name used to do filtering").Required(false)).
		Param(webservice.QueryParameter(query.ParameterPage, "page").Required(false).DataFormat("page=%d").DefaultValue("page=1")).
		Param(webservice.QueryParameter(query.ParameterLimit, "limit").Required(false)).
		Param(webservice.QueryParameter(query.ParameterAscending, "sort parameters, e.g. reverse=true").Required(false).DefaultValue("ascending=false")).
		Param(webservice.QueryParameter(query.ParameterOrderBy, "sort parameters, e.g. orderBy=createTime")).
		Returns(http.StatusOK, ok, api.ListResult{}))

	webservice.Route(webservice.GET("/{resources}/{name}").
		To(handler.handleGetResources).
		Metadata(restfulspec.KeyOpenAPITags, []string{tagClusteredResource}).
		Doc("Cluster level resource").
		Param(webservice.PathParameter("resources", "cluster level resource type, e.g. pods,jobs,configmaps,services.")).
		Param(webservice.PathParameter("name", "the name of the clustered resources")).
		Returns(http.StatusOK, api.StatusOK, nil))

	webservice.Route(webservice.GET("/namespaces/{namespace}/{resources}").
		To(handler.handleListResources).
		Metadata(restfulspec.KeyOpenAPITags, []string{tagNamespacedResource}).
		Doc("Namespace level resource query").
		Param(webservice.PathParameter("namespace", "the name of the project")).
		Param(webservice.PathParameter("resources", "namespace level resource type, e.g. pods,jobs,configmaps,services.")).
		Param(webservice.QueryParameter(query.ParameterName, "name used to do filtering").Required(false)).
		Param(webservice.QueryParameter(query.ParameterPage, "page").Required(false).DataFormat("page=%d").DefaultValue("page=1")).
		Param(webservice.QueryParameter(query.ParameterLimit, "limit").Required(false)).
		Param(webservice.QueryParameter(query.ParameterAscending, "sort parameters, e.g. reverse=true").Required(false).DefaultValue("ascending=false")).
		Param(webservice.QueryParameter(query.ParameterOrderBy, "sort parameters, e.g. orderBy=createTime")).
		Returns(http.StatusOK, ok, api.ListResult{}))

	webservice.Route(webservice.GET("/namespaces/{namespace}/{resources}/{name}").
		To(handler.handleGetResources).
		Metadata(restfulspec.KeyOpenAPITags, []string{tagNamespacedResource}).
		Doc("Namespace level get resource query").
		Param(webservice.PathParameter("namespace", "the name of the project")).
		Param(webservice.PathParameter("resources", "namespace level resource type, e.g. pods,jobs,configmaps,services.")).
		Param(webservice.PathParameter("name", "the name of resource")).
		Returns(http.StatusOK, ok, api.ListResult{}))

	webservice.Route(webservice.GET("/components").
		To(handler.handleGetComponents).
		Metadata(restfulspec.KeyOpenAPITags, []string{tagComponentStatus}).
		Doc("List the system components.").
		Returns(http.StatusOK, ok, []v1alpha2.ComponentStatus{}))
	webservice.Route(webservice.GET("/components/{component}").
		To(handler.handleGetComponentStatus).
		Metadata(restfulspec.KeyOpenAPITags, []string{tagComponentStatus}).
		Doc("Describe the specified system component.").
		Param(webservice.PathParameter("component", "component name")).
		Returns(http.StatusOK, ok, v1alpha2.ComponentStatus{}))
	webservice.Route(webservice.GET("/componenthealth").
		To(handler.handleGetSystemHealthStatus).
		Metadata(restfulspec.KeyOpenAPITags, []string{tagComponentStatus}).
		Doc("Get the health status of system components.").
		Returns(http.StatusOK, ok, v1alpha2.HealthStatus{}))

	webservice.Route(webservice.POST("/namespaces/{namespace}/registrysecrets/{secret}/verify").
		To(handler.handleVerifyImageRepositorySecret).
		Param(webservice.PathParameter("namespace", "Namespace of the image repository secret to create.").Required(true)).
		Param(webservice.PathParameter("secret", "Name of the secret name").Required(true)).
		Param(webservice.BodyParameter("secretSpec", "Secret specification, definition in k8s.io/api/core/v1/types.Secret")).
		Reads(v1.Secret{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{tagNamespacedResource}).
		Doc("Verify image repostiry secret.").
		Returns(http.StatusOK, ok, v1.Secret{}))

	webservice.Route(webservice.GET("/namespaces/{namespace}/imageconfig").
		To(handler.handleGetImageConfig).
		Param(webservice.PathParameter("namespace", "Namespace of the image repository secret.").Required(true)).
		Param(webservice.QueryParameter("secret", "Secret name of the image repository credential, left empty means anonymous fetch.").Required(false)).
		Param(webservice.QueryParameter("image", "Image name to query, e.g. kubesphere/ks-apiserver:v3.1.1").Required(true)).
		Metadata(restfulspec.KeyOpenAPITags, []string{tagNamespacedResource}).
		Doc("Get image config.").
		Returns(http.StatusOK, ok, v2.ImageConfig{}))

	webservice.Route(webservice.GET("/namespaces/{namespace}/repositorytags").
		To(handler.handleGetRepositoryTags).
		Param(webservice.PathParameter("namespace", "Namespace of the image repository secret.").Required(true)).
		Param(webservice.QueryParameter("repository", "Repository to query, e.g. calico/cni.").Required(true)).
		Param(webservice.QueryParameter("secret", "Secret name of the image repository credential, left empty means anonymous fetch.").Required(false)).
		Metadata(restfulspec.KeyOpenAPITags, []string{tagNamespacedResource}).
		Doc("List repository tags, this is an experimental API, use it by your own caution.").
		Returns(http.StatusOK, ok, v2.RepositoryTags{}))

	webservice.Route(webservice.GET("/abnormalworkloads").
		Doc("get abnormal workloads' count of whole cluster").
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.ClusterResourcesTag}).
		Returns(http.StatusOK, api.StatusOK, api.Workloads{}).
		To(handler.handleGetNamespacedAbnormalWorkloads))
	webservice.Route(webservice.GET("/namespaces/{namespace}/abnormalworkloads").
		Doc("get abnormal workloads' count of specified namespace").
		Param(webservice.PathParameter("namespace", "the name of the project")).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.NamespaceResourcesTag}).
		Returns(http.StatusOK, api.StatusOK, api.Workloads{}).
		To(handler.handleGetNamespacedAbnormalWorkloads))

	//webservice.Route(webservice.GET("/users/{user}/kubectl").
	//	To(handler.GetKubectlPod).
	//	Doc("get user's kubectl pod").
	//	Param(webservice.PathParameter("user", "username")).
	//	Metadata(restfulspec.KeyOpenAPITags, []string{constants.ToolboxTag}).
	//	Returns(http.StatusOK, api.StatusOK, models.PodInfo{}))

	c.Add(webservice)

	return nil
}
