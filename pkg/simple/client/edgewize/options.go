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

package edgewize

import (
	"github.com/spf13/pflag"
)

type Options struct {
	Gateway *Gateway `json:"gateway,omitempty" yaml:"gateway,omitempty"`
}

type Gateway struct {
	AdvertiseAddress []string `json:"advertiseAddress,omitempty" yaml:"advertiseAddress,omitempty"`
	DNSNames         []string `json:"dnsNames,omitempty" yaml:"dnsNames,omitempty"`
}

// NewOptions returns a default nil options
func NewOptions() *Options {
	return &Options{}
}

func (o *Options) Validate() []error {
	errors := make([]error, 0)

	return errors
}

func (o *Options) AddFlags(fs *pflag.FlagSet, s *Options) {

}
