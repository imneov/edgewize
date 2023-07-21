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
	"context"
	"fmt"
	"net/http"

	"github.com/google/gops/agent"
	"github.com/spf13/cobra"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog"

	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/edgewize-io/edgewize/cmd/edgewize-gateway/app/options"
	"github.com/edgewize-io/edgewize/cmd/edgewize-gateway/app/proxy"
	"github.com/edgewize-io/edgewize/pkg/utils/term"
	"github.com/edgewize-io/edgewize/pkg/version"
)

func NewGatewayServerCommand() *cobra.Command {
	s := options.NewServerRunOptions()

	// Load configuration from file
	serversCfg, err := options.TryLoadFromDisk()
	if err == nil {
		s.ServerEndpoints = *serversCfg
	} else {
		klog.Fatal("Failed to load configuration from disk", err)
	}

	cmd := &cobra.Command{
		Use:  "edgewize-gateway",
		Long: `The Edgewize gateway exposes multiple virtual control plane interfaces to the external ports by reusing ports.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if errs := s.Validate(); len(errs) != 0 {
				return utilerrors.NewAggregate(errs)
			}

			if s.GOPSEnabled {
				// Add agent to report additional information such as the current stack trace, Go version, memory stats, etc.
				// Bind to a random port on address 127.0.0.1.
				if err := agent.Listen(agent.Options{}); err != nil {
					klog.Fatal(err)
				}
			}

			return Run(s, options.WatchConfigChange(), signals.SetupSignalHandler())
		},
		SilenceUsage: true,
	}

	fs := cmd.Flags()
	namedFlagSets := s.Flags()
	for _, f := range namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}

	usageFmt := "Usage:\n  %s\n"
	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStdout(), namedFlagSets, cols)
	})

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version of Edgewize gateway",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println(version.Get())
		},
	}

	cmd.AddCommand(versionCmd)

	return cmd
}

func Run(s *options.ServerRunOptions, configCh <-chan options.ServerEndpoints, ctx context.Context) error {
	serverEndpoints, err := options.TryLoadFromDisk()
	if err != nil {
		return err
	}
	backendServers := proxy.NewServers()
	backendServers.LoadConfig(*serverEndpoints)

	ictx, cancelFunc := context.WithCancel(context.TODO())
	errCh := make(chan error)
	defer close(errCh)
	go func() {
		if err := run(s, ictx, backendServers); err != nil {
			errCh <- err
		}
	}()

	// The ctx (signals.SetupSignalHandler()) is to control the entire program life cycle,
	// The ictx(internal context)  is created here to control the life cycle of the ks-apiserver(http server, sharedInformer etc.)
	// when config change, stop server and renew context, start new server
	for {
		select {
		case <-ctx.Done():
			cancelFunc()
			return nil
		case cfg := <-configCh:
			backendServers.LoadConfig(cfg)
		case err := <-errCh:
			cancelFunc()
			return err
		}
	}
}

func run(s *options.ServerRunOptions, ctx context.Context, backendServers *proxy.ServerEndpoints) error {

	errCh := make(chan error)
	defer close(errCh)
	go func() {
		hubhttpserver := proxy.NewHTTPSProxyServer(s, 30002, backendServers) // ws
		err := hubhttpserver.Run(ctx)
		if err == http.ErrServerClosed {
			klog.Errorf("error in hubhttpserver: %v", err)
			return
		}
		errCh <- err
	}()
	go func() {
		hubserver := proxy.NewWebsocketProxyServer(s, 30000, backendServers) // ws
		err := hubserver.Run(ctx)
		if err == http.ErrServerClosed {
			klog.Errorf("error in hubserver: %v", err)
			return
		}
		errCh <- err
	}()
	go func() {
		tunnelserver := proxy.NewWebsocketProxyServer(s, 30004, backendServers) // ws
		err := tunnelserver.Run(ctx)
		if err == http.ErrServerClosed {
			klog.Errorf("error in tunnelserver: %v", err)
			return
		}
		errCh <- err
	}()
	go func() {
		otahttpserver := proxy.NewHTTPProxyServer(s, 80, 30080, backendServers) // ws
		err := otahttpserver.Run(ctx)
		if err == http.ErrServerClosed {
			klog.Errorf("error in hubhttpserver: %v", err)
			return
		}
		errCh <- err
	}()

	return <-errCh
}
