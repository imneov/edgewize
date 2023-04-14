/*
Copyright 2020 The KubeSphere Authors.

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

package config

import (
	"fmt"
	"os"
	"testing"

	"github.com/edgewize-io/edgewize/pkg/simple/client/alerting"
	"github.com/edgewize-io/edgewize/pkg/simple/client/edgewize"
	"github.com/edgewize-io/edgewize/pkg/simple/client/monitoring/prometheus"
	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v2"

	"github.com/edgewize-io/edgewize/pkg/models/terminal"
	"github.com/edgewize-io/edgewize/pkg/simple/client/cache"
	"github.com/edgewize-io/edgewize/pkg/simple/client/k8s"
)

func newTestConfig() (*Config, error) {
	var conf = &Config{

		KubernetesOptions: &k8s.KubernetesOptions{
			KubeConfig: "/Users/zry/.kube/config",
			Master:     "https://127.0.0.1:6443",
			QPS:        1e6,
			Burst:      1e6,
		},
		CacheOptions: &cache.Options{
			Type:    "redis",
			Options: map[string]interface{}{},
		},
		EdgeWizeOptions: edgewize.NewOptions(),
		TerminalOptions: &terminal.Options{
			Image:   "alpine:3.15",
			Timeout: 600,
		},
	}
	return conf, nil
}

func saveTestConfig(t *testing.T, conf *Config) {
	content, err := yaml.Marshal(conf)
	if err != nil {
		t.Fatalf("error marshal config. %v", err)
	}
	err = os.WriteFile(fmt.Sprintf("%s.yaml", defaultConfigurationName), content, 0640)
	if err != nil {
		t.Fatalf("error write configuration file, %v", err)
	}
}

func cleanTestConfig(t *testing.T) {
	file := fmt.Sprintf("%s.yaml", defaultConfigurationName)
	if _, err := os.Stat(file); os.IsNotExist(err) {
		t.Log("file not exists, skipping")
		return
	}

	err := os.Remove(file)
	if err != nil {
		t.Fatalf("remove %s file failed", file)
	}

}

func TestGet(t *testing.T) {
	conf, err := newTestConfig()
	if err != nil {
		t.Fatal(err)
	}
	saveTestConfig(t, conf)
	defer cleanTestConfig(t)

	conf2, err := TryLoadFromDisk()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(conf, conf2); diff != "" {
		t.Fatal(diff)
	}
}

func TestStripEmptyOptions(t *testing.T) {
	var config Config

	config.CacheOptions = &cache.Options{Type: ""}
	config.MonitoringOptions = &prometheus.Options{Endpoint: ""}
	config.EdgeWizeOptions = &edgewize.Options{}
	config.AlertingOptions = &alerting.Options{
		Endpoint:            "",
		PrometheusEndpoint:  "",
		ThanosRulerEndpoint: "",
	}
	config.stripEmptyOptions()

	if config.CacheOptions != nil ||
		config.EdgeWizeOptions != nil {
		t.Fatal("config stripEmptyOptions failed")
	}
}
