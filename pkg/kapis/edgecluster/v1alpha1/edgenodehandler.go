package v1alpha1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/emicklei/go-restful"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

const (
	KubeEdgeNamespace           = "kubeedge"
	KubeEdgeCloudCoreConfigName = "cloudcore"
	KubeEdgeTokenSecretName     = "tokensecret"
	CloudCoreService            = "cloudcore"
	StatusSucceeded             = "Succeeded"
	StatusFailure               = "Failure"
)

type CloudCoreConfig struct {
	Modules *Modules `json:"modules,omitempty"`
}

type Modules struct {
	CloudHub    *CloudHub    `yaml:"cloudHub,omitempty"`
	CloudStream *CloudStream `yaml:"cloudStream,omitempty"`
}

type CloudHub struct {
	Quic             *CloudHubQUIC      `yaml:"quic,omitempty"`
	WebSocket        *CloudHubWebSocket `json:"websocket,omitempty"`
	HTTPS            *CloudHubHTTPS     `json:"https,omitempty"`
	AdvertiseAddress []string           `yaml:"advertiseAddress,omitempty"`
}

type CloudHubQUIC struct {
	Port int32 `yaml:"port,omitempty"`
}

type CloudHubWebSocket struct {
	Port int32 `yaml:"port,omitempty"`
}

type CloudHubHTTPS struct {
	Port int32 `yaml:"port,omitempty"`
}

type CloudStream struct {
	TunnelPort int32 `yaml:"tunnelPort,omitempty"`
}

type EdgeJoinResponse struct {
	Code    uint32 `json:"code,omitempty"`
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
	Data    string `json:"data,omitempty"`
}

