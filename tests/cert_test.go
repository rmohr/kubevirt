package tests_test

import (
	"context"
	"encoding/pem"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/client-go/kubecli"

	"kubevirt.io/kubevirt/pkg/certificates/triple"
	"kubevirt.io/kubevirt/pkg/certificates/triple/cert"

	"kubevirt.io/kubevirt/tests/libcert"

	. "kubevirt.io/kubevirt/tests/framework/matcher"
	"kubevirt.io/kubevirt/tests/libinfra"
	"kubevirt.io/kubevirt/tests/libvmifact"

	"kubevirt.io/kubevirt/pkg/virt-operator/resource/generate/components"
	"kubevirt.io/kubevirt/tests/console"
	"kubevirt.io/kubevirt/tests/decorators"
	"kubevirt.io/kubevirt/tests/flags"
	"kubevirt.io/kubevirt/tests/framework/kubevirt"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	issuerName = "my-ca-issuer"
)

var _ = Describe("Infrastructure", Serial, decorators.SigCompute, func() {
	var (
		virtClient kubecli.KubevirtClient
		err        error
		rootSecret *v1.Secret
		cabundle   *v1.ConfigMap
		issuer     cmapi.Issuer
		caBundle   []byte
	)
	BeforeEach(func() {
		keyPair, err := triple.NewCA("testca", 24*time.Hour)
		tlsCrt := cert.EncodeCertPEM(keyPair.Cert)
		caBundle = tlsCrt
		tlsKey := cert.EncodePrivateKeyPEM(keyPair.Key)
		Expect(err).ToNot(HaveOccurred())
		virtClient = kubevirt.Client()

		rootSecret = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "root-secret",
			},
			Type: v1.SecretTypeTLS,
			Data: map[string][]byte{
				"ca.crt":  caBundle,
				"tls.crt": tlsCrt,
				"tls.key": tlsKey,
			},
		}
		Expect(libcert.EnsureRootSecretIsPresent(flags.KubeVirtInstallNamespace, rootSecret)).To(Succeed())

		cabundle = &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cabundle",
			},
			Data: map[string]string{
				"ca-bundle": string(tlsCrt),
			},
		}
		Expect(libcert.EnsureCabundleIsPresent(flags.KubeVirtInstallNamespace, cabundle)).To(Succeed())

		issuer = cmapi.Issuer{
			ObjectMeta: metav1.ObjectMeta{
				Name: issuerName,
			},
			Spec: cmapi.IssuerSpec{
				IssuerConfig: cmapi.IssuerConfig{
					CA: &cmapi.CAIssuer{
						SecretName: rootSecret.Name,
					},
				},
			},
		}
		Expect(libcert.EnsureIssuerIsPresent(flags.KubeVirtInstallNamespace, issuer)).To(Succeed())
	})

	Describe("certificates", Serial, func() {
		It(fmt.Sprintf("Switch between selfsigned and cert-manager 3 times"), Serial, func() {
			for i := 1; i <= 3; i++ {
				By("Setting self signed")
				libcert.SetCertificateRotationStrategySelfSigned(virtClient)

				By("checking that the CA secret gets created with a new ca bundle")
				var newCA []byte
				Eventually(func() []byte {
					newCA = libinfra.GetCertFromSecret(components.KubeVirtCASecretName)
					return newCA
				}, 10*time.Second, 1*time.Second).Should(Not(BeEmpty()))

				By("checking that the ca bundle gets propagated to the validating webhook")
				Eventually(func() bool {
					webhook, err := virtClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(context.Background(), components.VirtAPIValidatingWebhookName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					if len(webhook.Webhooks) > 0 {
						return libinfra.ContainsCrt(webhook.Webhooks[0].ClientConfig.CABundle, newCA)
					}
					return false
				}, 10*time.Second, 1*time.Second).Should(BeTrue())
				By("checking that the ca bundle gets propagated to the mutating webhook")
				Eventually(func() bool {
					webhook, err := virtClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(context.Background(), components.VirtAPIMutatingWebhookName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					if len(webhook.Webhooks) > 0 {
						return libinfra.ContainsCrt(webhook.Webhooks[0].ClientConfig.CABundle, newCA)
					}
					return false
				}, 10*time.Second, 1*time.Second).Should(BeTrue())

				By("checking that we can login to the vm via console")
				vmi := libvmifact.NewCirros()
				Eventually(ThisVMI(vmi), 60).Should(BeRunning())
				Eventually(func() (rotated bool) {
					Expect(console.LoginToCirros(vmi)).To(Succeed())
					return true
				}, 120*time.Second).Should(BeTrue())
				err = virtClient.VirtualMachineInstance(vmi.Namespace).Delete(context.Background(), vmi.Name, metav1.DeleteOptions{})
				Expect(err).ToNot(HaveOccurred())

				By("Setting cert-manager")
				libcert.SetCertificateRotationStrategyCertManager(virtClient, issuer.Name)

				caPEM, _ := pem.Decode(caBundle)
				newCA = pem.EncodeToMemory(caPEM)
				By("checking that the ca bundle gets propagated to the validating webhook")
				Eventually(func() bool {
					webhook, err := virtClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(context.Background(), components.VirtAPIValidatingWebhookName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					if len(webhook.Webhooks) > 0 {
						return libinfra.ContainsCrt(webhook.Webhooks[0].ClientConfig.CABundle, newCA)
					}
					return false
				}, 10*time.Second, 1*time.Second).Should(BeTrue())
				By("checking that the ca bundle gets propagated to the mutating webhook")
				Eventually(func() bool {
					webhook, err := virtClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(context.Background(), components.VirtAPIMutatingWebhookName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					if len(webhook.Webhooks) > 0 {
						return libinfra.ContainsCrt(webhook.Webhooks[0].ClientConfig.CABundle, newCA)
					}
					return false
				}, 10*time.Second, 1*time.Second).Should(BeTrue())

				By("checking that we can login to the vm via console")
				vmi = libvmifact.NewCirros()
				Eventually(ThisVMI(vmi), 60).Should(BeRunning())
				Eventually(func() bool {
					Expect(console.LoginToCirros(vmi)).To(Succeed())
					return true
				}, 120*time.Second).Should(BeTrue())
				err = virtClient.VirtualMachineInstance(vmi.Namespace).Delete(context.Background(), vmi.Name, metav1.DeleteOptions{})
				Expect(err).ToNot(HaveOccurred())
			}
		})
		It(fmt.Sprintf("Test certificateRequest approval"), func() {
			By("creating random certificaterequest")
			cr, err := libcert.CreateCertificateRequest(flags.KubeVirtInstallNamespace, issuerName)
			Expect(err).ToNot(HaveOccurred())
			Consistently(func() bool {
				approved, err := libcert.CheckCertificateRequstApproved(flags.KubeVirtInstallNamespace, cr.Name)
				Expect(err).ToNot(HaveOccurred())
				return approved
			}, 10*time.Second).Should(BeFalse())
		})
	})
})
