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
	"github.com/edgewize-io/edgewize/cmd/controller-manager/app/options"
	"github.com/edgewize-io/edgewize/pkg/controller/alerting"
	"github.com/edgewize-io/edgewize/pkg/controller/cluster"
	"github.com/edgewize-io/edgewize/pkg/controller/edgeappset"
	"github.com/edgewize-io/edgewize/pkg/controller/edgecluster"
	"github.com/edgewize-io/edgewize/pkg/informers"
	"github.com/edgewize-io/edgewize/pkg/simple/client/k8s"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var allControllers = []string{
	"edgecluster",
	"cluster",
	"rulegroup",
	"clusterrulegroup",
	"globalrulegroup",
}

// setup all available controllers one by one
func addAllControllers(mgr manager.Manager, client k8s.Client, informerFactory informers.InformerFactory,
	cmOptions *options.KubeSphereControllerManagerOptions, stopCh <-chan struct{}) error {

	kubesphereInformer := informerFactory.KubeSphereSharedInformerFactory()

	if cmOptions.InHostCluster() {
		clusterController := cluster.NewClusterController(
			client.Kubernetes(),
			client.KubeSphere(),
			client.Config(),
			kubesphereInformer.Infra().V1alpha1().Clusters(),
			cmOptions.EdgeWizeOptions.ClusterControllerResyncPeriod,
			cmOptions.EdgeWizeOptions.HostClusterName,
		)
		addController(mgr, "cluster", clusterController)

		// "edgecluster" controller
		edgeclusterReconciler := &edgecluster.Reconciler{}
		addControllerWithSetup(mgr, "edgecluster", edgeclusterReconciler)

		// "edgeappset" controller
		edgeAppSetReconciler := &edgeappset.Reconciler{}
		addControllerWithSetup(mgr, "edgeappset", edgeAppSetReconciler)
	}

	// controllers for alerting
	alertingOptionsEnable := cmOptions.AlertingOptions != nil && (cmOptions.AlertingOptions.PrometheusEndpoint != "" || cmOptions.AlertingOptions.ThanosRulerEndpoint != "")
	if alertingOptionsEnable {
		if !cmOptions.InHostCluster() {
			// "rulegroup" controller
			if cmOptions.IsControllerEnabled("rulegroup") {
				rulegroupReconciler := &alerting.RuleGroupReconciler{}
				addControllerWithSetup(mgr, "rulegroup", rulegroupReconciler)
			}
			// "clusterrulegroup" controller
			if cmOptions.IsControllerEnabled("clusterrulegroup") {
				clusterrulegroupReconciler := &alerting.ClusterRuleGroupReconciler{}
				addControllerWithSetup(mgr, "clusterrulegroup", clusterrulegroupReconciler)
			}
			// "globalrulegroup" controller
			if cmOptions.IsControllerEnabled("globalrulegroup") {
				globalrulegroupReconciler := &alerting.GlobalRuleGroupReconciler{}
				addControllerWithSetup(mgr, "globalrulegroup", globalrulegroupReconciler)
			}
		}

		if cmOptions.InHostCluster() {
			prometheusRuleReconcilers := &alerting.GlobalPrometheusRuleReconcilers{}
			addControllerWithSetup(mgr, "prometheusrules", prometheusRuleReconcilers)
		}
	}

	// log all controllers process result
	for _, name := range allControllers {
		if cmOptions.IsControllerEnabled(name) {
			if addSuccessfullyControllers.Has(name) {
				klog.Infof("%s controller is enabled and added successfully.", name)
			} else {
				klog.Infof("%s controller is enabled but is not going to run due to its dependent component being disabled.", name)
			}
		} else {
			klog.Infof("%s controller is disabled by controller selectors.", name)
		}
	}
	return nil
}

var addSuccessfullyControllers = sets.NewString()

func addController(mgr manager.Manager, name string, controller manager.Runnable) {
	if err := mgr.Add(controller); err != nil {
		klog.Fatalf("Unable to create %v controller: %v", name, err)
	}
	addSuccessfullyControllers.Insert(name)
}

type setupableController interface {
	SetupWithManager(mgr ctrl.Manager) error
}

func addControllerWithSetup(mgr manager.Manager, name string, controller setupableController) {
	if err := controller.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Unable to create %v controller: %v", name, err)
	}
	addSuccessfullyControllers.Insert(name)
}
