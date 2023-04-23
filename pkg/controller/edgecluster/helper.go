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

package edgecluster

import (
	"os"
	"path/filepath"

	infrav1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/infra/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/helm"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/client-go/util/homedir"
)

func InstallChart(file, name, namespace, kubeconfig string, createNamespace bool, values chartutil.Values) (infrav1alpha1.Status, error) {
	chartStatus, err := helm.Status(file, name, namespace, kubeconfig)
	if err != nil {
		return "", err
	}
	switch chartStatus {
	case release.StatusUnknown, release.StatusUninstalled:
		err = helm.Install(file, name, namespace, kubeconfig, createNamespace, values)
		if err != nil {
			return "", err
		}
		return infrav1alpha1.InstallingStatus, nil
	case release.StatusUninstalling:
		return infrav1alpha1.UninstallingStatus, nil
	case release.StatusDeployed:
		return infrav1alpha1.RunningStatus, nil
	case release.StatusFailed:
		return infrav1alpha1.ErrorStatus, nil
	case release.StatusPendingInstall, release.StatusPendingUpgrade, release.StatusPendingRollback:
		return infrav1alpha1.InstallingStatus, nil
	}
	return infrav1alpha1.ErrorStatus, nil
}

func SaveToLocal(name string, config []byte) error {
	path := filepath.Join(homedir.HomeDir(), ".kube", name)
	return os.WriteFile(path, config, 0644)
}
