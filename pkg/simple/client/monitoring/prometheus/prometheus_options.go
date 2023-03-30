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

package prometheus

import (
	"github.com/spf13/pflag"

	"github.com/prometheus/common/config"
)

type Options struct {
	EnableGPUMonitoring bool   `json:"enableGPUMonitoring,omitempty" yaml:"enableGPUMonitoring"`
	Endpoint            string `json:"endpoint,omitempty" yaml:"endpoint"`
	// HTTPClientConfig configures an HTTP client. mapstructure is not explicitly declared, use camel-case configuration.
	HTTPClientConfig config.HTTPClientConfig `json:"httpClientConfig,omitempty" yaml:"httpClientConfig,omitempty"`
}

func NewPrometheusOptions() *Options {
	return &Options{
		Endpoint:            "",
		EnableGPUMonitoring: true,
	}
}

func (s *Options) Validate() []error {
	var errs []error
	return errs
}

func (s *Options) ApplyTo(options *Options) {
	if s.Endpoint != "" {
		options.Endpoint = s.Endpoint
	}
	options.EnableGPUMonitoring = s.EnableGPUMonitoring
}

func (s *Options) AddFlags(fs *pflag.FlagSet, c *Options) {
	fs.BoolVar(&s.EnableGPUMonitoring, "enable-gpu-monitoring-query", c.EnableGPUMonitoring, ""+
		"The switch to enable/disable the GPU-related metrics query.")
	fs.StringVar(&s.Endpoint, "prometheus-endpoint", c.Endpoint, ""+
		"Prometheus service endpoint which stores KubeSphere monitoring data, if left "+
		"blank, will use builtin metrics-server as data source.")
}
