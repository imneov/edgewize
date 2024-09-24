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
	"k8s.io/client-go/kubernetes"

	kapi "github.com/edgewize-io/edgewize/pkg/api"
	appsv1alpha1 "github.com/edgewize-io/edgewize/pkg/api/apps/v1alpha1"
	apisappsv1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/apiserver/query"
	"github.com/edgewize-io/edgewize/pkg/apiserver/runtime"
	kubesphere "github.com/edgewize-io/edgewize/pkg/client/clientset/versioned"
	"github.com/edgewize-io/edgewize/pkg/constants"
	"github.com/edgewize-io/edgewize/pkg/informers"
)

func AddToContainer(container *restful.Container, ksclient kubesphere.Interface,
	client kubernetes.Interface, informers informers.InformerFactory) error {

	ws := runtime.NewWebService(apisappsv1alpha1.SchemeGroupVersion)

	handler := newHandler(ksclient, client, informers)

	ws.Route(ws.GET("/edgeappsets").
		To(handler.handleListAllEdgeAppSets).
		Doc("list edgeappsets in the all namespace").
		Param(ws.QueryParameter(query.ParameterName, "name used to do filtering").Required(false)).
		Param(ws.QueryParameter(query.ParameterPage, "page").Required(false).DataFormat("page=%d").DefaultValue("page=1")).
		Param(ws.QueryParameter(query.ParameterLimit, "limit").Required(false)).
		Param(ws.QueryParameter(query.ParameterAscending, "sort parameters, e.g. reverse=true").Required(false).DefaultValue("ascending=false")).
		Param(ws.QueryParameter(query.ParameterOrderBy, "sort parameters, e.g. orderBy=createTime")).
		Returns(http.StatusOK, kapi.StatusOK, kapi.ListResult{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeAppSetTag}))

	ws.Route(ws.GET("/namespaces/{namespace}/edgeappsets").
		To(handler.handleListEdgeAppSets).
		Doc("list edgeappsets in the specified namespace").
		Param(ws.QueryParameter(query.ParameterName, "name used to do filtering").Required(false)).
		Param(ws.QueryParameter(query.ParameterPage, "page").Required(false).DataFormat("page=%d").DefaultValue("page=1")).
		Param(ws.QueryParameter(query.ParameterLimit, "limit").Required(false)).
		Param(ws.QueryParameter(query.ParameterAscending, "sort parameters, e.g. reverse=true").Required(false).DefaultValue("ascending=false")).
		Param(ws.QueryParameter(query.ParameterOrderBy, "sort parameters, e.g. orderBy=createTime")).
		Returns(http.StatusOK, kapi.StatusOK, kapi.ListResult{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeAppSetTag}))

	ws.Route(ws.GET("/namespaces/{namespace}/edgeappsets/{name}").
		To(handler.handleGetEdgeAppSet).
		Doc("get the edgeappset with the specified name in the specified namespace").
		Returns(http.StatusOK, kapi.StatusOK, appsv1alpha1.EdgeAppSet{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeAppSetTag}))

	ws.Route(ws.DELETE("/namespaces/{namespace}/edgeappsets/{name}").
		To(handler.handleDeleteEdgeAppSet).
		Doc("get the edgeappset with the specified name in the specified namespace").
		Returns(http.StatusOK, kapi.StatusOK, appsv1alpha1.EdgeAppSet{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeAppSetTag}))

	ws.Route(ws.GET("/workspaces/{workspace}/apptemplates").
		To(handler.handleListWorkSpaceAppTemplates).
		Doc("list all apptemplates").
		Param(ws.QueryParameter(query.ParameterName, "name used to do filtering").Required(false)).
		Param(ws.QueryParameter(query.ParameterPage, "page").Required(false).DataFormat("page=%d").DefaultValue("page=1")).
		Param(ws.QueryParameter(query.ParameterLimit, "limit").Required(false)).
		Param(ws.QueryParameter(query.ParameterAscending, "sort parameters, e.g. reverse=true").Required(false).DefaultValue("ascending=false")).
		Param(ws.QueryParameter(query.ParameterOrderBy, "sort parameters, e.g. orderBy=createTime")).
		Returns(http.StatusOK, kapi.StatusOK, kapi.ListResult{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeAppTemplateTag}))

	ws.Route(ws.GET("/workspaces/{workspace}/apptemplates/{name}").
		To(handler.handleGetWorkSpaceAppTemplate).
		Doc("get the apptemplate with the specified name").
		Returns(http.StatusOK, kapi.StatusOK, appsv1alpha1.AppTemplate{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeAppTemplateTag}))

	ws.Route(ws.GET("/apptemplates").
		To(handler.handleListAppTemplates).
		Doc("list all apptemplates").
		Param(ws.QueryParameter(query.ParameterName, "name used to do filtering").Required(false)).
		Param(ws.QueryParameter(query.ParameterPage, "page").Required(false).DataFormat("page=%d").DefaultValue("page=1")).
		Param(ws.QueryParameter(query.ParameterLimit, "limit").Required(false)).
		Param(ws.QueryParameter(query.ParameterAscending, "sort parameters, e.g. reverse=true").Required(false).DefaultValue("ascending=false")).
		Param(ws.QueryParameter(query.ParameterOrderBy, "sort parameters, e.g. orderBy=createTime")).
		Returns(http.StatusOK, kapi.StatusOK, kapi.ListResult{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeAppTemplateTag}))

	ws.Route(ws.GET("/apptemplates/{name}").
		To(handler.handleGetAppTemplate).
		Doc("get the apptemplate with the specified name").
		Returns(http.StatusOK, kapi.StatusOK, appsv1alpha1.AppTemplate{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeAppTemplateTag}))

	ws.Route(ws.GET("/workspaces/{workspace}/infermodeltemplates").
		To(handler.handleListWorkspaceInferModelTemplates).
		Param(ws.QueryParameter(query.ParameterName, "name used to do filtering").Required(false)).
		Param(ws.QueryParameter(query.ParameterPage, "page").Required(false).DataFormat("page=%d").DefaultValue("page=1")).
		Param(ws.QueryParameter(query.ParameterLimit, "limit").Required(false)).
		Param(ws.QueryParameter(query.ParameterAscending, "sort parameters, e.g. reverse=true").Required(false).DefaultValue("ascending=false")).
		Param(ws.QueryParameter(query.ParameterOrderBy, "sort parameters, e.g. sortBy=createTime")).
		Returns(http.StatusOK, kapi.StatusOK, kapi.ListResult{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeInferModelTemplateTag}))

	ws.Route(ws.GET("/workspaces/{workspace}/infermodeltemplates/{name}").
		To(handler.handleGetWorkSpaceInferModelTemplate).
		Doc("get the infer model template with the specified name").
		Returns(http.StatusOK, kapi.StatusOK, appsv1alpha1.InferModelTemplate{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeInferModelTemplateTag}))

	ws.Route(ws.GET("/infermodeltemplates").
		To(handler.handleListInferModelTemplates).
		Doc("list all imtemplates").
		Param(ws.QueryParameter(query.ParameterName, "name used to do filtering").Required(false)).
		Param(ws.QueryParameter(query.ParameterPage, "page").Required(false).DataFormat("page=%d").DefaultValue("page=1")).
		Param(ws.QueryParameter(query.ParameterLimit, "limit").Required(false)).
		Param(ws.QueryParameter(query.ParameterAscending, "sort parameters, e.g. reverse=true").Required(false).DefaultValue("ascending=false")).
		Param(ws.QueryParameter(query.ParameterOrderBy, "sort parameters, e.g. orderBy=createTime")).
		Returns(http.StatusOK, kapi.StatusOK, kapi.ListResult{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeInferModelTemplateTag}))

	ws.Route(ws.GET("/infermodeltemplates/{name}").
		To(handler.handleGetInferModelTemplate).
		Doc("get the apptemplate with the specified name").
		Returns(http.StatusOK, kapi.StatusOK, appsv1alpha1.AppTemplate{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeInferModelTemplateTag}))

	ws.Route(ws.GET("/infermodeldeployments").
		To(handler.handleListAllInferModelDeployments).
		Doc("list infer model deployments in the all namespace").
		Param(ws.QueryParameter(query.ParameterName, "name used to do filtering").Required(false)).
		Param(ws.QueryParameter(query.ParameterPage, "page").Required(false).DataFormat("page=%d").DefaultValue("page=1")).
		Param(ws.QueryParameter(query.ParameterLimit, "limit").Required(false)).
		Param(ws.QueryParameter(query.ParameterAscending, "sort parameters, e.g. reverse=true").Required(false).DefaultValue("ascending=false")).
		Param(ws.QueryParameter(query.ParameterOrderBy, "sort parameters, e.g. orderBy=createTime")).
		Returns(http.StatusOK, kapi.StatusOK, kapi.ListResult{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeInferModelDeploymentTag}))

	ws.Route(ws.GET("/namespaces/{namespace}/infermodeldeployments").
		To(handler.handleListInferModelDeployments).
		Doc("list infer model deployments in the specified namespace").
		Param(ws.QueryParameter(query.ParameterName, "name used to do filtering").Required(false)).
		Param(ws.QueryParameter(query.ParameterPage, "page").Required(false).DataFormat("page=%d").DefaultValue("page=1")).
		Param(ws.QueryParameter(query.ParameterLimit, "limit").Required(false)).
		Param(ws.QueryParameter(query.ParameterAscending, "sort parameters, e.g. reverse=true").Required(false).DefaultValue("ascending=false")).
		Param(ws.QueryParameter(query.ParameterOrderBy, "sort parameters, e.g. orderBy=createTime")).
		Returns(http.StatusOK, kapi.StatusOK, kapi.ListResult{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeInferModelDeploymentTag}))

	ws.Route(ws.GET("/namespaces/{namespace}/infermodeldeployments/{name}").
		To(handler.handleGetInferModelDeployment).
		Doc("get the infer model deployment with the specified name in the specified namespace").
		Returns(http.StatusOK, kapi.StatusOK, appsv1alpha1.InferModelDeployment{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeInferModelDeploymentTag}))

	ws.Route(ws.DELETE("/namespaces/{namespace}/infermodeldeployments/{name}").
		To(handler.handleDeleteInferModelDeployment).
		Doc("get the infer model deployment with the specified name in the specified namespace").
		Returns(http.StatusOK, kapi.StatusOK, appsv1alpha1.InferModelDeployment{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeInferModelDeploymentTag}))

	ws.Route(ws.GET("/specification/{node}").
		To(handler.handleGetSpecifications).
		Doc("get NPU/GPU specifications for the node").
		Returns(http.StatusOK, kapi.StatusOK, appsv1alpha1.NodeSpecifications{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeInferModelDeploymentTag}))

	ws.Route(ws.GET("/infermodelservers").
		To(handler.handleGetDeployedInferModelServer).
		Doc("get running infer model servers").
		Param(ws.QueryParameter("node_group", "node group of edge node")).
		Returns(http.StatusOK, kapi.StatusOK, appsv1alpha1.RunningInferModelServers{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.EdgeInferModelDeploymentTag}))

	container.Add(ws)

	return nil
}
