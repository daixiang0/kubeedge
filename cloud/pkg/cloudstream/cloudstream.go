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

package cloudstream

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"sync"

	"github.com/kubeedge/kubeedge/pkg/apis/componentconfig/cloudcore/v1alpha1"

	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/kubeedge/beehive/pkg/core"
	"github.com/kubeedge/kubeedge/cloud/pkg/cloudstream/config"
	"github.com/kubeedge/kubeedge/cloud/pkg/common/modules"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"k8s.io/klog/v2"
	"sigs.k8s.io/apiserver-network-proxy/konnectivity-client/proto/client"
	anpserver "sigs.k8s.io/apiserver-network-proxy/pkg/server"
	"sigs.k8s.io/apiserver-network-proxy/proto/agent"
)

var udsListenerLock sync.Mutex

const udsName = "/var/lib/kubeedge/proxy.sock"

type cloudStream struct {
	enable bool
}

func newCloudStream(enable bool) *cloudStream {
	return &cloudStream{
		enable: true,
	}
}

func Register(controller *v1alpha1.CloudStream) {
	config.InitConfigure(controller)
	core.Register(newCloudStream(true))
}

func (c *cloudStream) Name() string {
	return modules.CloudStreamModuleName
}

func (c *cloudStream) Group() string {
	return modules.CloudStreamGroupName
}

func (c *cloudStream) Start() {
	time.Sleep(10 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	proxyServer := anpserver.NewProxyServer(uuid.New().String(), 1, &anpserver.AgentTokenAuthenticationOptions{})
	klog.V(1).Infoln("Starting master server for client connections.")

	masterStop, err := c.runMasterServer(ctx, proxyServer)
	if err != nil {
		klog.Errorf("failed to run the master server: %v", err)
	}

	klog.V(1).Infoln("Starting agent server for tunnel connections.")
	err = c.runAgentServer(proxyServer)
	if err != nil {
		klog.Errorf("failed to run the agent server: %v", err)
	}

	stopCh := setupSignalHandler()
	<-stopCh
	klog.V(1).Infoln("Shutting down server.")

	if masterStop != nil {
		masterStop()
	}
}

func (c *cloudStream) Enable() bool {
	return c.enable
}

type StopFunc func()

func (c *cloudStream) runMasterServer(ctx context.Context, s *anpserver.ProxyServer) (StopFunc, error) {
	var stop StopFunc

	grpcServer := grpc.NewServer()
	client.RegisterProxyServiceServer(grpcServer, s)
	lis, err := getUDSListener(ctx, udsName)
	if err != nil {
		return nil, fmt.Errorf("failed to get uds listener: %v", err)
	}
	go grpcServer.Serve(lis)
	stop = grpcServer.GracefulStop

	return stop, nil
}

func getUDSListener(ctx context.Context, udsName string) (net.Listener, error) {
	udsListenerLock.Lock()
	defer udsListenerLock.Unlock()
	oldUmask := syscall.Umask(0007)
	defer syscall.Umask(oldUmask)
	var lc net.ListenConfig
	lis, err := lc.Listen(ctx, "unix", udsName)
	if err != nil {
		return nil, fmt.Errorf("failed to listen(unix) name %s: %v", udsName, err)
	}
	return lis, nil
}

func (c *cloudStream) runAgentServer(server *anpserver.ProxyServer) error {
	var tlsConfig *tls.Config
	var err error
	if tlsConfig, err = getTLSConfig(config.Config.TLSTunnelCAFile, config.Config.TLSTunnelCertFile, config.Config.TLSTunnelPrivateKeyFile); err != nil {
		return err
	}

	addr := fmt.Sprintf(":%d", 18132)
	serverOptions := []grpc.ServerOption{
		grpc.Creds(credentials.NewTLS(tlsConfig)),
		grpc.KeepaliveParams(keepalive.ServerParameters{Time: 30 * time.Second}),
	}
	grpcServer := grpc.NewServer(serverOptions...)
	agent.RegisterAgentServiceServer(grpcServer, server)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", addr, err)
	}
	go grpcServer.Serve(lis)

	return nil
}

var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

func setupSignalHandler() (stopCh <-chan struct{}) {
	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		close(stop)
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return stop
}

func getTLSConfig(caFile, certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load X509 key pair %s and %s: %v", certFile, keyFile, err)
	}

	if caFile == "" {
		return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
	}

	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read cluster CA cert %s: %v", caFile, err)
	}

	certPool := x509.NewCertPool()

	ok := certPool.AppendCertsFromPEM(caCert)
	if !ok {
		return nil, fmt.Errorf("failed to append cluster CA cert to the cert pool")
	}
	tlsConfig := &tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{cert},
		ClientCAs:    certPool,
	}

	return tlsConfig, nil
}
