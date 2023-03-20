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

package apiserver

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	rt "runtime"
	"strconv"
	"sync"
	"time"

	infrav1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/infra/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/apiserver/dispatch"
	"github.com/emicklei/go-restful"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	urlruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	apiserverconfig "github.com/edgewize-io/edgewize/pkg/apiserver/config"
	"github.com/edgewize-io/edgewize/pkg/apiserver/filters"
	"github.com/edgewize-io/edgewize/pkg/apiserver/request"
	"github.com/edgewize-io/edgewize/pkg/informers"
	"github.com/edgewize-io/edgewize/pkg/kapis/crd"
	clusterkapisv1alpha1 "github.com/edgewize-io/edgewize/pkg/kapis/edgecluster/v1alpha1"
	resourcev1alpha3 "github.com/edgewize-io/edgewize/pkg/kapis/resources/v1alpha3"
	terminalv1alpha2 "github.com/edgewize-io/edgewize/pkg/kapis/terminal/v1alpha2"
	"github.com/edgewize-io/edgewize/pkg/simple/client/cache"
	"github.com/edgewize-io/edgewize/pkg/simple/client/k8s"
	"github.com/edgewize-io/edgewize/pkg/utils/edgeclusterclient"
	"github.com/edgewize-io/edgewize/pkg/utils/iputil"
	"github.com/edgewize-io/edgewize/pkg/utils/metrics"
)

var initMetrics sync.Once

type APIServer struct {
	// number of kubesphere apiserver
	ServerCount int

	Server *http.Server

	Config *apiserverconfig.Config

	// webservice container, where all webservice defines
	container *restful.Container

	// kubeClient is a collection of all kubernetes(include CRDs) objects clientset
	KubernetesClient k8s.Client

	// informerFactory is a collection of all kubernetes(include CRDs) objects informers,
	// mainly for fast query
	InformerFactory informers.InformerFactory

	// cache is used for short lived objects, like session
	CacheClient cache.Interface

	// controller-runtime cache
	RuntimeCache runtimecache.Cache

	// controller-runtime client
	RuntimeClient runtimeclient.Client

	ClusterClient edgeclusterclient.ClusterClients
}

func (s *APIServer) PrepareRun(stopCh <-chan struct{}) error {
	s.container = restful.NewContainer()
	s.container.Filter(logRequestAndResponse)
	s.container.Router(restful.CurlyRouter{})
	s.container.RecoverHandler(func(panicReason interface{}, httpWriter http.ResponseWriter) {
		logStackOnRecover(panicReason, httpWriter)
	})

	s.installKubeSphereAPIs(stopCh)
	s.installCRDAPIs()
	s.installMetricsAPI()
	s.container.Filter(monitorRequest)

	for _, ws := range s.container.RegisteredWebServices() {
		klog.V(2).Infof("%s", ws.RootPath())
	}

	s.Server.Handler = s.container

	s.buildHandlerChain(stopCh)

	return nil
}

func monitorRequest(r *restful.Request, response *restful.Response, chain *restful.FilterChain) {
	start := time.Now()
	chain.ProcessFilter(r, response)
	reqInfo, exists := request.RequestInfoFrom(r.Request.Context())
	if exists && reqInfo.APIGroup != "" {
		RequestCounter.WithLabelValues(reqInfo.Verb, reqInfo.APIGroup, reqInfo.APIVersion, reqInfo.Resource, strconv.Itoa(response.StatusCode())).Inc()
		elapsedSeconds := time.Now().Sub(start).Seconds()
		RequestLatencies.WithLabelValues(reqInfo.Verb, reqInfo.APIGroup, reqInfo.APIVersion, reqInfo.Resource).Observe(elapsedSeconds)
	}
}

func (s *APIServer) installMetricsAPI() {
	initMetrics.Do(registerMetrics)
	metrics.Defaults.Install(s.container)
}

// Install all kubesphere api groups
// Installation happens before all informers start to cache objects, so
//
//	any attempt to list objects using listers will get empty results.
func (s *APIServer) installKubeSphereAPIs(stopCh <-chan struct{}) {

	urlruntime.Must(resourcev1alpha3.AddToContainer(s.container, s.InformerFactory, s.RuntimeCache))
	urlruntime.Must(terminalv1alpha2.AddToContainer(s.container, s.KubernetesClient.Kubernetes(), s.KubernetesClient.Config(), s.Config.TerminalOptions))
	urlruntime.Must(clusterkapisv1alpha1.AddToContainer(s.container,
		s.KubernetesClient.KubeSphere(),
		s.KubernetesClient.Kubernetes(),
		s.InformerFactory,
		s.RuntimeCache,
		s.KubernetesClient.Kubernetes().Discovery()))
}

