package helm

import "helm.sh/helm/v3/pkg/release"

func Install(distro, name, namespace string) error {
	cha, err := LoadChart(distro)
	if err != nil {
		return err
	}

	installer := NewHelmInstaller(cha, name, namespace)
	if err = installer.Init(); err != nil {
		return err
	}
	if err = installer.Install(); err != nil {
		return err
	}
	return nil
}

func Status(distro, name, namespace string) (release.Status, error) {
	cha, err := LoadChart(distro)
	if err != nil {
		return "", err
	}

	installer := NewHelmInstaller(cha, name, namespace)
	if err = installer.Init(); err != nil {
		return "", err
	}
	return installer.Status()
}

func Uninstall(distro, name, namespace string) error {
	cha, err := LoadChart(distro)
	if err != nil {
		return err
	}

	installer := NewHelmInstaller(cha, name, namespace)
	if err = installer.Init(); err != nil {
		return err
	}
	if err = installer.Uninstall(); err != nil {
		return err
	}

	return nil
}
