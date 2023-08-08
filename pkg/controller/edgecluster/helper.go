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

package edgecluster

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	infrav1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/infra/v1alpha1"
	"math"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"

	"github.com/edgewize-io/edgewize/pkg/helm"
)

func needInstall(status release.Status) bool {
	if status == release.StatusUnknown || status == release.StatusUninstalled {
		return true
	}
	return false
}

func needUpgrade(status release.Status) bool {
	if status == release.StatusDeployed || status == release.StatusFailed {
		return true
	}
	return false
}

func InstallChart(file, name, namespace, kubeconfig string, createNamespace bool, values chartutil.Values) (infrav1alpha1.Status, error) {
	oldStatus, err := helm.Status(file, name, namespace, kubeconfig)
	if err != nil {
		return "", err
	}
	klog.V(3).Infof("current chart info, chart: %s, status: %s, kubeconfig: %s, values: %v,", name, oldStatus, kubeconfig, values)
	if needInstall(oldStatus) {
		klog.V(3).Infof("begin to install chart, chart: %s, kubeconfig: %s", name, kubeconfig)
		err = helm.Install(file, name, namespace, kubeconfig, values)
		if err != nil {
			klog.Errorf("install chart error, err: %v", err)
			return "", err
		}
		klog.V(3).Infof("install chart success, chart: %s, kubeconfig: %s", name, kubeconfig)
	}
	time.Sleep(time.Second)
	newStatus, err := helm.Status(file, name, namespace, kubeconfig)
	if err != nil {
		return "", err
	}
	klog.V(3).Infof("current chart info, chart: %s, status: %s, kubeconfig: %s, values: %v,", name, newStatus, kubeconfig, values)
	switch newStatus {
	case release.StatusUninstalling:
		return infrav1alpha1.UninstallingStatus, nil
	case release.StatusDeployed:
		return infrav1alpha1.RunningStatus, nil
	case release.StatusFailed:
		return infrav1alpha1.ErrorStatus, nil
	case release.StatusPendingInstall, release.StatusPendingUpgrade, release.StatusPendingRollback:
		return infrav1alpha1.PendingStatus, nil
	default:
		return "", fmt.Errorf("invalid status: %s", newStatus)
	}
}

func UpgradeChart(file, name, namespace, kubeconfig string, values chartutil.Values, upgrade bool) (infrav1alpha1.Status, error) {
	oldStatus, err := helm.Status(file, name, namespace, kubeconfig)
	if err != nil {
		return "", err
	}
	klog.V(3).Infof("current chart info, chart: %s, status: %s, kubeconfig: %s, values: %v,", name, oldStatus, kubeconfig, values)
	if needInstall(oldStatus) {
		klog.V(3).Infof("begin to install chart, chart: %s, kubeconfig: %s", name, kubeconfig)
		err = helm.Install(file, name, namespace, kubeconfig, values)
		if err != nil {
			klog.Errorf("install chart error, err: %v", err)
			return "", err
		}
		klog.V(3).Infof("install chart success, chart: %s, kubeconfig: %s", name, kubeconfig)
	} else if upgrade && needUpgrade(oldStatus) {
		klog.V(3).Infof("begin to upgrade chart, chart: %s, kubeconfig: %s", name, kubeconfig)
		err = helm.Upgrade(file, name, namespace, kubeconfig, values)
		if err != nil {
			klog.Errorf("upgrade chart error, err: %v", err)
			return "", err
		}
		klog.V(3).Infof("upgrade chart success, chart: %s, kubeconfig: %s", name, kubeconfig)
	}
	time.Sleep(time.Second)
	newStatus, err := helm.Status(file, name, namespace, kubeconfig)
	if err != nil {
		return "", err
	}
	klog.V(3).Infof("current chart info, chart: %s, status: %s, kubeconfig: %s, values: %v,", name, newStatus, kubeconfig, values)
	switch newStatus {
	case release.StatusUninstalling:
		return infrav1alpha1.UninstallingStatus, nil
	case release.StatusDeployed:
		return infrav1alpha1.RunningStatus, nil
	case release.StatusFailed:
		return infrav1alpha1.ErrorStatus, nil
	case release.StatusPendingInstall, release.StatusPendingUpgrade, release.StatusPendingRollback:
		return infrav1alpha1.PendingStatus, nil
	default:
		return "", fmt.Errorf("invalid status: %s", newStatus)
	}
}

