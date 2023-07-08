package proxy

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"github.com/golang-jwt/jwt"
	"k8s.io/klog"

	"github.com/edgewize-io/edgewize/cmd/edgewize-gateway/app/options"
)

const (
	ClusterName string = "clustername"
	ClusterType string = "clustertype"
)

type HTTPProxyServer struct {
	sync.RWMutex
	backendServers                              *ServerEndpoints
	proxyPort                                   int
	serverCAFile, serverCertFile, serverKeyFile string
	clientCertFile, clientKeyFile               string
}

func NewHTTPProxyServer(opt *options.ServerRunOptions, proxyPort int, backendServers *ServerEndpoints) *HTTPProxyServer {
	return &HTTPProxyServer{
		backendServers: backendServers,
		proxyPort:      proxyPort,
		serverCAFile:   opt.CertDir + options.ServerCAFile,
		serverCertFile: opt.CertDir + options.ServerCertFile,
		serverKeyFile:  opt.CertDir + options.ServerKeyFile,
		clientCertFile: opt.CertDir + options.ClientCertFile,
		clientKeyFile:  opt.CertDir + options.ClientKeyFile,
	}
}

func (s *HTTPProxyServer) Run(ctx context.Context) error {
	klog.Infof("port: %d, certs: %s, %s, %s, %s", s.proxyPort, s.serverCertFile, s.clientCertFile, s.serverKeyFile, s.clientKeyFile)

	// Load the backend server certificate and private key
	serverCert, err := LoadX509KeyFromFile(s.serverCertFile, s.serverKeyFile)
	if err != nil {
		err := fmt.Errorf("Error loading server certificate and private key (%s,%s): %v", s.serverCertFile, s.serverKeyFile, err)
		return err
	}

	// Create a TLS configuration with mutual authentication
	tlsConfig := &tls.Config{
		ClientAuth:         tls.RequestClientCert,
		Certificates:       []tls.Certificate{serverCert},
		InsecureSkipVerify: true,
	}

	// Create HTTPS server
	server := &http.Server{
		Addr:      fmt.Sprintf(":%d", s.proxyPort),
		TLSConfig: tlsConfig,
		Handler:   s,
	}

	// Listen for incoming HTTPS connections with TLS encryption and handle errors
	err = server.ListenAndServeTLS("", "")
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

	// Load the client certificate and private key
	cliCert, err := LoadX509KeyFromFile(s.clientCertFile, s.clientKeyFile)
	if err != nil {
		klog.Error("error load client certificate and private key", err)
		return
	}

	// Create a new http transport with a custom TLS client configuration
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			// Skip verification of the server's certificate chain
			InsecureSkipVerify: true,
			// Set the client certificate to use for authentication
			Certificates: []tls.Certificate{cliCert},
		},
	}
	proxy.Transport = transport
	proxy.ServeHTTP(w, r)
}

func (s *HTTPProxyServer) selectServer(r *http.Request) (*url.URL, error) {
	authorizationHeader := r.Header.Get("authorization")

	klog.V(7).Infof("authorizationHeader  %v", authorizationHeader)

	bearerToken := strings.Split(authorizationHeader, " ")
	if len(bearerToken) != 2 {
		err := fmt.Errorf("bearerToken(%s) size error, got %d want 2", bearerToken, len(bearerToken))
		klog.Error(err)
		return nil, err
	}
	token, err := jwt.Parse(bearerToken[1], func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("there was an error")
		}
		return nil, nil
	})
	klog.V(3).Infof("authorizationHeader(%v) token(%v) %v", authorizationHeader, token, err)
	clusterName := getHeaderString(token.Header, ClusterName)
	clusterType := getHeaderString(token.Header, ClusterType)
	if clusterType == "EdgeCluster" {
		backend, ok := s.backendServers.Get(clusterName)
		if !ok {
			klog.Errorf("can't find backend server for cluster(%s)", clusterName)
			return nil, fmt.Errorf("con't find backend server for cluster(%s)", clusterName)
		}
		backend = fmt.Sprintf("https://%s:%d", backend, s.proxyPort)
		ret, err := url.Parse(backend)
		if err != nil {
			klog.Errorf("parse backend server(%s) error: %v", backend, err)
		} else {
			klog.Infof("ret, %v", *ret)
		}
		return ret, err
	}
	klog.Errorf("unsupport cluster type(%s)", clusterType)
	return nil, errors.New("unsupport cluster type")
}

func getHeaderString(header map[string]interface{}, key string) (val string) {
	ret, ok := header[key]
	if !ok {
		return ""
	}
	res, ok := ret.(string)
	if !ok {
		return ""
	}
	return res
}
