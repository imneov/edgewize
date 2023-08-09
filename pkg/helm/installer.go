/*
Copyright 2021 The EdgeWize Authors.

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

package helm

import (
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/util/homedir"
)

type Installer struct {
	chart         *chart.Chart
	configuration *action.Configuration
	name          string
	namespace     string
	kubeconfig    string
}

func NewHelmInstaller(cha *chart.Chart, name string, namespace string, kubeconfig string) *Installer {

	return &Installer{
		name:       name,
		namespace:  namespace,
		chart:      cha,
		kubeconfig: kubeconfig,
	}
}

func (i *Installer) Init() error {
	i.configuration = &action.Configuration{}
	k8sFlags := &genericclioptions.ConfigFlags{
		Namespace: &i.namespace,
	}
	if i.kubeconfig != "" {
		configpath := filepath.Join(homedir.HomeDir(), ".kube", i.kubeconfig)
		k8sFlags.KubeConfig = &configpath
	}
	debugFunc := func(format string, v ...interface{}) {}
	err := i.configuration.Init(k8sFlags, i.namespace, "", debugFunc)
	if err != nil {
		return errors.Wrap(err, "helm action configuration init err")
	}

	return nil
}

func (i *Installer) Install(values chartutil.Values) error {
	installer := action.NewInstall(i.configuration)
	installer.Namespace = i.namespace
	installer.ReleaseName = i.name
	installer.CreateNamespace = false
	installer.Wait = true
	installer.Timeout = 600 * time.Second
	if _, err := installer.Run(i.chart, values); err != nil {
		return errors.Wrap(err, "Installation failure")
	}
	return nil
}

func (i *Installer) Status() (release.Status, error) {
	if i.configuration.Releases == nil {
		return release.StatusUnknown, nil
	}
	last, err := i.configuration.Releases.Last(i.name)
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) {
			return release.StatusUnknown, nil
		}
		return "", err
	}
	return last.Info.Status, nil
}

func (i *Installer) Upgrade(values chartutil.Values) error {
	upgrader := action.NewUpgrade(i.configuration)
	upgrader.Namespace = i.namespace
	upgrader.Install = true
	upgrader.MaxHistory = 1
	upgrader.Wait = true
	upgrader.Timeout = 600 * time.Second
	if _, err := upgrader.Run(i.name, i.chart, values); err != nil {
		return errors.Wrap(err, "upgrade failure")
	}
	return nil
}

func (i *Installer) Uninstall() error {
	uninstallClint := action.NewUninstall(i.configuration)
	uninstallClint.KeepHistory = false
	_, err := uninstallClint.Run(i.name)
	if err != nil {
		err = errors.Wrap(err, "call uninstall err")
		return err
	}
	return nil
}
