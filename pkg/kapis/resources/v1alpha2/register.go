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

package v1alpha2

import (
	"net/http"

	"github.com/emicklei/go-restful"
	restfulspec "github.com/emicklei/go-restful-openapi"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"

	"kubesphere.io/kubesphere/pkg/api"
	"kubesphere.io/kubesphere/pkg/apiserver/runtime"
	"kubesphere.io/kubesphere/pkg/constants"
	"kubesphere.io/kubesphere/pkg/informers"
	"kubesphere.io/kubesphere/pkg/models"
	"kubesphere.io/kubesphere/pkg/server/params"
)

const (
	GroupName = "resources.kubesphere.io"
)

var GroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1alpha2"}

func AddToContainer(c *restful.Container, k8sClient kubernetes.Interface, factory informers.InformerFactory, masterURL string) error {
	webservice := runtime.NewWebService(GroupVersion)
	handler := newResourceHandler(k8sClient, factory, masterURL)

	webservice.Route(webservice.GET("/namespaces/{namespace}/{resources}").
		To(handler.handleListNamespaceResources).
		Deprecate().
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.NamespaceResourcesTag}).
		Doc("Namespace level resource query").
		Param(webservice.PathParameter("namespace", "the name of the project")).
		Param(webservice.PathParameter("resources", "namespace level resource type, e.g. pods,jobs,configmaps,services.")).
		Param(webservice.QueryParameter(params.ConditionsParam, "query conditions,connect multiple conditions with commas, equal symbol for exact query, wave symbol for fuzzy query e.g. name~a").
			Required(false).
			DataFormat("key=%s,key~%s")).
		Param(webservice.QueryParameter(params.PagingParam, "paging query, e.g. limit=100,page=1").
			Required(false).
			DataFormat("limit=%d,page=%d").
			DefaultValue("limit=10,page=1")).
		Param(webservice.QueryParameter(params.ReverseParam, "sort parameters, e.g. reverse=true")).
		Param(webservice.QueryParameter(params.OrderByParam, "sort parameters, e.g. orderBy=createTime")).
		Returns(http.StatusOK, api.StatusOK, models.PageableResponse{}))

	webservice.Route(webservice.GET("/{resources}").
		To(handler.handleListNamespaceResources).
		Deprecate().
		Returns(http.StatusOK, api.StatusOK, models.PageableResponse{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.ClusterResourcesTag}).
		Doc("Cluster level resources").
		Param(webservice.PathParameter("resources", "cluster level resource type, e.g. nodes,workspaces,storageclasses,clusterrole.")).
		Param(webservice.QueryParameter(params.ConditionsParam, "query conditions, connect multiple conditions with commas, equal symbol for exact query, wave symbol for fuzzy query e.g. name~a").
			Required(false).
			DataFormat("key=value,key~value").
			DefaultValue("")).
		Param(webservice.QueryParameter(params.PagingParam, "paging query, e.g. limit=100,page=1").
			Required(false).
			DataFormat("limit=%d,page=%d").
			DefaultValue("limit=10,page=1")).
		Param(webservice.QueryParameter(params.ReverseParam, "sort parameters, e.g. reverse=true")).
		Param(webservice.QueryParameter(params.OrderByParam, "sort parameters, e.g. orderBy=createTime")))

	webservice.Route(webservice.GET("/namespaces/{namespace}/daemonsets/{daemonset}/revisions/{revision}").
		To(handler.handleGetDaemonSetRevision).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.NamespaceResourcesTag}).
		Doc("Get the specified daemonset revision").
		Param(webservice.PathParameter("daemonset", "the name of the daemonset")).
		Param(webservice.PathParameter("namespace", "the namespace of the daemonset")).
		Param(webservice.PathParameter("revision", "the revision of the daemonset")).
		Returns(http.StatusOK, api.StatusOK, appsv1.DaemonSet{}))
	webservice.Route(webservice.GET("/namespaces/{namespace}/deployments/{deployment}/revisions/{revision}").
		To(handler.handleGetDeploymentRevision).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.NamespaceResourcesTag}).
		Doc("Get the specified deployment revision").
		Param(webservice.PathParameter("deployment", "the name of deployment")).
		Param(webservice.PathParameter("namespace", "the namespace of the deployment")).
		Param(webservice.PathParameter("revision", "the revision of the deployment")).
		Returns(http.StatusOK, api.StatusOK, appsv1.ReplicaSet{}))
	webservice.Route(webservice.GET("/namespaces/{namespace}/statefulsets/{statefulset}/revisions/{revision}").
		To(handler.handleGetStatefulSetRevision).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.NamespaceResourcesTag}).
		Doc("Get the specified statefulset revision").
		Param(webservice.PathParameter("statefulset", "the name of the statefulset")).
		Param(webservice.PathParameter("namespace", "the namespace of the statefulset")).
		Param(webservice.PathParameter("revision", "the revision of the statefulset")).
		Returns(http.StatusOK, api.StatusOK, appsv1.StatefulSet{}))

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

	c.Add(webservice)

	return nil
}