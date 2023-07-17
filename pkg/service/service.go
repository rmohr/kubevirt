/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright The KubeVirt Authors.
 *
 */

package service

import (
	goflag "flag"
	"fmt"
	"net"
	"os"
	"strconv"

	flag "github.com/spf13/pflag"
	"k8s.io/client-go/util/certificate"

	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/client-go/log"

	"kubevirt.io/kubevirt/pkg/certificates/bootstrap"
	virtconfig "kubevirt.io/kubevirt/pkg/virt-config"
)

const (
	certificateDir = "/tmp/"
)

func init() {
	kubecli.Init()
}

type Service interface {
	Run()
	AddFlags()
}

type ServiceListen struct {
	Name               string
	Namespace          string
	BindAddress        string
	Port               int
	CertDir            string
	CertificateManager certificate.Manager
	PodIpAddress       string
	PodName            string
	ClusterConfig      *virtconfig.ClusterConfig
}

type ServiceLibvirt struct {
	LibvirtUri string
}

type CertificateConfigCallback func(certStore certificate.Store, name string, component string, dnsSANs []string, ipSANs []net.IP, namespace string, clusterConfig *virtconfig.ClusterConfig) *bootstrap.CertificateRequestCertificateManagerConfig

func (service *ServiceListen) Address() string {
	return fmt.Sprintf("%s:%s", service.BindAddress, strconv.Itoa(service.Port))
}

func (service *ServiceListen) InitFlags() {
	flag.CommandLine.AddGoFlag(goflag.CommandLine.Lookup("v"))
	flag.CommandLine.AddGoFlag(goflag.CommandLine.Lookup("kubeconfig"))
	flag.CommandLine.AddGoFlag(goflag.CommandLine.Lookup("master"))
}

func (service *ServiceListen) AddCommonFlags() {
	flag.StringVar(&service.BindAddress, "listen", service.BindAddress, "Address where to listen on")
	flag.IntVar(&service.Port, "port", service.Port, "Port to listen on")
	flag.StringVar(&service.CertDir, "cert-dir", certificateDir, "Certificate store directory")
	flag.StringVar(&service.PodIpAddress, "pod-ip-address", "", "The pod ip address")
	flag.StringVar(&service.PodName, "pod-name", "", "The pod name")
}

func (service *ServiceLibvirt) AddLibvirtFlags() {
	flag.StringVar(&service.LibvirtUri, "libvirt-uri", service.LibvirtUri, "Libvirt connection string")

}

func (service *ServiceListen) SetupCertificateManager(clusterConfig *virtconfig.ClusterConfig, certificateConfigFunc CertificateConfigCallback) certificate.Manager {
	var err error
	certificateManager, err := SetupCertificateManager(service.Name, service.CertDir, service.PodName, net.ParseIP(service.PodIpAddress), service.Namespace, clusterConfig, certificateConfigFunc)
	if err != nil {
		log.Log.Criticalf("Failed to setup certificate manager: %v", err)
		os.Exit(2)
	}
	return certificateManager
}

func SetupCertificateManager(component string, certDir string, podName string, podIP net.IP, namespace string, clusterConfig *virtconfig.ClusterConfig, certificateConfigFunc CertificateConfigCallback) (manager certificate.Manager, err error) {
	err = os.MkdirAll(certDir, 0700)
	if err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("failed to create certificate directory: %v", err)
	}
	store, err := certificate.NewFileStore("kubevirt-client", certDir, certDir, "", "")
	if err != nil {
		return nil, fmt.Errorf("unable to initialize certificate store: %v", err)
	}
	config := certificateConfigFunc(store, podName, component, []string{}, []net.IP{podIP}, namespace, clusterConfig)
	manager, err = bootstrap.NewCertificateRequestCertificateManager(config)
	if err != nil {
		return nil, fmt.Errorf("failed to setup the certificate manager: %v", err)
	}
	return manager, nil
}

func Setup(service Service) {
	service.AddFlags()

	flag.Parse()
}
