// Package approver implements an automated approver for kubelet certificates.
package approver

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"reflect"
	"strings"

	authorization "k8s.io/api/authorization/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	cmclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"

	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/client-go/log"

	"kubevirt.io/kubevirt/pkg/virt-operator/certificates"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type crRecognizer struct {
	recognize      func(csr *cmapi.CertificateRequest, x509cr *x509.CertificateRequest) bool
	permission     authorization.ResourceAttributes
	successMessage string
}

type approver struct {
	ctx         context.Context
	cmClient    *cmclient.Clientset
	client      clientset.Interface
	namespace   string
	recognizers []crRecognizer
}

func NewCRApprovingController(ctx context.Context, client clientset.Interface, namespace string, crInformer cache.SharedIndexInformer) *certificates.CertificateController {

	restConfig, err := kubecli.GetKubevirtClientConfig()
	if err != nil {
		log.DefaultLogger().Reason(err).Errorf("failed to get kubevirt client")
	}

	cmClient, err := cmclient.NewForConfig(restConfig)
	if err != nil {
		log.DefaultLogger().Reason(err).Errorf("failed to create cmClient")
	}

	approver := &approver{
		ctx:         ctx,
		client:      client,
		cmClient:    cmClient,
		namespace:   namespace,
		recognizers: recognizers(),
	}
	return certificates.NewCertificateController(
		ctx,
		client,
		cmClient,
		namespace,
		crInformer,
		approver.handle,
	)
}

func recognizers() []crRecognizer {
	recognizers := []crRecognizer{
		{
			recognize:      isKubeVirtCert,
			permission:     authorization.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "kubevirt"},
			successMessage: "Auto approving kubevirt.io client certificate after SubjectAccessReview.",
		},
	}
	return recognizers
}

func (a *approver) handle(cr *cmapi.CertificateRequest) error {
	if len(cr.Status.Certificate) != 0 {
		return nil
	}
	if approved, denied := certificates.GetCertApprovalCondition(&cr.Status); approved || denied {
		log.Log.V(2).Infof("cr was previously processed")
		return nil
	}
	x509cr, err := parseCR(cr.Spec.Request)
	if err != nil {
		return fmt.Errorf("unable to parse csr %q: %v", cr.Name, err)
	}

	tried := []string{}

	for _, r := range a.recognizers {
		if !r.recognize(cr, x509cr) {
			log.Log.V(2).Infof("cr not recognized")
			continue
		}

		certificates.SetCertificateRequestCondition(cr, cmapi.CertificateRequestConditionApproved,
			cmmeta.ConditionTrue, "AutoApproved", r.successMessage)

		_, err = a.cmClient.CertmanagerV1().CertificateRequests(a.namespace).UpdateStatus(a.ctx, cr, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	}

	if len(tried) != 0 {
		return certificates.IgnorableError("recognized csr %q as %v but subject access review was not approved", cr.Name, tried)
	}

	return nil
}

func hasExactUsages(cr *cmapi.CertificateRequest, usages []cmapi.KeyUsage) bool {
	if len(usages) != len(cr.Spec.Usages) {
		return false
	}

	usageMap := map[cmapi.KeyUsage]struct{}{}
	for _, u := range usages {
		usageMap[u] = struct{}{}
	}

	for _, u := range cr.Spec.Usages {
		if _, ok := usageMap[u]; !ok {
			return false
		}
	}

	return true
}

var serverUsages = []cmapi.KeyUsage{
	cmapi.UsageKeyEncipherment,
	cmapi.UsageDigitalSignature,
	cmapi.UsageServerAuth,
}

var clientUsages = []cmapi.KeyUsage{
	cmapi.UsageKeyEncipherment,
	cmapi.UsageDigitalSignature,
	cmapi.UsageClientAuth,
}

func isKubeVirtCert(cr *cmapi.CertificateRequest, x509cr *x509.CertificateRequest) bool {
	// Only sign for the kubevirt.io:system org
	if !reflect.DeepEqual([]string{"kubevirt.io:system"}, x509cr.Subject.Organization) {
		return false
	}

	// No email needed
	if len(x509cr.EmailAddresses) > 0 {
		return false
	}

	// This is the PodIP, we want to have that on every cert
	if len(x509cr.IPAddresses) != 1 {
		return false
	}

	switch cr.Spec.Username {

	case "system:serviceaccount:kubevirt:kubevirt-handler":
		// virt-handler is server and client
		if strings.HasPrefix(x509cr.Subject.CommonName, "kubevirt.io:system:client") {
			if !hasExactUsages(cr, clientUsages) {
				return false
			}
		} else if strings.HasPrefix(x509cr.Subject.CommonName, "kubevirt.io:system:node") {
			if !hasExactUsages(cr, serverUsages) {
				return false
			}
		} else {
			return false
		}
		// virt-handler does not need any dns names, communication only happens via IPs
		// TODO: ensure that virt-handler node matches the IP of the virt-handler daemonset pod on that node.
		if len(x509cr.DNSNames) > 0 {
			return false
		}
	case "system:serviceaccount:kubevirt:kubevirt-operator":
		// virt-operator is only a server
		if !hasExactUsages(cr, serverUsages) {
			return false
		}
		// virt-operator should be reachable via the .svc dns name
		if len(x509cr.DNSNames) != 1 {
			return false
		}
		// virt-operator acts on the cluster level, not on the node level
		if x509cr.Subject.CommonName != "kubevirt.io:system:server:kubevirt-operator-webhook" {
			return false
		}
	case "system:serviceaccount:kubevirt:kubevirt-controller":
		// virt-controller is only a server
		if !hasExactUsages(cr, serverUsages) {
			return false
		}
		// virt-controller does not need any dns names, communication only happens via IPs
		if len(x509cr.DNSNames) > 0 {
			return false
		}
		// virt-controller acts on the cluster level, not on the node level
		if x509cr.Subject.CommonName != "kubevirt.io:system:server:virt-controller" {
			return false
		}
	case "system:serviceaccount:kubevirt:kubevirt-apiserver":
		// this virt-api name is for server
		if strings.HasPrefix(x509cr.Subject.CommonName, "kubevirt.io:system:client") {
			if !hasExactUsages(cr, clientUsages) {
				return false
			}
		} else if strings.HasPrefix(x509cr.Subject.CommonName, "kubevirt.io:system:server") {
			if !hasExactUsages(cr, serverUsages) {
				return false
			}
		} else {
			return false
		}
		// virt-api needs to be reachable via the virt-api service in the install namespace
		if len(x509cr.DNSNames) != 1 {
			return false
		}
	case "system:serviceaccount:kubevirt:kubevirt-exportproxy":
		// virt-exportproxy is only a server
		if !hasExactUsages(cr, serverUsages) {
			return false
		}
		// virt-exportproxy does not need any dns names, communication only happens via IPs
		if len(x509cr.DNSNames) > 0 {
			return false
		}
		// virt-exportproxy acts on the cluster level, not on the node level
		if x509cr.Subject.CommonName != "kubevirt.io:system:server:virt-exportproxy" {
			return false
		}
	default:
		return false
	}
	return true
}

func parseCR(pemData []byte) (*x509.CertificateRequest, error) {
	// extract PEM from request object
	block, _ := pem.Decode(pemData)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, fmt.Errorf("PEM block type must be CERTIFICATE REQUEST")
	}
	return x509.ParseCertificateRequest(block.Bytes)
}
