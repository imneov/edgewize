package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/edgewize-io/edgewize/cmd/edgewize-gateway/app/options"
	"github.com/golang-jwt/jwt"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/klog"
)

type HTTPProxyServer struct {
	sync.RWMutex
	backendServers                              map[string]string
	proxyPort                                   int
	serverCAFile, serverCertFile, serverKeyFile string
	clientCertFile, clientKeyFile               string
}

func NewHTTPProxyServer(opt *options.ServerRunOptions, proxyPort int, backendServers map[string]string) *HTTPProxyServer {
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

	// Load the client CA certificate
	caCert, err := os.ReadFile(s.serverCAFile)
	if err != nil {
		err := fmt.Errorf("Error loading server certificate and private key (%s): %v", s.serverCAFile, err)
		return err
	}
	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{Type: certutil.CertificateBlockType, Bytes: caCert}))
	if !ok {
		err := fmt.Errorf("failed to append CA cert")
		return err
	}

	// Create a TLS configuration with mutual authentication
	tlsConfig := &tls.Config{
		ClientAuth: tls.RequestClientCert,
		// ClientCAs:    caCertPool,
		Certificates: []tls.Certificate{serverCert},
		// MinVersion:   tls.VersionTLS12,
		// CipherSuites: []uint16{
		// 	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		// 	tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
		// },
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
	if strings.HasPrefix(r.URL.Path, "/ca.crt") {
		backend, err = url.Parse("https://10.233.50.204:30002/")
	} else {
		// Forward request to other server
		backend, err = s.selectServer(r)
		if err != nil {
			klog.Errorf("error in select server:%w", err)
			return
		}
	}

	// // Forward request to other server
	// backend, err := s.selectServer(r)
	// if err != nil {
	// 	klog.Errorf("error in select server:%w", err)
	// 	return
	// }

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
		klog.V(3).Infoln(w.Status, r.URL.String())
		return nil
	}

	// Load the client certificate and private key
	cliCert, err := LoadX509KeyFromFile(s.clientCertFile, s.clientKeyFile)
	if err != nil {
		klog.Errorf("error load client certificate and private key", err)
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

	klog.V(3).Infof("authorizationHeader  %v", authorizationHeader)

	bearerToken := strings.Split(authorizationHeader, " ")
	if len(bearerToken) != 2 {
		klog.Error("bearerToken size")
		return nil, nil
	}
	token, err := jwt.Parse(bearerToken[1], func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("there was an error")
		}
		klog.Error("bearerToken size")
		return nil, nil
	})
	klog.V(3).Infof("authorizationHeader(%v) token(%v)", authorizationHeader, token)

	// Print client certificate information
	cert := r.TLS.PeerCertificates[0]
	byt, err := json.Marshal(cert)
	if err != nil {
		klog.Errorf("error in decode cert")
		return nil, err
	}
	klog.V(6).Infof("Received request from %v with certificate %v\n", r.RemoteAddr, cert.Subject.CommonName)
	klog.V(8).Infof("err: %v json: %v\n", err, string(byt))

	s.RLock()
	defer s.RUnlock()
	CN := cert.Subject.CommonName
	names := strings.Split(CN, ".")
	clusterName := ""
	if len(names) == 2 {
		clusterName = names[1]
	} else {
		klog.Errorf("unknown cluster name, CommonName: %s", CN)
		return nil, errors.New("unknown cluster")
	}
	backend, ok := s.backendServers[clusterName]
	if !ok {
		klog.Errorf("can't find backend server for CN(%s)", CN)
		return nil, fmt.Errorf("con't find backend server for CN(%s)", CN)
	}
	backend = fmt.Sprintf("%s:%d", backend, s.proxyPort)
	ret, err := url.Parse(backend)
	if err != nil {
		klog.Errorf("parse backend server(%s) error: %w", backend, err)
	}
	klog.Infof("ret, %v", *ret)
	return ret, err
}
