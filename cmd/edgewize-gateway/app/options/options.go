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

package options

import (
	"flag"
	"strings"

	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog"
)

const (
	CertDir        = "/etc/edgewize/gateway-certs/"
	ServerCAFile   = "server.ca"
	ServerCertFile = "server.crt"
	ServerKeyFile  = "server.key"
	ClientCertFile = "client.crt"
	ClientKeyFile  = "client.key"
)

type ServerRunOptions struct {
	ServersConfigFile string
	Servers           string
	CertDir           string
	GOPSEnabled       bool
	ServerEndpoints   ServerEndpoints
}

func NewServerRunOptions() *ServerRunOptions {
	s := &ServerRunOptions{
		ServerEndpoints: ServerEndpoints{},
		CertDir:         CertDir,
		GOPSEnabled:     true,
	}

	return s
}

func (s *ServerRunOptions) Flags() (fss cliflag.NamedFlagSets) {
	fs := fss.FlagSet("generic")

	fs = fss.FlagSet("klog")
	local := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(local)
	local.VisitAll(func(fl *flag.Flag) {
		fl.Name = strings.Replace(fl.Name, "_", "-", -1)
		fs.AddGoFlag(fl)
	})

	edgefs := fss.FlagSet("edgewize")
	edgefs.StringVar(&s.CertDir, "cert-dir", s.CertDir, ""+
		"Certificate directory used to setup Edgewize gateway, need server.crt, server.key, server.ca, client.crt and client.key placed inside."+
		"if not set, webhook server would look up the server key and certificate in"+
		"/etc/edgewize/gateway-certs")
	edgefs.StringVar(&s.ServersConfigFile, "servers-config", s.ServersConfigFile, ""+
		"Certificate directory used to setup Edgewize gateway, need server.crt, server.key, server.ca, client.crt and client.key placed inside."+
		"if not set, webhook server would look up the server key and certificate in"+
		"/etc/edgewize/gateway-certs")

	return fss
}

// MergeConfig merge new config without validation
// When misconfigured, the app should just crash directly
func (s *ServerRunOptions) MergeConfig(cfg *ServerEndpoints) {

}