package helm

import (
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
)

func Install(file, name, namespace, kubeconfig string, createNamespace bool, values chartutil.Values) error {
	cha, err := LoadChart(file)
	if err != nil {
		return err
	}

	installer := NewHelmInstaller(cha, name, namespace, kubeconfig)
	if err = installer.Init(); err != nil {
		return err
	}
	if err = installer.Install(values, createNamespace); err != nil {
		return err
	}
	return nil
}

func Status(file, name, namespace, kubeconfig string) (release.Status, error) {
	cha, err := LoadChart(file)
	if err != nil {
		return "", err
	}
	installer := NewHelmInstaller(cha, name, namespace, kubeconfig)
	if err = installer.Init(); err != nil {
		return "", err
	}
	return installer.Status()
}

func Uninstall(file, name, namespace, kubeconfig string) error {
	cha, err := LoadChart(file)
	if err != nil {
		return err
	}

	installer := NewHelmInstaller(cha, name, namespace, kubeconfig)
	if err = installer.Init(); err != nil {
		return err
	}
	if err = installer.Uninstall(); err != nil {
		return err
	}

	return nil
}