// installCRDAPIs Install CRDs to the KAPIs with List and Get options
func (s *APIServer) installCRDAPIs() {
	crds := &extv1.CustomResourceDefinitionList{}
	// TODO Maybe we need a better label name
	urlruntime.Must(s.RuntimeClient.List(context.TODO(), crds, runtimeclient.MatchingLabels{"kubesphere.io/resource-served": "true"}))
	urlruntime.Must(crd.AddToContainer(s.container, s.RuntimeClient, s.RuntimeCache, crds))
}

func (s *APIServer) Run(ctx context.Context) (err error) {

	err = s.waitForResourceSync(ctx)
	if err != nil {
		return err
	}

	shutdownCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-ctx.Done()
		_ = s.Server.Shutdown(shutdownCtx)
	}()

	klog.V(0).Infof("Start listening on %s", s.Server.Addr)
	if s.Server.TLSConfig != nil {
		err = s.Server.ListenAndServeTLS("", "")
	} else {
		err = s.Server.ListenAndServe()
	}

	return err
}

func (s *APIServer) buildHandlerChain(stopCh <-chan struct{}) {
	requestInfoResolver := &request.RequestInfoFactory{
		APIPrefixes:          sets.NewString("api", "apis", "kapis", "kapi"),
		GrouplessAPIPrefixes: sets.NewString("api", "kapi"),
		GlobalResources: []schema.GroupResource{
			infrav1alpha1.Resource(infrav1alpha1.ResourcesPluralCluster),
			resourcev1alpha3.Resource(infrav1alpha1.ResourcesPluralCluster),
		},
	}

	handler := s.Server.Handler
	handler = filters.WithKubeAPIServer(handler, s.KubernetesClient.Config(), &errorResponder{})

	clusterDispatcher := dispatch.NewClusterDispatch(s.ClusterClient)
	handler = filters.WithEdgeClusterDispatcher(handler, clusterDispatcher)
	handler = filters.WithRequestInfo(handler, requestInfoResolver)

	s.Server.Handler = handler
}

func isResourceExists(apiResources []v1.APIResource, resource schema.GroupVersionResource) bool {
	for _, apiResource := range apiResources {
		if apiResource.Name == resource.Resource {
			return true
		}
	}
	return false
}

type informerForResourceFunc func(resource schema.GroupVersionResource) (interface{}, error)

func waitForCacheSync(discoveryClient discovery.DiscoveryInterface, sharedInformerFactory informers.GenericInformerFactory, informerForResourceFunc informerForResourceFunc, GVRs map[schema.GroupVersion][]string, stopCh <-chan struct{}) error {
	for groupVersion, resourceNames := range GVRs {
		var apiResourceList *v1.APIResourceList
		var err error
		err = retry.OnError(retry.DefaultRetry, func(err error) bool {
			return !errors.IsNotFound(err)
		}, func() error {
			apiResourceList, err = discoveryClient.ServerResourcesForGroupVersion(groupVersion.String())
			return err
		})
		if err != nil {
			if errors.IsNotFound(err) {
				klog.Warningf("group version %s not exists in the cluster", groupVersion)
				continue
			}
			return fmt.Errorf("failed to fetch group version resources %s: %s", groupVersion, err)
		}
		for _, resourceName := range resourceNames {
			groupVersionResource := groupVersion.WithResource(resourceName)
			if !isResourceExists(apiResourceList.APIResources, groupVersionResource) {
				klog.Warningf("resource %s not exists in the cluster", groupVersionResource)
			} else {
				// reflect.ValueOf(sharedInformerFactory).MethodByName("ForResource").Call([]reflect.Value{reflect.ValueOf(groupVersionResource)})
				if _, err = informerForResourceFunc(groupVersionResource); err != nil {
					return fmt.Errorf("failed to create informer for %s: %s", groupVersionResource, err)
				}
			}
		}
	}
	sharedInformerFactory.Start(stopCh)
	sharedInformerFactory.WaitForCacheSync(stopCh)
	return nil
}

