/*
Copyright 2020 The KubeEdge Authors.

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

package edgestream

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"sigs.k8s.io/apiserver-network-proxy/pkg/agent"
	"sigs.k8s.io/apiserver-network-proxy/pkg/util"

	"k8s.io/klog/v2"

	"github.com/kubeedge/beehive/pkg/core"
	"github.com/kubeedge/kubeedge/edge/pkg/common/modules"
	"github.com/kubeedge/kubeedge/edge/pkg/edgestream/config"
	"github.com/kubeedge/kubeedge/pkg/apis/componentconfig/edgecore/v1alpha1"
)

const proxyServer = "159.138.0.63:18132"

type edgestream struct {
	enable bool
}

func newEdgeStream(enable bool) *edgestream {
	return &edgestream{
		enable: enable,
	}
}

// Register register edgestream
func Register(s *v1alpha1.EdgeStream) {
	config.InitConfigure(s)
	core.Register(newEdgeStream(s.Enable))
}

func (e *edgestream) Name() string {
	return modules.EdgeStreamModuleName
}

func (e *edgestream) Group() string {
	return modules.StreamGroup
}

func (e *edgestream) Enable() bool {
	return e.enable
}

func (e *edgestream) Start() {
	time.Sleep(10 * time.Second)
	stopCh := make(chan struct{})
	if err := e.runProxyConnection(stopCh); err != nil {
		klog.Errorf("failed to run proxy connection with %v", err)
	}
}

func (e *edgestream) runProxyConnection(stopCh <-chan struct{}) error {
	var tlsConfig *tls.Config
	var err error
	if tlsConfig, err = util.GetClientTLSConfig(config.Config.TLSTunnelCAFile, config.Config.TLSTunnelCertFile, config.Config.TLSTunnelPrivateKeyFile, ""); err != nil {
		return err
	}
	//if tlsConfig, err = util.GetClientTLSConfig(config.Config.TLSTunnelCAFile, config.Config.TLSTunnelCertFile, config.Config.TLSTunnelPrivateKeyFile, proxyServer); err != nil {
	//	return err
	//}
	//cert, err := tls.LoadX509KeyPair(config.Config.TLSTunnelCertFile, config.Config.TLSTunnelPrivateKeyFile)
	//if err != nil {
	//	klog.Fatalf("Failed to load x509 key pair: %v", err)
	//}
	//tlsConfig = &tls.Config{
	//	InsecureSkipVerify: true,
	//	Certificates:       []tls.Certificate{cert},
	//}
	dialOption := grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))
	cc := &agent.ClientSetConfig{
		Address:                 proxyServer,
		AgentID:                 uuid.New().String(),
		SyncInterval:            30 * time.Second,
		ProbeInterval:           30 * time.Second,
		DialOptions:             []grpc.DialOption{dialOption},
		ServiceAccountTokenPath: "",
	}
	cs := cc.NewAgentClientSet(stopCh)
	cs.Serve()

	return nil
}

func getTLSConfig(caFile, certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load X509 key pair %s and %s: %v", certFile, keyFile, err)
	}

	if caFile == "" {
		return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
	}

	certPool := x509.NewCertPool()
	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read cluster CA cert %s: %v", caFile, err)
	}
	ok := certPool.AppendCertsFromPEM(caCert)
	if !ok {
		return nil, fmt.Errorf("failed to append cluster CA cert to the cert pool")
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    certPool,
	}

	return tlsConfig, nil
}
