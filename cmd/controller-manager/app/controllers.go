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

package app

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"
	"kubesphere.io/kubesphere/cmd/controller-manager/app/options"
	"kubesphere.io/kubesphere/pkg/controller/edgecluster"
	"kubesphere.io/kubesphere/pkg/informers"
	"kubesphere.io/kubesphere/pkg/simple/client/k8s"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var allControllers = []string{
	"cluster",
	"edgecluster",
}

// setup all available controllers one by one
func addAllControllers(mgr manager.Manager, client k8s.Client, informerFactory informers.InformerFactory,
	cmOptions *options.KubeSphereControllerManagerOptions,
	stopCh <-chan struct{}) error {

	kubesphereInformer := informerFactory.KubeSphereSharedInformerFactory()

	// "edgecluster" controller
	clusterController := edgecluster.NewClusterController(
		client.Kubernetes(),
		client.KubeSphere(),
		client.Config(),
		kubesphereInformer.Infra().V1alpha1().Clusters(),
		cmOptions.MultiClusterOptions.ClusterControllerResyncPeriod,
		cmOptions.MultiClusterOptions.HostClusterName,
	)
	addController(mgr, "cluster", clusterController)

	return nil
}

var addSuccessfullyControllers = sets.NewString()

func addController(mgr manager.Manager, name string, controller manager.Runnable) {
	if err := mgr.Add(controller); err != nil {
		klog.Fatalf("Unable to create %v controller: %v", name, err)
	}
	addSuccessfullyControllers.Insert(name)
}
