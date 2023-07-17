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
 * Copyright 2017 Red Hat, Inc.
 *
 */

package libcert

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	cryptorand "crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"net"
	"time"

	"kubevirt.io/kubevirt/tests/framework/kubevirt"
	"kubevirt.io/kubevirt/tests/libkubevirt"
	"kubevirt.io/kubevirt/tests/libkubevirt/config"

	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	cmclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"

	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"

	"kubevirt.io/kubevirt/tests/flags"
)

func SetCertificateRotationStrategySelfSigned(virtClient kubecli.KubevirtClient) {
	kv := libkubevirt.GetCurrentKv(virtClient)
	spec := kv.Spec
	spec.CertificateRotationStrategy = v1.KubeVirtCertificateRotateStrategy{SelfSigned: &v1.KubeVirtSelfSignConfiguration{
		CA: &v1.CertConfig{
			Duration:    &metav1.Duration{Duration: 20 * time.Minute},
			RenewBefore: &metav1.Duration{Duration: 12 * time.Minute},
		},
		Server: &v1.CertConfig{
			Duration:    &metav1.Duration{Duration: 14 * time.Minute},
			RenewBefore: &metav1.Duration{Duration: 10 * time.Minute},
		},
	}}
	config.UpdateKubeVirtConfigValueAndWait(spec.Configuration)
}

func SetCertificateRotationStrategyCertManager(virtClient kubecli.KubevirtClient, issuerName string) {
	kv := libkubevirt.GetCurrentKv(virtClient)
	spec := kv.Spec
	apigroup := "cert-manager.io"
	spec.CertificateRotationStrategy = v1.KubeVirtCertificateRotateStrategy{}
	spec.CertificateRotationStrategy.SelfSigned = nil
	spec.CertificateRotationStrategy.CertManager = &v1.CertManagerConfiguration{
		IssuerRef: &k8sv1.TypedLocalObjectReference{
			Name:     "my-ca-issuer",
			Kind:     "Issuer",
			APIGroup: &apigroup,
		},
		CaBundleConfigMapRef: &k8sv1.TypedLocalObjectReference{
			Name: "cabundle",
			Kind: "ConfigMap",
		},
		Duration: &metav1.Duration{Duration: 14 * time.Minute},
	}
	config.UpdateKubeVirtConfigValueAndWait(spec.Configuration)
}

func EnsureIssuerIsPresent(namespace string, issuer cmapi.Issuer) (err error) {
	virtClient := kubevirt.Client()
	clientSet, err := cmclient.NewForConfig(virtClient.Config())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = clientSet.CertmanagerV1().Issuers(namespace).Get(ctx, issuer.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = clientSet.CertmanagerV1().Issuers(namespace).Create(ctx, &issuer, metav1.CreateOptions{})
	}
	return err
}

func EnsureRootSecretIsPresent(namespace string, secret *k8sv1.Secret) (err error) {
	virtClient := kubevirt.Client()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = virtClient.CoreV1().Secrets(namespace).Get(ctx, secret.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = virtClient.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	}
	return err
}

func EnsureCabundleIsPresent(namespace string, configMap *k8sv1.ConfigMap) (err error) {
	virtClient := kubevirt.Client()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = virtClient.CoreV1().ConfigMaps(namespace).Get(ctx, configMap.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = virtClient.CoreV1().ConfigMaps(namespace).Create(ctx, configMap, metav1.CreateOptions{})
	}
	return err
}

func CreateCertificateRequest(namespace string, issuerName string) (*cmapi.CertificateRequest, error) {
	cmClient, err := cmclient.NewForConfig(kubevirt.Client().Config())
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	csrTemplate := &x509.CertificateRequest{
		Subject: pkix.Name{
			Organization: []string{"kubevirt.io:system"},
			CommonName:   "kubevirt.io:system:server:dummy",
		},
		DNSNames:    []string{},
		IPAddresses: []net.IP{},
	}
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), cryptorand.Reader)
	if err != nil {
		panic(err)
	}
	csrDER, err := x509.CreateCertificateRequest(cryptorand.Reader, csrTemplate, privateKey)
	if err != nil {
		panic(err)
	}

	csrPemBlock := &pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrDER,
	}
	cr := cmapi.CertificateRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "dummy-cr-",
			Namespace:    flags.KubeVirtInstallNamespace,
		},
		Spec: cmapi.CertificateRequestSpec{
			Request: pem.EncodeToMemory(csrPemBlock),
			IsCA:    false,
			Usages: []cmapi.KeyUsage{
				cmapi.UsageDigitalSignature,
				cmapi.UsageKeyEncipherment,
				cmapi.UsageServerAuth,
			},
			Duration: &metav1.Duration{Duration: time.Hour * 2160},
			IssuerRef: cmmeta.ObjectReference{
				Name:  issuerName,
				Kind:  "Issuer",
				Group: "cert-manager.io",
			},
		},
	}
	req, err := cmClient.CertmanagerV1().CertificateRequests(namespace).Create(ctx, &cr, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return req, nil
}

func GetCertificateRequestCondition(namespace string, name string) (condition cmapi.CertificateRequestConditionType, err error) {
	cmClient, err := cmclient.NewForConfig(kubevirt.Client().Config())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cr, err := cmClient.CertmanagerV1().CertificateRequests(namespace).Get(ctx, name, metav1.GetOptions{})
	for _, c := range cr.Status.Conditions {
		if c.Type == cmapi.CertificateRequestConditionDenied {
			return cmapi.CertificateRequestConditionDenied, nil
		}
		if c.Type == cmapi.CertificateRequestConditionInvalidRequest {
			return cmapi.CertificateRequestConditionInvalidRequest, nil
		}
		if c.Type == cmapi.CertificateRequestConditionApproved {
			return cmapi.CertificateRequestConditionApproved, nil
		}
	}
	return "", nil
}

func CheckCertificateRequstApproved(namespace string, name string) (approved bool, err error) {
	condition, err := GetCertificateRequestCondition(namespace, name)
	if err != nil {
		return false, err
	}
	return condition == cmapi.CertificateRequestConditionApproved, nil
}
