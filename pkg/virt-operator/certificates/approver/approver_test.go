package approver

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/rand"
	"net"
	"testing"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
)

func TestHasKubeletUsages(t *testing.T) {
	cases := []struct {
		usages   []cmapi.KeyUsage
		expected bool
	}{
		{
			usages:   nil,
			expected: false,
		},
		{
			usages:   []cmapi.KeyUsage{},
			expected: false,
		},
		{
			usages: []cmapi.KeyUsage{
				cmapi.UsageKeyEncipherment,
				cmapi.UsageDigitalSignature,
			},
			expected: false,
		},
		{
			usages: []cmapi.KeyUsage{
				cmapi.UsageKeyEncipherment,
				cmapi.UsageDigitalSignature,
				cmapi.UsageServerAuth,
			},
			expected: false,
		},
		{
			usages: []cmapi.KeyUsage{
				cmapi.UsageKeyEncipherment,
				cmapi.UsageDigitalSignature,
				cmapi.UsageClientAuth,
			},
			expected: true,
		},
	}
	for _, c := range cases {
		if hasExactUsages(&cmapi.CertificateRequest{
			Spec: cmapi.CertificateRequestSpec{
				Usages: c.usages,
			},
		}, clientUsages) != c.expected {
			t.Errorf("unexpected result of hasKubeletUsages(%v), expecting: %v", c.usages, c.expected)
		}
	}
}

func TestVirtHandlerRecognizers(t *testing.T) {
	goodCases := []func(b *crBuilder){
		func(b *crBuilder) {
		},
	}

	testVirtHandlerClientRecognizer(t, goodCases, isKubeVirtCert, true)

	badCases := []func(b *crBuilder){
		func(b *crBuilder) {
			b.cn = "mike"
		},
		func(b *crBuilder) {
			b.orgs = nil
		},
		func(b *crBuilder) {
			b.emails = []string{"something"}
		},
		func(b *crBuilder) {
			b.orgs = []string{"system:master"}
		},
		func(b *crBuilder) {
			b.usages = append(b.usages, cmapi.UsageServerAuth)
		},
		func(b *crBuilder) {
			b.ips = nil
		},
		func(b *crBuilder) {
			b.ips = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.1")}
		},
		func(b *crBuilder) {
			b.requestor = "joe"
		},
		func(b *crBuilder) {
			b.dns = []string{"something"}
		},
	}

	testVirtHandlerClientRecognizer(t, badCases, isKubeVirtCert, false)
}

func TestAPIServerClientRecognizer(t *testing.T) {
	goodCases := []func(b *crBuilder){
		func(b *crBuilder) {
		},
	}

	testAPIServerClientRecognizer(t, goodCases, isKubeVirtCert, true)

	badCases := []func(b *crBuilder){
		func(b *crBuilder) {
			b.cn = "mike"
		},
		func(b *crBuilder) {
			b.orgs = nil
		},
		func(b *crBuilder) {
			b.emails = []string{"something"}
		},
		func(b *crBuilder) {
			b.orgs = []string{"system:master"}
		},
		func(b *crBuilder) {
			b.usages = append(b.usages, cmapi.UsageServerAuth)
		},
		func(b *crBuilder) {
			b.ips = nil
		},
		func(b *crBuilder) {
			b.ips = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.1")}
		},
		func(b *crBuilder) {
			b.requestor = "joe"
		},
		func(b *crBuilder) {
			b.dns = []string{}
		},
	}

	testAPIServerClientRecognizer(t, badCases, isKubeVirtCert, false)
}

func TestServiceRecognizers(t *testing.T) {
	goodCases := []func(b *crBuilder){
		func(b *crBuilder) {
		},
	}

	testControllerRecognizer(t, goodCases, isKubeVirtCert, true)
	testVirtHandlerServerRecognizer(t, goodCases, isKubeVirtCert, true)

	badCases := []func(b *crBuilder){
		func(b *crBuilder) {
			b.cn = "mike"
		},
		func(b *crBuilder) {
			b.emails = []string{"something"}
		},
		func(b *crBuilder) {
			b.orgs = nil
		},
		func(b *crBuilder) {
			b.orgs = []string{"system:master"}
		},
		func(b *crBuilder) {
			b.usages = append(b.usages, cmapi.UsageClientAuth)
		},
		func(b *crBuilder) {
			b.usages = []cmapi.KeyUsage{cmapi.UsageClientAuth}
		},
		func(b *crBuilder) {
			b.ips = nil
		},
		func(b *crBuilder) {
			b.ips = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.1")}
		},
		func(b *crBuilder) {
			b.requestor = "joe"
		},
		func(b *crBuilder) {
			b.dns = []string{"something"}
		},
	}

	testControllerRecognizer(t, badCases, isKubeVirtCert, false)
	testVirtHandlerServerRecognizer(t, badCases, isKubeVirtCert, false)
}

