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
 * Copyright 2018 Red Hat, Inc.
 *
 */

package virt_operator

import (
	"io/ioutil"
	golog "log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	k8coresv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	clientrest "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	"kubevirt.io/kubevirt/pkg/certificates"
	"kubevirt.io/kubevirt/pkg/controller"
	"kubevirt.io/kubevirt/pkg/kubecli"
	"kubevirt.io/kubevirt/pkg/log"
	"kubevirt.io/kubevirt/pkg/service"
	kvutil "kubevirt.io/kubevirt/pkg/util"
	"kubevirt.io/kubevirt/pkg/virt-operator/util"
)

const (
	controllerThreads = 3

	// Default port that virt-operator listens on.
	defaultPort = 8186

	// Default address that virt-operator listens on.
	defaultHost = "0.0.0.0"
)

type VirtOperatorApp struct {
	service.ServiceListen

	clientSet       kubecli.KubevirtClient
	restClient      *clientrest.RESTClient
	informerFactory controller.KubeInformerFactory

	kubeVirtController KubeVirtController
	kubeVirtRecorder   record.EventRecorder

	operatorNamespace string

	kubeVirtInformer cache.SharedIndexInformer
	kubeVirtCache    cache.Store

	serviceAccountInformer cache.SharedIndexInformer
	serviceAccountCache    cache.Store

	clusterRoleInformer cache.SharedIndexInformer
	clusterRoleCache    cache.Store

	clusterRoleBindingInformer cache.SharedIndexInformer
	clusterRoleBindingCache    cache.Store

	roleInformer cache.SharedIndexInformer
	roleCache    cache.Store

	roleBindingInformer cache.SharedIndexInformer
	roleBindingCache    cache.Store

	crdInformer cache.SharedIndexInformer
	crdCache    cache.Store

	serviceInformer cache.SharedIndexInformer
	serviceCache    cache.Store

	deploymentInformer cache.SharedIndexInformer
	deploymentCache    cache.Store

	daemonSetInformer cache.SharedIndexInformer
	daemonSetCache    cache.Store
}

var _ service.Service = &VirtOperatorApp{}

func Execute() {
	var err error
	app := VirtOperatorApp{}

	service.Setup(&app)

	log.InitializeLogging("virt-operator")

	app.clientSet, err = kubecli.GetKubevirtClient()

	if err != nil {
		golog.Fatal(err)
	}

	app.restClient = app.clientSet.RestClient()

	app.operatorNamespace, err = kvutil.GetNamespace()
	if err != nil {
		golog.Fatalf("Error searching for namespace: %v", err)
	}
	app.informerFactory = controller.NewKubeInformerFactory(app.restClient, app.clientSet, app.operatorNamespace)

	app.kubeVirtInformer = app.informerFactory.KubeVirt()
	app.kubeVirtCache = app.kubeVirtInformer.GetStore()

	app.serviceAccountInformer = app.informerFactory.OperatorServiceAccount()
	app.serviceAccountCache = app.serviceAccountInformer.GetStore()

	app.clusterRoleInformer = app.informerFactory.OperatorClusterRole()
	app.clusterRoleCache = app.clusterRoleInformer.GetStore()

	app.clusterRoleBindingInformer = app.informerFactory.OperatorClusterRoleBinding()
	app.clusterRoleBindingCache = app.clusterRoleBindingInformer.GetStore()

	app.roleInformer = app.informerFactory.OperatorRole()
	app.roleCache = app.roleInformer.GetStore()

	app.roleBindingInformer = app.informerFactory.OperatorRoleBinding()
	app.roleBindingCache = app.roleBindingInformer.GetStore()

	app.crdInformer = app.informerFactory.OperatorCRD()
	app.crdCache = app.crdInformer.GetStore()

	app.serviceInformer = app.informerFactory.OperatorService()
	app.serviceCache = app.serviceInformer.GetStore()

	app.deploymentInformer = app.informerFactory.OperatorDeployment()
	app.deploymentCache = app.deploymentInformer.GetStore()

	app.daemonSetInformer = app.informerFactory.OperatorDaemonSet()
	app.daemonSetCache = app.daemonSetInformer.GetStore()

	app.kubeVirtRecorder = app.getNewRecorder(k8sv1.NamespaceAll, "virt-operator")
	app.kubeVirtController = *NewKubeVirtController(app.clientSet, app.kubeVirtInformer, app.kubeVirtRecorder)

	image := os.Getenv(util.OperatorImageEnvName)
	if image == "" {
		golog.Fatalf("Error getting operator's image: %v", err)
	}
	log.Log.Infof("Operator image: %s", image)

	app.Run()
}

func (app *VirtOperatorApp) Run() {

	// prepare certs
	certsDirectory, err := ioutil.TempDir("", "certsdir")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(certsDirectory)

	certStore, err := certificates.GenerateSelfSignedCert(certsDirectory, "virt-operator", app.operatorNamespace)
	if err != nil {
		log.Log.Reason(err).Error("unable to generate certificates")
		panic(err)
	}

	// run app
	stop := make(chan struct{})
	defer close(stop)

	app.informerFactory.Start(stop)
	go app.kubeVirtController.Run(controllerThreads, stop)

	// serve metrics
	http.Handle("/metrics", promhttp.Handler())
	err = http.ListenAndServeTLS(app.ServiceListen.Address(), certStore.CurrentPath(), certStore.CurrentPath(), nil)
	if err != nil {
		log.Log.Reason(err).Error("Serving prometheus failed.")
		panic(err)
	}

}

func (app *VirtOperatorApp) getNewRecorder(namespace string, componentName string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&k8coresv1.EventSinkImpl{Interface: app.clientSet.CoreV1().Events(namespace)})
	return eventBroadcaster.NewRecorder(scheme.Scheme, k8sv1.EventSource{Component: componentName})
}

func (app *VirtOperatorApp) AddFlags() {
	app.InitFlags()

	app.BindAddress = defaultHost
	app.Port = defaultPort

	app.AddCommonFlags()
}
