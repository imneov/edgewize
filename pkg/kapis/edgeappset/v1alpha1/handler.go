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
	"strconv"

	"github.com/emicklei/go-restful"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	kapi "github.com/edgewize-io/edgewize/pkg/api"
	"github.com/edgewize-io/edgewize/pkg/apiserver/query"
	kubesphere "github.com/edgewize-io/edgewize/pkg/client/clientset/versioned"
	"github.com/edgewize-io/edgewize/pkg/informers"
	apptemplatemodels "github.com/edgewize-io/edgewize/pkg/models/apptemplate"
	edgeappsetmodels "github.com/edgewize-io/edgewize/pkg/models/edgeappset"
	infermodelmodels "github.com/edgewize-io/edgewize/pkg/models/infermodel"
)

type handler struct {
	operator            edgeappsetmodels.Operator
	appTemplateOperator apptemplatemodels.Operator
	imtOperator         infermodelmodels.IMTOperator
	imdOperator         infermodelmodels.IMDOperator
}

func newHandler(ksclient kubesphere.Interface, client kubernetes.Interface, informers informers.InformerFactory) *handler {
	return &handler{
		operator:            edgeappsetmodels.NewAppSetOperator(ksclient, client, informers),
		appTemplateOperator: apptemplatemodels.NewAppTemplateOperator(informers),
		imtOperator:         infermodelmodels.NewInferModelTemplateOperator(informers),
		imdOperator:         infermodelmodels.NewInferModelDeploymentOperator(ksclient, client, informers),
	}
}

func (h *handler) handleListAppTemplates(req *restful.Request, resp *restful.Response) {
	queryParams := query.ParseQueryParameter(req)

	result, err := h.appTemplateOperator.ListAppTemplate(req.Request.Context(), "", queryParams)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *handler) handleGetAppTemplate(req *restful.Request, resp *restful.Response) {
	name := req.PathParameter("name")

	result, err := h.appTemplateOperator.GetAppTemplate(req.Request.Context(), "", name)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *handler) handleListWorkSpaceAppTemplates(req *restful.Request, resp *restful.Response) {
	queryParams := query.ParseQueryParameter(req)
	workspace := req.PathParameter("workspace")

	result, err := h.appTemplateOperator.ListAppTemplate(req.Request.Context(), workspace, queryParams)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *handler) handleGetWorkSpaceAppTemplate(req *restful.Request, resp *restful.Response) {
	name := req.PathParameter("name")
	workspace := req.PathParameter("workspace")

	result, err := h.appTemplateOperator.GetAppTemplate(req.Request.Context(), workspace, name)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *handler) handleListAllEdgeAppSets(req *restful.Request, resp *restful.Response) {
	queryParams := query.ParseQueryParameter(req)

	result, err := h.operator.ListEdgeAppSets(req.Request.Context(), "", queryParams)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *handler) handleListEdgeAppSets(req *restful.Request, resp *restful.Response) {
	namespace := req.PathParameter("namespace")
	queryParams := query.ParseQueryParameter(req)

	result, err := h.operator.ListEdgeAppSets(req.Request.Context(), namespace, queryParams)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *handler) handleGetEdgeAppSet(req *restful.Request, resp *restful.Response) {
	namespace := req.PathParameter("namespace")
	name := req.PathParameter("name")

	result, err := h.operator.GetEdgeAppSet(req.Request.Context(), namespace, name)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *handler) handleDeleteEdgeAppSet(req *restful.Request, resp *restful.Response) {
	namespace := req.PathParameter("namespace")
	name := req.PathParameter("name")
	deleteWorkloads, _ := strconv.ParseBool(req.QueryParameter("delete_workloads"))

	result, err := h.operator.DeleteEdgeAppSet(req.Request.Context(), namespace, name, deleteWorkloads)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *handler) handleListWorkspaceInferModelTemplates(req *restful.Request, resp *restful.Response) {
	queryParams := query.ParseQueryParameter(req)
	workspace := req.PathParameter("workspace")

	result, err := h.imtOperator.ListInferModelTemplate(req.Request.Context(), workspace, queryParams)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *handler) handleGetWorkSpaceInferModelTemplate(req *restful.Request, resp *restful.Response) {
	name := req.PathParameter("name")
	workspace := req.PathParameter("workspace")

	result, err := h.imtOperator.GetInferModelTemplate(req.Request.Context(), workspace, name)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *handler) handleListInferModelTemplates(req *restful.Request, resp *restful.Response) {
	queryParams := query.ParseQueryParameter(req)

	result, err := h.imtOperator.ListInferModelTemplate(req.Request.Context(), "", queryParams)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *handler) handleGetInferModelTemplate(req *restful.Request, resp *restful.Response) {
	name := req.PathParameter("name")

	result, err := h.imtOperator.GetInferModelTemplate(req.Request.Context(), "", name)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *handler) handleListAllInferModelDeployments(req *restful.Request, resp *restful.Response) {
	queryParams := query.ParseQueryParameter(req)

	result, err := h.imdOperator.ListInferModelDeployments(req.Request.Context(), "", queryParams)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *handler) handleListInferModelDeployments(req *restful.Request, resp *restful.Response) {
	namespace := req.PathParameter("namespace")
	queryParams := query.ParseQueryParameter(req)

	result, err := h.imdOperator.ListInferModelDeployments(req.Request.Context(), namespace, queryParams)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *handler) handleGetInferModelDeployment(req *restful.Request, resp *restful.Response) {
	namespace := req.PathParameter("namespace")
	name := req.PathParameter("name")

	result, err := h.imdOperator.GetInferModelDeployment(req.Request.Context(), namespace, name)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *handler) handleDeleteInferModelDeployment(req *restful.Request, resp *restful.Response) {
	namespace := req.PathParameter("namespace")
	name := req.PathParameter("name")
	deleteWorkloads, _ := strconv.ParseBool(req.QueryParameter("delete_workloads"))

	result, err := h.imdOperator.DeleteInferModelDeployment(req.Request.Context(), namespace, name, deleteWorkloads)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}
	resp.WriteEntity(result)
}

func (h *handler) handleGetSpecifications(req *restful.Request, resp *restful.Response) {
	nodeName := req.PathParameter("node")
	result, err := h.imdOperator.ListNodeSpecifications(req.Request.Context(), nodeName)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}

	resp.WriteEntity(result)
}

func (h *handler) handleGetDeployedInferModelServer(req *restful.Request, resp *restful.Response) {
	nodeGroup := req.QueryParameter("node_group")

	result, err := h.imdOperator.ListRunningInferModelServers(req.Request.Context(), nodeGroup)
	if err != nil {
		klog.Error(err)
		kapi.HandleError(resp, req, err)
		return
	}

	resp.WriteEntity(result)
}