func (s *APIServer) waitForResourceSync(ctx context.Context) error {
	klog.V(0).Info("Start cache objects")

	stopCh := ctx.Done()
	// resources we have to create informer first
	k8sGVRs := map[schema.GroupVersion][]string{
		{Group: "", Version: "v1"}: {
			"namespaces",
			"nodes",
			"resourcequotas",
			"pods",
			"services",
			"persistentvolumeclaims",
			"persistentvolumes",
			"secrets",
			"configmaps",
			"serviceaccounts",
		},
		{Group: "rbac.authorization.k8s.io", Version: "v1"}: {
			"roles",
			"rolebindings",
			"clusterroles",
			"clusterrolebindings",
		},
		{Group: "apps", Version: "v1"}: {
			"deployments",
			"daemonsets",
			"replicasets",
			"statefulsets",
			"controllerrevisions",
		},
		{Group: "storage.k8s.io", Version: "v1"}: {
			"storageclasses",
		},
		{Group: "batch", Version: "v1"}: {
			"jobs",
		},
		{Group: "batch", Version: "v1beta1"}: {
			"cronjobs",
		},
		{Group: "networking.k8s.io", Version: "v1"}: {
			"ingresses",
			"networkpolicies",
		},
		{Group: "autoscaling", Version: "v2beta2"}: {
			"horizontalpodautoscalers",
		},
	}

	if err := waitForCacheSync(s.KubernetesClient.Kubernetes().Discovery(),
		s.InformerFactory.KubernetesSharedInformerFactory(),
		func(resource schema.GroupVersionResource) (interface{}, error) {
			return s.InformerFactory.KubernetesSharedInformerFactory().ForResource(resource)
		},
		k8sGVRs, stopCh); err != nil {
		return err
	}

	ksGVRs := map[schema.GroupVersion][]string{}

	if err := waitForCacheSync(s.KubernetesClient.Kubernetes().Discovery(),
		s.InformerFactory.KubeSphereSharedInformerFactory(),
		func(resource schema.GroupVersionResource) (interface{}, error) {
			return s.InformerFactory.KubeSphereSharedInformerFactory().ForResource(resource)
		},
		ksGVRs, stopCh); err != nil {
		return err
	}

	snapshotGVRs := map[schema.GroupVersion][]string{
		{Group: "snapshot.storage.k8s.io", Version: "v1"}: {
			"volumesnapshots",
			"volumesnapshotcontents",
			"volumesnapshotclasses",
		},
	}

	if err := waitForCacheSync(s.KubernetesClient.Kubernetes().Discovery(),
		s.InformerFactory.SnapshotSharedInformerFactory(), func(resource schema.GroupVersionResource) (interface{}, error) {
			return s.InformerFactory.SnapshotSharedInformerFactory().ForResource(resource)
		},
		snapshotGVRs, stopCh); err != nil {
		return err
	}

	apiextensionsGVRs := map[schema.GroupVersion][]string{
		{Group: "apiextensions.k8s.io", Version: "v1"}: {
			"customresourcedefinitions",
		},
	}

	if err := waitForCacheSync(s.KubernetesClient.Kubernetes().Discovery(),
		s.InformerFactory.ApiExtensionSharedInformerFactory(), func(resource schema.GroupVersionResource) (interface{}, error) {
			return s.InformerFactory.ApiExtensionSharedInformerFactory().ForResource(resource)
		},
		apiextensionsGVRs, stopCh); err != nil {
		return err
	}

	go s.RuntimeCache.Start(ctx)
	s.RuntimeCache.WaitForCacheSync(ctx)

	klog.V(0).Info("Finished caching objects")
	return nil

}

func logStackOnRecover(panicReason interface{}, w http.ResponseWriter) {
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("recover from panic situation: - %v\r\n", panicReason))
	for i := 2; ; i += 1 {
		_, file, line, ok := rt.Caller(i)
		if !ok {
			break
		}
		buffer.WriteString(fmt.Sprintf("    %s:%d\r\n", file, line))
	}
	klog.Errorln(buffer.String())

	headers := http.Header{}
	if ct := w.Header().Get("Content-Type"); len(ct) > 0 {
		headers.Set("Accept", ct)
	}

	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("Internal server error"))
}

func logRequestAndResponse(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	start := time.Now()
	chain.ProcessFilter(req, resp)

	// Always log error response
	logWithVerbose := klog.V(4)
	if resp.StatusCode() > http.StatusBadRequest {
		logWithVerbose = klog.V(0)
	}

	logWithVerbose.Infof("%s - \"%s %s %s\" %d %d %dms",
		iputil.RemoteIp(req.Request),
		req.Request.Method,
		req.Request.URL,
		req.Request.Proto,
		resp.StatusCode(),
		resp.ContentLength(),
		time.Since(start)/time.Millisecond,
	)
}

type errorResponder struct{}

func (e *errorResponder) Error(w http.ResponseWriter, req *http.Request, err error) {
	klog.Error(err)
	responsewriters.InternalError(w, req, err)
}