func SaveToLocal(name string, config []byte) error {
	path := filepath.Join(homedir.HomeDir(), ".kube", name)
	return os.WriteFile(path, config, 0644)
}

func SignCloudCoreCert(cacrt, cakey []byte) ([]byte, []byte, error) {
	cfg := &certutil.Config{
		CommonName:   "EdgeWize",
		Organization: []string{"EdgeWize"},
		Usages: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		AltNames: certutil.AltNames{
			IPs: []net.IP{}, // TODO
		},
	}
	var keyDER []byte
	caCert, err := x509.ParseCertificate(cacrt)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse a caCert from the given ASN.1 DER data, err: %v", err)
	}
	serverKey, err := NewPrivateKey(caCert.SignatureAlgorithm)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate a privateKey, err: %v", err)
	}
	var caKey crypto.Signer
	switch caCert.SignatureAlgorithm {
	case x509.ECDSAWithSHA256:
		caKey, err = x509.ParseECPrivateKey(cakey)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse ECPrivateKey, err: %v", err)
		}
		keyDER, err = x509.MarshalECPrivateKey(serverKey.(*ecdsa.PrivateKey))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to convert an EC private key to SEC 1, ASN.1 DER form, err: %v", err)
		}
	case x509.SHA256WithRSA:
		caKey, err = x509.ParsePKCS1PrivateKey(cakey)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse PKCS1PrivateKey, err: %v", err)
		}
		keyDER = x509.MarshalPKCS1PrivateKey(serverKey.(*rsa.PrivateKey))
	default:
		return nil, nil, fmt.Errorf("unsupport signature algorithm: %s", caCert.SignatureAlgorithm.String())
	}

	certDER, err := NewCertFromCa(cfg, caCert, serverKey.Public(), caKey, 365*100)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate a certificate using the given CA certificate and key, err: %v", err)
	}
	return certDER, keyDER, nil
}

func NewPrivateKey(algorithm x509.SignatureAlgorithm) (crypto.Signer, error) {
	switch algorithm {
	case x509.ECDSAWithSHA256:
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case x509.SHA256WithRSA:
		return rsa.GenerateKey(rand.Reader, 2048)
	default:
		return nil, fmt.Errorf("unsepport signature algorithm: %s", algorithm.String())
	}
}

// NewCertFromCa creates a signed certificate using the given CA certificate and key
func NewCertFromCa(cfg *certutil.Config, caCert *x509.Certificate, serverKey crypto.PublicKey, caKey crypto.Signer, validalityPeriod time.Duration) ([]byte, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}
	if len(cfg.CommonName) == 0 {
		return nil, errors.New("must specify a CommonName")
	}
	if len(cfg.Usages) == 0 {
		return nil, errors.New("must specify at least one ExtKeyUsage")
	}

	certTmpl := x509.Certificate{
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: cfg.Organization,
		},
		DNSNames:     cfg.AltNames.DNSNames,
		IPAddresses:  cfg.AltNames.IPs,
		SerialNumber: serial,
		NotBefore:    time.Now().UTC(),
		NotAfter:     time.Now().Add(time.Hour * 24 * validalityPeriod),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  cfg.Usages,
	}
	certDERBytes, err := x509.CreateCertificate(rand.Reader, &certTmpl, caCert, serverKey, caKey)
	if err != nil {
		return nil, err
	}
	return certDERBytes, nil
}
