package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/kubelet/server"
	"k8s.io/kubernetes/pkg/kubelet/server/stats"

	"github.com/kubeedge/kubeedge/common/constants"
	"github.com/kubeedge/kubeedge/edge/pkg/edged/podmanager"
	hubconfig "github.com/kubeedge/kubeedge/edge/pkg/edgehub/config"
)

//constants to define server address
const (
	ServerAddr = "127.0.0.1"
)

//Server is object to define server
type Server struct {
	podManager podmanager.Manager
}

//NewServer creates and returns a new server object
func NewServer(podManager podmanager.Manager) *Server {
	return &Server{
		podManager: podManager,
	}
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
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{cert},
		ClientCAs:    certPool,
	}

	return tlsConfig, nil
}

// ListenAndServe starts a HTTP server and sets up a listener on the given host/port
func (s *Server) ListenAndServe(host server.HostInterface, resourceAnalyzer stats.ResourceAnalyzer, enableCAdvisorJSONEndpoints bool) {
	klog.Infof("starting to listen read-only on %s:%v", ServerAddr, constants.ServerPort)
	handler := server.NewServer(host, resourceAnalyzer, nil, enableCAdvisorJSONEndpoints, true, false, false, false, nil)
	var err error
	var tlsConfig *tls.Config
	//if tlsConfig, err = getTLSConfig(hubconfig.Config.TLSCAFile, hubconfig.Config.TLSCertFile, hubconfig.Config.TLSPrivateKeyFile); err != nil {
	if tlsConfig, err = getTLSConfig("", hubconfig.Config.TLSCertFile, hubconfig.Config.TLSPrivateKeyFile); err != nil {
		klog.Fatal(err)
	}
	hubServer := &http.Server{
		Addr:           net.JoinHostPort(ServerAddr, fmt.Sprintf("%d", constants.ServerPort)),
		Handler:        &handler,
		MaxHeaderBytes: 1 << 20,
		TLSConfig:      tlsConfig,
	}
	klog.Fatal(hubServer.ListenAndServeTLS(hubconfig.Config.TLSCertFile, hubconfig.Config.TLSPrivateKeyFile))
}