func TestOperatorRecognizer(t *testing.T) {
	goodCases := []func(b *crBuilder){
		func(b *crBuilder) {
		},
	}

	testOperatorRecognizer(t, goodCases, isKubeVirtCert, true)

	badCases := []func(b *crBuilder){
		func(b *crBuilder) {
			b.cn = "mike"
		},
		func(b *crBuilder) {
			b.emails = []string{"something"}
		},
		func(b *crBuilder) {
			b.orgs = nil
		},
		func(b *crBuilder) {
			b.orgs = []string{"system:master"}
		},
		func(b *crBuilder) {
			b.usages = append(b.usages, cmapi.UsageClientAuth)
		},
		func(b *crBuilder) {
			b.usages = []cmapi.KeyUsage{cmapi.UsageClientAuth}
		},
		func(b *crBuilder) {
			b.ips = nil
		},
		func(b *crBuilder) {
			b.ips = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.1")}
		},
		func(b *crBuilder) {
			b.requestor = "joe"
		},
		func(b *crBuilder) {
			b.dns = []string{}
		},
	}

	testOperatorRecognizer(t, badCases, isKubeVirtCert, false)
}

func TestAPIServerRecognizers(t *testing.T) {
	goodCases := []func(b *crBuilder){
		func(b *crBuilder) {
		},
	}

	testAPIServerRecognizer(t, goodCases, isKubeVirtCert, true)

	badCases := []func(b *crBuilder){
		func(b *crBuilder) {
			b.cn = "mike"
		},
		func(b *crBuilder) {
			b.emails = []string{"something"}
		},
		func(b *crBuilder) {
			b.orgs = nil
		},
		func(b *crBuilder) {
			b.orgs = []string{"system:master"}
		},
		func(b *crBuilder) {
			b.usages = append(b.usages, cmapi.UsageClientAuth)
		},
		func(b *crBuilder) {
			b.usages = []cmapi.KeyUsage{cmapi.UsageClientAuth}
		},
		func(b *crBuilder) {
			b.ips = nil
		},
		func(b *crBuilder) {
			b.ips = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.1")}
		},
		func(b *crBuilder) {
			b.requestor = "joe"
		},
		func(b *crBuilder) {
			b.dns = []string{"something", "else"}
		},
		func(b *crBuilder) {
			b.dns = nil
		},
	}

	testAPIServerRecognizer(t, badCases, isKubeVirtCert, false)
}

func testVirtHandlerServerRecognizer(t *testing.T, cases []func(b *crBuilder), recognizeFunc func(cr *cmapi.CertificateRequest, x509cr *x509.CertificateRequest) bool, shouldRecognize bool) {
	for _, c := range cases {
		b := crBuilder{
			cn:        "kubevirt.io:system:node:",
			orgs:      []string{"kubevirt.io:system"},
			requestor: "system:serviceaccount:kubevirt:kubevirt-handler",
			usages: []cmapi.KeyUsage{
				cmapi.UsageKeyEncipherment,
				cmapi.UsageDigitalSignature,
				cmapi.UsageServerAuth,
			},
			ips: []net.IP{net.ParseIP("127.0.0.1")},
		}
		c(&b)
		t.Run(fmt.Sprintf("cr:%#v", b), func(t *testing.T) {
			cr := makeFancyTestcr(b)
			x509cr, err := parseCR(cr.Spec.Request)
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			if recognizeFunc(cr, x509cr) != shouldRecognize {
				t.Errorf("expected recognized to be %v", shouldRecognize)
			}
		})
	}
}

func testVirtHandlerClientRecognizer(t *testing.T, cases []func(b *crBuilder), recognizeFunc func(cr *cmapi.CertificateRequest, x509cr *x509.CertificateRequest) bool, shouldRecognize bool) {
	for _, c := range cases {
		b := crBuilder{
			cn:        "kubevirt.io:system:client:",
			orgs:      []string{"kubevirt.io:system"},
			requestor: "system:serviceaccount:kubevirt:kubevirt-handler",
			usages: []cmapi.KeyUsage{
				cmapi.UsageKeyEncipherment,
				cmapi.UsageDigitalSignature,
				cmapi.UsageClientAuth,
			},
			ips: []net.IP{net.ParseIP("127.0.0.1")},
		}
		c(&b)
		t.Run(fmt.Sprintf("cr:%#v", b), func(t *testing.T) {
			cr := makeFancyTestcr(b)
			x509cr, err := parseCR(cr.Spec.Request)
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			if recognizeFunc(cr, x509cr) != shouldRecognize {
				t.Errorf("expected recognized to be %v", shouldRecognize)
			}
		})
	}
}

func testControllerRecognizer(t *testing.T, cases []func(b *crBuilder), recognizeFunc func(cr *cmapi.CertificateRequest, x509cr *x509.CertificateRequest) bool, shouldRecognize bool) {
	for _, c := range cases {
		b := crBuilder{
			cn:        "kubevirt.io:system:server:virt-controller",
			orgs:      []string{"kubevirt.io:system"},
			requestor: "system:serviceaccount:kubevirt:kubevirt-controller",
			usages: []cmapi.KeyUsage{
				cmapi.UsageKeyEncipherment,
				cmapi.UsageDigitalSignature,
				cmapi.UsageServerAuth,
			},
			ips: []net.IP{net.ParseIP("127.0.0.1")},
		}
		c(&b)
		t.Run(fmt.Sprintf("cr:%#v", b), func(t *testing.T) {
			cr := makeFancyTestcr(b)
			x509cr, err := parseCR(cr.Spec.Request)
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			if recognizeFunc(cr, x509cr) != shouldRecognize {
				t.Errorf("expected recognized to be %v", shouldRecognize)
			}
		})
	}
}

