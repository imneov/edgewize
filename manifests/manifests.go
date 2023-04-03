/*
Copyright 2022 The KubeEdge Authors.

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
package manifests

import (
	"embed"
	"io/fs"
	"os"
)

// FS embeds the manifests
//go:embed charts/k0s/* charts/k3s/* charts/k8s/*
//go:embed charts/k0s/templates/*
//go:embed charts/k3s/templates/*
//go:embed charts/k8s/templates/*

//go:embed charts/edgewize/*
//go:embed charts/edgewize/crds/*
//go:embed charts/edgewize/templates/*

//go:embed charts/cloudcore/*
//go:embed charts/cloudcore/crds/*
//go:embed charts/cloudcore/templates/*

//go:embed charts/fluent-operator/*
//go:embed charts/fluent-operator/crds/*
//go:embed charts/fluent-operator/templates/*

//go:embed charts/edgewize-monitor/*
//go:embed charts/edgewize-monitor/crds/*
//go:embed charts/edgewize-monitor/templates/*
//go:embed charts/edgewize-monitor/templates/kube-state-metrics/*
//go:embed charts/edgewize-monitor/templates/kubernetes/*
//go:embed charts/edgewize-monitor/templates/prometheus/*
//go:embed charts/edgewize-monitor/templates/prometheus-operator/*
//go:embed charts/edgewize-monitor/templates/whizard-agent-proxy/*

var FS embed.FS

// BuiltinOrDir returns a FS for the provided directory. If no directory is passed, the compiled in
// FS will be used
func BuiltinOrDir(dir string) fs.FS {
	if dir == "" {
		return FS
	}
	return os.DirFS(dir)
}