func (h *handler) joinNode(request *restful.Request, response *restful.Response) {
	nodeName := request.QueryParameter("node_name")
	version := request.QueryParameter("version")
	runtime := request.QueryParameter("runtime")
	imageRepository := request.QueryParameter("image-repository")
	hasDefaultTaint, _ := strconv.ParseBool(request.QueryParameter("add_default_taint"))
	//withNodePort, _ := strconv.ParseBool(request.QueryParameter("with_nodeport"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//Validate Node name
	msgs := validation.NameIsDNSSubdomain(nodeName, false)
	if len(msgs) != 0 {
		klog.Infof("EdgeNodeJoin: Invalid node name: %s\n", msgs[0])
		response.AddHeader("Content-Type", "text/json")
		response.WriteHeader(http.StatusBadRequest)
		response.WriteAsJson(&EdgeJoinResponse{
			Code:    http.StatusBadRequest,
			Status:  StatusFailure,
			Message: fmt.Sprintf("Invalid node name: %s", msgs[0]),
		})
		return
	}

	//Check Node name and IP used
	nodeList, err := h.k8sclient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Infof("EdgeNodeJoin: List nodes error [+%v]\n", err)
		response.AddHeader("Content-Type", "text/json")
		response.WriteHeader(http.StatusInternalServerError)
		response.WriteAsJson(&EdgeJoinResponse{
			Code:    http.StatusInternalServerError,
			Status:  StatusFailure,
			Message: fmt.Sprintf("List nodes error [+%v]", err),
		})
		return
	}

	nodeNames := make(map[string]bool, 0)
	for _, n := range nodeList.Items {
		nodeNames[n.Name] = true
	}

	_, ok := nodeNames[nodeName]
	if ok {
		klog.Infof("EdgeNodeJoin: Node name %s in use\n", nodeName)
		response.AddHeader("Content-Type", "text/json")
		response.WriteHeader(http.StatusBadRequest)
		response.WriteAsJson(&EdgeJoinResponse{
			Code:    http.StatusBadRequest,
			Status:  StatusFailure,
			Message: fmt.Sprintf("Node name %s in use", nodeName),
		})
		return
	}

	// Get secret
	secret, err := h.k8sclient.CoreV1().Secrets(KubeEdgeNamespace).Get(ctx, KubeEdgeTokenSecretName, metav1.GetOptions{})
	if err != nil {
		klog.Infof("EdgeNodeJoin: Read cloudcore token secret error [+%v]\n", err)
		response.AddHeader("Content-Type", "text/json")
		response.WriteHeader(http.StatusInternalServerError)
		response.WriteAsJson(&EdgeJoinResponse{
			Code:    http.StatusInternalServerError,
			Status:  StatusFailure,
			Message: fmt.Sprintf("Read cloudcore token secret error [+%v]", err),
		})
		return
	}

	if version == "" {
		version = "v1.13.0"
	} else if version[0] != 'v' {
		version = fmt.Sprintf("v%s", version)
	}

	uri := fmt.Sprintf("https://kubeedge.pek3b.qingstor.com/bin/%s/$arch/keadm-%s-linux-$arch.tar.gz", version, version)

	// Get configmap for cloudcore
	configMap, err := h.k8sclient.CoreV1().ConfigMaps(KubeEdgeNamespace).Get(ctx, KubeEdgeCloudCoreConfigName, metav1.GetOptions{})
	if err != nil {
		klog.Infof("EdgeNodeJoin: Read cloudcore configmap error [+%v]\n", err)
		response.AddHeader("Content-Type", "text/json")
		response.WriteHeader(http.StatusInternalServerError)
		response.WriteAsJson(&EdgeJoinResponse{
			Code:    http.StatusInternalServerError,
			Status:  StatusFailure,
			Message: fmt.Sprintf("Read cloudcore configmap error [+%v]", err),
		})
		return
	}

	var cloudCoreConfig CloudCoreConfig
	err = yaml.Unmarshal([]byte(configMap.Data["cloudcore.yaml"]), &cloudCoreConfig)
	if err != nil {
		klog.Infof("EdgeNodeJoin: Unmarshal cloudcore configmap error [+%v]\n", err)
		response.AddHeader("Content-Type", "text/json")
		response.WriteHeader(http.StatusInternalServerError)
		response.WriteAsJson(&EdgeJoinResponse{
			Code:    http.StatusInternalServerError,
			Status:  StatusFailure,
			Message: fmt.Sprintf("Unmarshal cloudcore configmap error [+%v]", err),
		})
		return
	}
	modules := cloudCoreConfig.Modules
	advertiseAddress := modules.CloudHub.AdvertiseAddress[0]
	webSocketPort := modules.CloudHub.WebSocket.Port
	quicPort := modules.CloudHub.Quic.Port
	certPort := modules.CloudHub.HTTPS.Port
	tunnelPort := modules.CloudStream.TunnelPort

	ccService, err := h.k8sclient.CoreV1().Services(KubeEdgeNamespace).Get(ctx, CloudCoreService, metav1.GetOptions{})
	if err != nil {
		klog.Infof("EdgeNodeJoin: Read cloudcore service error [+%v]\n", err)
		response.AddHeader("Content-Type", "text/json")
		response.WriteHeader(http.StatusInternalServerError)
		response.WriteAsJson(&EdgeJoinResponse{
			Code:    http.StatusInternalServerError,
			Status:  StatusFailure,
			Message: fmt.Sprintf("Read cloudcore service error [+%v]", err),
		})
		return
	}

	ports := ccService.Spec.Ports
	for _, port := range ports {
		switch port.Name {
		case "cloudhub":
			{
				webSocketPort = port.Port
			}
		case "cloudhub-quic":
			{
				quicPort = port.Port
			}
		case "cloudhub-https":
			{
				certPort = port.Port
			}
		case "tunnelport":
			{
				tunnelPort = port.Port
			}
		}
	}

	var cmd string
	var withEdgeTaint string
	if hasDefaultTaint {
		withEdgeTaint = " --with-edge-taint"
	}
	cmd = fmt.Sprintf("arch=$(uname -m); curl -LO %s  && tar xvf keadm-%s-linux-$arch.tar.gz && chmod +x keadm && ./keadm join --kubeedge-version=%s --cloudcore-ipport=%s:%d --quicport %d --certport %d --tunnelport %d --edgenode-name %s --token %s%s ", uri, version, strings.ReplaceAll(version, "v", ""), advertiseAddress, webSocketPort, quicPort, certPort, tunnelPort, nodeName, string(secret.Data["tokendata"]), withEdgeTaint)
	if runtime == "docker" {
		cmd = cmd + "--remote-runtime-endpoint=unix:///var/run/dockershim.sock --runtimetype=docker"
	}
	if imageRepository != "" {
		cmd = fmt.Sprintf("%s --image-repository=%s ", cmd, imageRepository)
	}
	resp := EdgeJoinResponse{
		Code:   http.StatusOK,
		Status: StatusSucceeded,
		Data:   cmd,
	}
	bf := bytes.NewBufferString("")
	jsonEncoder := json.NewEncoder(bf)
	jsonEncoder.SetEscapeHTML(false)
	jsonEncoder.Encode(resp)

	response.AddHeader("Content-Type", "text/json")
	response.WriteHeader(http.StatusOK)
	response.Write(bf.Bytes())
}