func testAPIServerRecognizer(t *testing.T, cases []func(b *crBuilder), recognizeFunc func(cr *cmapi.CertificateRequest, x509cr *x509.CertificateRequest) bool, shouldRecognize bool) {
	for _, c := range cases {
		b := crBuilder{
			cn:        "kubevirt.io:system:server:kubevirt-apiserver",
			orgs:      []string{"kubevirt.io:system"},
			requestor: "system:serviceaccount:kubevirt:kubevirt-apiserver",
			usages: []cmapi.KeyUsage{
				cmapi.UsageKeyEncipherment,
				cmapi.UsageDigitalSignature,
				cmapi.UsageServerAuth,
			},
			ips: []net.IP{net.ParseIP("127.0.0.1")},
			dns: []string{"virt-api.kubevirt.svc"},
		}
		c(&b)
		t.Run(fmt.Sprintf("cr:%#v", b), func(t *testing.T) {
			cr := makeFancyTestcr(b)
			x509cr, err := parseCR(cr.Spec.Request)
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			if recognizeFunc(cr, x509cr) != shouldRecognize {
				t.Errorf("expected recognized to be %v", shouldRecognize)
			}
		})
	}
}

func testAPIServerClientRecognizer(t *testing.T, cases []func(b *crBuilder), recognizeFunc func(cr *cmapi.CertificateRequest, x509cr *x509.CertificateRequest) bool, shouldRecognize bool) {
	for _, c := range cases {
		b := crBuilder{
			cn:        "kubevirt.io:system:client:kubevirt-apiserver",
			orgs:      []string{"kubevirt.io:system"},
			requestor: "system:serviceaccount:kubevirt:kubevirt-apiserver",
			usages: []cmapi.KeyUsage{
				cmapi.UsageKeyEncipherment,
				cmapi.UsageDigitalSignature,
				cmapi.UsageClientAuth,
			},
			ips: []net.IP{net.ParseIP("127.0.0.1")},
			dns: []string{"virt-api.kubevirt.svc"},
		}
		c(&b)
		t.Run(fmt.Sprintf("cr:%#v", b), func(t *testing.T) {
			cr := makeFancyTestcr(b)
			x509cr, err := parseCR(cr.Spec.Request)
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			if recognizeFunc(cr, x509cr) != shouldRecognize {
				t.Errorf("expected recognized to be %v", shouldRecognize)
			}
		})
	}
}

func testOperatorRecognizer(t *testing.T, cases []func(b *crBuilder), recognizeFunc func(cr *cmapi.CertificateRequest, x509cr *x509.CertificateRequest) bool, shouldRecognize bool) {
	for _, c := range cases {
		b := crBuilder{
			cn:        "kubevirt.io:system:server:kubevirt-operator-webhook",
			orgs:      []string{"kubevirt.io:system"},
			requestor: "system:serviceaccount:kubevirt:kubevirt-operator",
			usages: []cmapi.KeyUsage{
				cmapi.UsageKeyEncipherment,
				cmapi.UsageDigitalSignature,
				cmapi.UsageServerAuth,
			},
			ips: []net.IP{net.ParseIP("127.0.0.1")},
			dns: []string{"virt-operator.kubevirt.svc"},
		}
		c(&b)
		t.Run(fmt.Sprintf("cr:%#v", b), func(t *testing.T) {
			cr := makeFancyTestcr(b)
			x509cr, err := parseCR(cr.Spec.Request)
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			if recognizeFunc(cr, x509cr) != shouldRecognize {
				t.Errorf("expected recognized to be %v", shouldRecognize)
			}
		})
	}
}

// noncryptographic for faster testing
// DO NOT COPY THIS CODE
var insecureRand = rand.New(rand.NewSource(0))

func makeTestcr() *cmapi.CertificateRequest {
	return makeFancyTestcr(crBuilder{cn: "test-cert"})
}

type crBuilder struct {
	cn        string
	orgs      []string
	requestor string
	usages    []cmapi.KeyUsage
	dns       []string
	emails    []string
	ips       []net.IP
}

func makeFancyTestcr(b crBuilder) *cmapi.CertificateRequest {
	pk, err := ecdsa.GenerateKey(elliptic.P224(), insecureRand)
	if err != nil {
		panic(err)
	}
	crb, err := x509.CreateCertificateRequest(insecureRand, &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   b.cn,
			Organization: b.orgs,
		},
		DNSNames:       b.dns,
		EmailAddresses: b.emails,
		IPAddresses:    b.ips,
	}, pk)
	if err != nil {
		panic(err)
	}
	return &cmapi.CertificateRequest{
		Spec: cmapi.CertificateRequestSpec{
			Username: b.requestor,
			Usages:   b.usages,
			Request:  pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: crb}),
		},
	}
}
