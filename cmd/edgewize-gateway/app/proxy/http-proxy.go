package proxy

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"

	"k8s.io/klog/v2"

	"github.com/edgewize-io/edgewize/cmd/edgewize-gateway/app/options"
)

type HTTPProxyServer struct {
	sync.RWMutex
	backendServers *ServerEndpoints
	proxyPort      int
	listenPort     int
}

func NewHTTPProxyServer(opt *options.ServerRunOptions, proxyPort int, listenPort int, backendServers *ServerEndpoints) *HTTPProxyServer {
	return &HTTPProxyServer{
		backendServers: backendServers,
		proxyPort:      proxyPort,
		listenPort:     listenPort,
	}
}

func (s *HTTPProxyServer) Run(ctx context.Context) error {

	// Create HTTPS server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.listenPort),
		Handler: s,
	}

	// Listen for incoming HTTPS connections with TLS encryption and handle errors
	err := server.ListenAndServe()
	if err != nil {
		klog.Errorf("Error occurred while listening: %v", err)
		return err
	}
	return nil
}

func (s *HTTPProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		backend *url.URL
		err     error
	)

	// Forward request to other server
	backend, err = s.selectServer(r)
	if err != nil {
		klog.Errorf("error in select server:%v", err)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(backend)
	proxy.Director = func(request *http.Request) {
		// Set the request properties to the backend properties
		request.URL.Scheme = backend.Scheme
		request.URL.Host = backend.Host
		request.Host = backend.Host

		// Add the backend query to the request raw query
		backendQuery := backend.RawQuery
		if backendQuery == "" || request.URL.RawQuery == "" {
			request.URL.RawQuery = backendQuery + request.URL.RawQuery
		} else {
			request.URL.RawQuery = backendQuery + "&" + request.URL.RawQuery
		}
		// Add a default user agent if one is not specified
		if _, ok := request.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default value
			request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 6.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/88.0.4324.96 Safari/537.36")
		}
		klog.V(7).Infof("request.URL.Path：%s request.URL.RawQuery：%v", request.URL.Path, request.URL.RawQuery)
	}
	proxy.ModifyResponse = func(w *http.Response) error {
		r := w.Request
		responseDump, err := httputil.DumpResponse(w, true)
		if err != nil {
			return err
		}
		klog.V(8).Infoln(w.Status, r.URL.String(), base64.StdEncoding.EncodeToString(responseDump))
		return nil
	}

	// Create a new http transport with a custom TLS client configuration
	transport := &http.Transport{}
	proxy.Transport = transport
	proxy.ServeHTTP(w, r)
}

func (s *HTTPProxyServer) selectServer(r *http.Request) (*url.URL, error) {
	clusterName := r.Header.Get("cluster")

	clusterNameDecorate := fmt.Sprintf("otaserver-%s", clusterName)
	backend, ok := s.backendServers.Get(clusterNameDecorate)
	if !ok {
		klog.Errorf("can't find backend server for cluster(%s)", clusterName)
		return nil, fmt.Errorf("con't find backend server for cluster(%s)", clusterName)
	}
	backend = fmt.Sprintf("http://%s:%d", backend, s.proxyPort)
	ret, err := url.Parse(backend)
	if err != nil {
		klog.Errorf("parse backend server(%s) error: %v", backend, err)
	} else {
		klog.Infof("ret, %v", *ret)
	}
	return ret, err
}
