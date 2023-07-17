package bootstrap

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/certificate"
	"k8s.io/client-go/util/keyutil"

	"kubevirt.io/kubevirt/pkg/certificates/triple"
	virtconfig "kubevirt.io/kubevirt/pkg/virt-config"

	"kubevirt.io/client-go/log"

	"kubevirt.io/kubevirt/pkg/certificates/triple/cert"

	watchtools "k8s.io/client-go/tools/watch"
	"kubevirt.io/client-go/kubecli"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	cmclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

const (
	CertBytesValue = "tls.crt"
	KeyBytesValue  = "tls.key"
)

type FileCertificateManager struct {
	stopCh             chan struct{}
	certAccessLock     sync.Mutex
	stopped            bool
	cert               *tls.Certificate
	certBytesPath      string
	keyBytesPath       string
	errorRetryInterval time.Duration
}

// NewFallbackCertificateManager returns a certificate manager which can fall back to a self signed certificate,
// if there is currently no kubevirt installation present on the cluster. This helps dealing with situations where e.g.
// readiness probes try to access an API which can't right now provide a fully managed certificate.
// virt-operator is the main recipient of this manager, since the certificate management infrastructure is not always
// already present when virt-operator gets created.
func NewFallbackCertificateManager(clusterConfig *virtconfig.ClusterConfig, fileCertManager certificate.Manager, crCertManager certificate.Manager) *FallbackCertificateManager {
	caKeyPair, _ := triple.NewCA("kubevirt.io", time.Hour*24*7)
	keyPair, _ := triple.NewServerKeyPair(
		caKeyPair,
		"fallback.certificate.kubevirt.io",
		"fallback",
		"fallback",
		"cluster.local",
		nil,
		nil,
		time.Hour*24*356*10,
	)
	crt, err := tls.X509KeyPair(cert.EncodeCertPEM(keyPair.Cert), cert.EncodePrivateKeyPEM(keyPair.Key))
	if err != nil {
		log.DefaultLogger().Reason(err).Critical("Failed to generate a fallback certificate.")
	}
	crt.Leaf = keyPair.Cert

	return &FallbackCertificateManager{
		fileCertManager:     fileCertManager,
		crCertManager:       crCertManager,
		fallbackCertificate: &crt,
		clusterConfig:       clusterConfig,
	}
}

type FallbackCertificateManager struct {
	fileCertManager     certificate.Manager
	crCertManager       certificate.Manager
	CurrentCertManager  certificate.Manager
	fallbackCertificate *tls.Certificate
	clusterConfig       *virtconfig.ClusterConfig
}

func (f *FallbackCertificateManager) Start() {
	f.fileCertManager.Start()
	f.crCertManager.Start()
}

func (f *FallbackCertificateManager) Stop() {
	f.fileCertManager.Stop()
	f.crCertManager.Stop()
}

func (f *FallbackCertificateManager) Current() *tls.Certificate {
	if f.CurrentCertManager == nil {
		kv := f.clusterConfig.GetConfigFromKubeVirtCR()
		if kv == nil {
			return f.fallbackCertificate
		}
		if kv.Spec.CertificateRotationStrategy.CertManager != nil {
			log.DefaultLogger().Infof("using cert-manager")
			f.CurrentCertManager = f.crCertManager
		} else {
			log.DefaultLogger().Infof("using selfsigned")
			f.CurrentCertManager = f.fileCertManager
		}
	}
	crt := f.CurrentCertManager.Current()
	if crt != nil {
		log.DefaultLogger().Infof("cert found")
		return crt
	}
	log.DefaultLogger().Infof("cert not found")
	return f.fallbackCertificate
}

func (f *FallbackCertificateManager) ServerHealthy() bool {
	kv := f.clusterConfig.GetConfigFromKubeVirtCR()
	if kv.Spec.CertificateRotationStrategy.CertManager != nil {
		return f.crCertManager.ServerHealthy()
	} else {
		return f.fileCertManager.ServerHealthy()
	}
}

func NewFileCertificateManager(certBytesPath string, keyBytesPath string) *FileCertificateManager {
	return &FileCertificateManager{
		certBytesPath:      certBytesPath,
		keyBytesPath:       keyBytesPath,
		stopCh:             make(chan struct{}, 1),
		errorRetryInterval: 1 * time.Minute,
	}
}

func (f *FileCertificateManager) Start() {
	objectUpdated := make(chan struct{}, 1)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.DefaultLogger().Reason(err).Critical("Failed to create an inotify watcher")
	}
	defer watcher.Close()

	certDir := filepath.Dir(f.certBytesPath)
	err = watcher.Add(certDir)
	if err != nil {
		log.DefaultLogger().Reason(err).Criticalf("Failed to establish a watch on %s", f.certBytesPath)
	}
	keyDir := filepath.Dir(f.keyBytesPath)
	if keyDir != certDir {
		err = watcher.Add(keyDir)
		if err != nil {
			log.DefaultLogger().Reason(err).Criticalf("Failed to establish a watch on %s", f.keyBytesPath)
		}
	}

	go func() {
		for {
			select {
			case _, ok := <-watcher.Events:
				if !ok {
					return
				}
				select {
				case objectUpdated <- struct{}{}:
				default:
					log.DefaultLogger().V(5).Infof("Dropping redundant wakeup for cert reload")
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.DefaultLogger().Reason(err).Errorf("An error occurred when watching certificates files %s and %s", f.certBytesPath, f.keyBytesPath)
			}
		}
	}()

	// ensure we load the certificates on startup
	objectUpdated <- struct{}{}

sync:
	for {
		select {
		case <-objectUpdated:
			if err := f.rotateCerts(); err != nil {
				go func() {
					time.Sleep(f.errorRetryInterval)
					select {
					case objectUpdated <- struct{}{}:
					default:
						log.DefaultLogger().V(5).Infof("Dropping redundant wakeup for cert reload")
					}
				}()
			}
		case <-f.stopCh:
			break sync
		}
	}
}

func (f *FileCertificateManager) Stop() {
	f.certAccessLock.Lock()
	defer f.certAccessLock.Unlock()
	if f.stopped {
		return
	}
	close(f.stopCh)
	f.stopped = true
}

func (f *FileCertificateManager) ServerHealthy() bool {
	panic("implement me")
}

func (s *FileCertificateManager) Current() *tls.Certificate {
	s.certAccessLock.Lock()
	defer s.certAccessLock.Unlock()
	return s.cert
}

func (f *FileCertificateManager) rotateCerts() error {
	crt, err := f.loadCertificates()
	if err != nil {
		log.DefaultLogger().Reason(err).Errorf("failed to load the certificate %s and %s", f.certBytesPath, f.keyBytesPath)
		return err
	}

	f.certAccessLock.Lock()
	defer f.certAccessLock.Unlock()
	// update after the callback, to ensure that the reconfiguration succeeded
	f.cert = crt

	log.DefaultLogger().Infof("certificate with common name '%s' retrieved.", crt.Leaf.Subject.CommonName)
	return nil
}

func (f *FileCertificateManager) loadCertificates() (serverCrt *tls.Certificate, err error) {
	// #nosec No risk for path injection. Used for specific cert file for key rotation
	certBytes, err := os.ReadFile(f.certBytesPath)
	if err != nil {
		return nil, err
	}
	// #nosec No risk for path injection. Used for specific cert file for key rotation
	keyBytes, err := os.ReadFile(f.keyBytesPath)
	if err != nil {
		return nil, err
	}

	crt, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %v\n", err)
	}
	leaf, err := cert.ParseCertsPEM(certBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to load leaf certificate: %v\n", err)
	}
	crt.Leaf = leaf[0]
	return &crt, nil
}

type SecretCertificateManager struct {
	store     cache.Store
	secretKey string
	tlsCrt    string
	tlsKey    string
	crtLock   *sync.Mutex
	revision  string
	crt       *tls.Certificate
}

func (s *SecretCertificateManager) Start() {
}

func (s *SecretCertificateManager) Stop() {
}

func (s *SecretCertificateManager) Current() *tls.Certificate {
	s.crtLock.Lock()
	defer s.crtLock.Unlock()
	rawSecret, exists, err := s.store.GetByKey(s.secretKey)
	if err != nil {
		log.DefaultLogger().Reason(err).Errorf("Secret %s can't be retrieved from the cache", s.secretKey)
		return s.crt
	} else if !exists {
		return s.crt
	}
	secret := rawSecret.(*v1.Secret)
	if secret.ObjectMeta.ResourceVersion == s.revision {
		return s.crt
	}
	crt, err := tls.X509KeyPair(secret.Data[s.tlsCrt], secret.Data[s.tlsKey])
	if err != nil {
		log.DefaultLogger().Reason(err).Errorf("failed to load certificate from secret %s", s.secretKey)
		return s.crt
	}
	leaf, err := cert.ParseCertsPEM(secret.Data[s.tlsCrt])
	if err != nil {
		log.DefaultLogger().Reason(err).Errorf("failed to load leaf certificate from secret %s", s.secretKey)
		return s.crt
	}
	crt.Leaf = leaf[0]
	s.revision = secret.ResourceVersion
	s.crt = &crt
	return s.crt
}

func (s *SecretCertificateManager) ServerHealthy() bool {
	panic("implement me")
}

// NewSecretCertificateManager takes a secret store and the name and the  namespace of a secret. If there is a newer
// version of the secret in the cache, the next Current() call will immediately wield it. It takes resource versions
// into account to be efficient.
func NewSecretCertificateManager(name string, namespace string, store cache.Store) *SecretCertificateManager {
	return &SecretCertificateManager{
		store:     store,
		secretKey: fmt.Sprintf("%s/%s", namespace, name),
		tlsCrt:    CertBytesValue,
		tlsKey:    KeyBytesValue,
		crtLock:   &sync.Mutex{},
	}
}

type CertificateRequestCertificateManagerConfig struct {
	// Template is the CertificateRequest that will be used as a template for
	// generating certificate signing requests for all new keys generated as
	// part of rotation. It follows the same rules as the template parameter of
	// crypto.x509.CreateCertificateRequest in the Go standard libraries.
	Template *x509.CertificateRequest
	// GetTemplate returns the CertificateRequest that will be used as a template for
	// generating certificate signing requests for all new keys generated as
	// part of rotation. It follows the same rules as the template parameter of
	// crypto.x509.CreateCertificateRequest in the Go standard libraries.
	// If no template is available, nil may be returned, and no certificate will be requested.
	// If specified, takes precedence over Template.
	GetTemplate func() *x509.CertificateRequest
	// SignerName is the name of the certificate signer that should sign certificates
	// generated by the manager.
	SignerName string
	// RequestedCertificateLifetime is the requested lifetime length for certificates generated by the manager.
	// Optional.
	// This will set the spec.expirationSeconds field on the CSR.  Controlling the lifetime of
	// the issued certificate is not guaranteed as the signer may choose to ignore the request.
	RequestedCertificateLifetime *time.Duration
	// Usages is the types of usages that certificates generated by the manager
	// can be used for. It is mutually exclusive with GetUsages.
	Usages []cmapi.KeyUsage
	// GetUsages is dynamic way to get the types of usages that certificates generated by the manager
	// can be used for. If Usages is not nil, GetUsages has to be nil, vice versa.
	// It is mutually exclusive with Usages.
	GetUsages func(privateKey interface{}) []cmapi.KeyUsage
	// CertificateStore is a persistent store where the current cert/key is
	// kept and future cert/key pairs will be persisted after they are
	// generated.
	CertificateStore Store
	ClusterConfig    *virtconfig.ClusterConfig
	// Name is an optional string that will be used when writing log output
	// or returning errors from manager methods. If not set, SignerName will
	// be used, if SignerName is not set, if Usages includes client auth the
	// name will be "client auth", otherwise the value will be "server".
	Name      string
	Namespace string
}

// Store is responsible for getting and updating the current certificate.
// Depending on the concrete implementation, the backing store for this
// behavior may vary.
type Store interface {
	// Current returns the currently selected certificate, as well as the
	// associated certificate and key data in PEM format. If the Store doesn't
	// have a cert/key pair currently, it should return a NoCertKeyError so
	// that the Manager can recover by using bootstrap certificates to request
	// a new cert/key pair.
	Current() (*tls.Certificate, error)
	// Update accepts the PEM data for the cert/key pair and makes the new
	// cert/key pair the 'current' pair, that will be returned by future calls
	// to Current().
	Update(cert, key []byte) (*tls.Certificate, error)
}

type ClientsetFunc func(current *tls.Certificate) (clientset.Interface, error)

type CertificateRequestCertificateManager struct {
	getTemplate func() *x509.CertificateRequest

	lastRequestLock   sync.Mutex
	lastRequestCancel context.CancelFunc
	lastRequest       *x509.CertificateRequest

	signerName    string
	getUsages     func() []cmapi.KeyUsage
	forceRotation bool

	certStore Store

	// the following variables must only be accessed under certAccessLock
	certAccessLock sync.RWMutex
	cert           *tls.Certificate
	serverHealth   bool

	// the clientFn must only be accessed under the clientAccessLock
	clientAccessLock sync.Mutex
	clientsetFn      ClientsetFunc
	stopCh           chan struct{}
	stopped          bool
	clientSet        *cmclient.Clientset

	// Set to time.Now but can be stubbed out for testing
	now func() time.Time

	clusterConfig *virtconfig.ClusterConfig

	name      string
	namespace string
}

func NewCertificateRequestCertificateManager(config *CertificateRequestCertificateManagerConfig) (*CertificateRequestCertificateManager, error) {
	cert, forceRotation, err := getCurrentCertificateOrBootstrap(config.CertificateStore)
	if err != nil {
		return nil, err
	}

	template := config.Template
	log.DefaultLogger().V(2).Infof("got template: %+v", *template)

	getTemplate := func() *x509.CertificateRequest { return template }
	getUsages := func() []cmapi.KeyUsage {
		return config.Usages
	}

	restConfig, err := kubecli.GetKubevirtClientConfig()
	if err != nil {
		log.DefaultLogger().Reason(err).Errorf("failed to get kubevirt client")
	}

	clientSet, err := cmclient.NewForConfig(restConfig)
	if err != nil {
		log.DefaultLogger().Reason(err).Errorf("failed to create cmClient")
	}

	return &CertificateRequestCertificateManager{
		namespace:     config.Namespace,
		name:          config.Name,
		stopCh:        make(chan struct{}),
		getTemplate:   getTemplate,
		signerName:    config.SignerName,
		getUsages:     getUsages,
		cert:          cert,
		forceRotation: forceRotation,
		now:           time.Now,
		clientSet:     clientSet,
		certStore:     config.CertificateStore,
		clusterConfig: config.ClusterConfig,
	}, nil

}

func (c *CertificateRequestCertificateManager) Start() {
	log.DefaultLogger().V(2).Infof("Certificate rotation is enabled")

	templateChanged := make(chan struct{})
	go wait.Until(func() {
		deadline := c.nextRotationDeadline()
		log.DefaultLogger().V(2).Infof("deadline: %v", deadline)
		if sleepInterval := time.Until(deadline); sleepInterval > 0 {
			log.DefaultLogger().V(2).Infof("Waiting %v for next certificate rotation", sleepInterval)

			timer := time.NewTimer(sleepInterval)
			defer timer.Stop()

			select {
			case <-timer.C:
				// unblock when deadline expires
			case <-templateChanged:
				_, lastRequestTemplate := c.getLastRequest()
				if reflect.DeepEqual(lastRequestTemplate, c.getTemplate()) {
					// if the template now matches what we last requested, restart the rotation deadline loop
					return
				}
				log.DefaultLogger().V(2).Infof("Certificate template changed, rotating")
			}
		}

		backoff := wait.Backoff{
			Duration: 2 * time.Second,
			Factor:   2,
			Jitter:   0.1,
			Steps:    5,
		}
		if err := wait.ExponentialBackoff(backoff, c.rotateCerts); err != nil {
			utilruntime.HandleError(fmt.Errorf("reached backoff limit, still unable to rotate certs: %v", err))
			wait.PollInfinite(32*time.Second, c.rotateCerts)
		}
	}, time.Second, c.stopCh)

}

func (c *CertificateRequestCertificateManager) Current() *tls.Certificate {
	c.certAccessLock.RLock()
	defer c.certAccessLock.RUnlock()
	if c.cert != nil && c.cert.Leaf != nil && c.now().After(c.cert.Leaf.NotAfter) {
		log.DefaultLogger().V(2).Infof("%s: Current certificate is expired", c.name)
		return nil
	}
	return c.cert
}

func (c *CertificateRequestCertificateManager) ServerHealthy() bool {
	panic("implement me")
}

func (c *CertificateRequestCertificateManager) Stop() {
}

func getCurrentCertificateOrBootstrap(store Store) (cert *tls.Certificate, shouldRotate bool, errResult error) {
	currentCert, err := store.Current()
	if err == nil {
		// if the current cert is expired, fall back to the bootstrap cert
		if currentCert.Leaf != nil && time.Now().Before(currentCert.Leaf.NotAfter) {
			return currentCert, false, nil
		}
	} else {
		// log.DefaultLogger().V(2).Infof("cert store error is: %v", err)
		if _, ok := err.(*certificate.NoCertKeyError); !ok {
			return nil, false, err
		}
	}

	return nil, true, nil
}

func (c *CertificateRequestCertificateManager) nextRotationDeadline() time.Time {
	// forceRotation is not protected by locks
	if c.forceRotation {
		c.forceRotation = false
		return c.now()
	}

	c.certAccessLock.RLock()
	defer c.certAccessLock.RUnlock()

	if !c.certSatisfiesTemplateLocked() {
		return c.now()
	}

	notAfter := c.cert.Leaf.NotAfter
	totalDuration := float64(notAfter.Sub(c.cert.Leaf.NotBefore))
	deadline := c.cert.Leaf.NotBefore.Add(jitteryDuration(totalDuration))

	log.DefaultLogger().V(2).Infof("%s: Certificate expiration is %v, rotation deadline is %v", c.name, notAfter, deadline)
	return deadline
}

func (c *CertificateRequestCertificateManager) certSatisfiesTemplateLocked() bool {
	//todo
	return true
}

var jitteryDuration = func(totalDuration float64) time.Duration {
	return wait.Jitter(time.Duration(totalDuration), 0.2) - time.Duration(totalDuration*0.3)
}

func (c *CertificateRequestCertificateManager) getLastRequest() (context.CancelFunc, *x509.CertificateRequest) {
	c.lastRequestLock.Lock()
	defer c.lastRequestLock.Unlock()
	return c.lastRequestCancel, c.lastRequest
}

func (c *CertificateRequestCertificateManager) rotateCerts() (bool, error) {
	log.DefaultLogger().V(2).Infof("%s: Rotating certificates", c.name)

	_, csrPEM, keyPEM, _, err := c.generateCSR()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("%s: Unable to generate a certificate request: %v", c.name, err))
		return false, nil
	}

	usages := c.getUsages()

	kv := c.clusterConfig.GetConfigFromKubeVirtCR()
	if kv == nil {
		return false, nil
	}

	if kv.Spec.CertificateRotationStrategy.CertManager == nil {
		log.DefaultLogger().Infof("cert-manager isn't enabled")
		return false, nil
	}

	duration := &metav1.Duration{Duration: time.Hour * 2160}
	if kv.Spec.CertificateRotationStrategy.CertManager.Duration != nil {
		duration = kv.Spec.CertificateRotationStrategy.CertManager.Duration
	}

	cr := cmapi.CertificateRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: c.name + "-cr-",
			Namespace:    c.namespace,
		},
		Spec: cmapi.CertificateRequestSpec{
			Request:  csrPEM,
			IsCA:     false,
			Usages:   usages,
			Duration: duration,
			IssuerRef: cmmeta.ObjectReference{
				Name:  kv.Spec.CertificateRotationStrategy.CertManager.IssuerRef.Name,
				Kind:  kv.Spec.CertificateRotationStrategy.CertManager.IssuerRef.Kind,
				Group: *kv.Spec.CertificateRotationStrategy.CertManager.IssuerRef.APIGroup,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	req, err := c.clientSet.CertmanagerV1().CertificateRequests(c.namespace).Create(ctx, &cr, metav1.CreateOptions{})
	if err != nil {
		log.DefaultLogger().Reason(err).Errorf("failed to create certificaterequest")
	}

	fieldSelector := fields.OneTermEqualSelector("metadata.name", req.Name).String()
	obj := &cmapi.CertificateRequest{}
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return c.clientSet.CertmanagerV1().CertificateRequests(c.namespace).List(ctx, metav1.ListOptions{FieldSelector: fieldSelector})
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return c.clientSet.CertmanagerV1().CertificateRequests(c.namespace).Watch(ctx, metav1.ListOptions{FieldSelector: fieldSelector})
		},
	}

	var issuedCertificate []byte

	_, err = watchtools.UntilWithSync(
		ctx,
		lw,
		obj,
		nil,
		func(event watch.Event) (bool, error) {
			switch event.Type {
			case watch.Modified, watch.Added:
			case watch.Deleted:
				return false, fmt.Errorf("cr was deleted")
			default:
				return false, nil
			}

			switch cr := event.Object.(type) {
			case *cmapi.CertificateRequest:
				approved := false
				for _, c := range cr.Status.Conditions {
					if c.Type == cmapi.CertificateRequestConditionDenied {
						return false, fmt.Errorf("certificate request is denied, reason: %v, message: %v", c.Reason, c.Message)
					}
					if c.Type == cmapi.CertificateRequestConditionInvalidRequest {
						return false, fmt.Errorf("certificate request failed, reason: %v, message: %v", c.Reason, c.Message)
					}
					if c.Type == cmapi.CertificateRequestConditionApproved {
						approved = true
					}
				}
				if approved {
					if len(cr.Status.Certificate) > 0 {
						issuedCertificate = cr.Status.Certificate
						return true, nil
					}
				}
			default:
				return false, fmt.Errorf("unexpected type received: %T", event.Object)
			}

			return false, nil
		},
	)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("%s: certificate request was not signed: %v", c.name, err))
		return false, nil
	}

	log.DefaultLogger().Infof("retrieved new cr %v", issuedCertificate)
	cert, err := c.certStore.Update(issuedCertificate, keyPEM)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("%s: Unable to store the new cert/key pair: %v", c.name, err))
		return false, nil
	}

	c.updateCached(cert)

	return true, nil
}

func (c *CertificateRequestCertificateManager) updateCached(cert *tls.Certificate) *tls.Certificate {
	c.certAccessLock.Lock()
	defer c.certAccessLock.Unlock()
	c.serverHealth = true
	old := c.cert
	c.cert = cert
	return old
}

func (c *CertificateRequestCertificateManager) generateCSR() (template *x509.CertificateRequest, csrPEM []byte, keyPEM []byte, key interface{}, err error) {
	// Generate a new private key.
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%s: unable to generate a new private key: %v", c.name, err)
	}
	der, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%s: unable to marshal the new key to DER: %v", c.name, err)
	}

	keyPEM = pem.EncodeToMemory(&pem.Block{Type: keyutil.ECPrivateKeyBlockType, Bytes: der})

	template = c.getTemplate()
	if template == nil {
		return nil, nil, nil, nil, fmt.Errorf("%s: unable to create a csr, no template available", c.name)
	}
	csrPEM, err = makeCSRFromTemplate(privateKey, template)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%s: unable to create a csr from the private key: %v", c.name, err)
	}
	return template, csrPEM, keyPEM, privateKey, nil
}

func makeCSRFromTemplate(privateKey interface{}, template *x509.CertificateRequest) ([]byte, error) {
	t := *template
	t.SignatureAlgorithm = sigType(privateKey)

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &t, privateKey)
	if err != nil {
		return nil, err
	}

	csrPemBlock := &pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrDER,
	}

	return pem.EncodeToMemory(csrPemBlock), nil
}

func sigType(privateKey interface{}) x509.SignatureAlgorithm {
	// Customize the signature for RSA keys, depending on the key size
	if privateKey, ok := privateKey.(*rsa.PrivateKey); ok {
		keySize := privateKey.N.BitLen()
		switch {
		case keySize >= 4096:
			return x509.SHA512WithRSA
		case keySize >= 3072:
			return x509.SHA384WithRSA
		default:
			return x509.SHA256WithRSA
		}
	}
	return x509.UnknownSignatureAlgorithm
}

// LoadCertConfigForService is used by server components such as virt-operator and virt-controller.
// They will request certificates with the service name in the common name.
// For example the "kubevirt-operator-webhook" service will request a certificate with CN = kubevirt.io:system:server:kubevirt-operator-webhook
func LoadCertConfigForService(certStore certificate.Store, podName string, component string, dnsSANs []string, ipSANs []net.IP, namespace string, clusterConfig *virtconfig.ClusterConfig) *CertificateRequestCertificateManagerConfig {
	return &CertificateRequestCertificateManagerConfig{
		Template: &x509.CertificateRequest{
			Subject: pkix.Name{
				Organization: []string{"kubevirt.io:system"},
				CommonName:   "kubevirt.io:system:server:" + component,
			},
			DNSNames:    dnsSANs,
			IPAddresses: ipSANs,
		},
		CertificateStore: certStore,
		Usages: []cmapi.KeyUsage{
			cmapi.UsageDigitalSignature,
			cmapi.UsageKeyEncipherment,
			cmapi.UsageServerAuth,
		},
		Namespace:     namespace,
		Name:          podName,
		ClusterConfig: clusterConfig,
	}
}

// LoadCertConfigForService is used by components when acting as a client such as virt-api
// They will request certificates with the pod name in the common name.
// For example a pod named virt-api-7858454b7b-db9lb will request a certificate with CN = kubevirt.io:system:client:virt-api-7858454b7b-db9lb
func LoadCertConfigForClient(certStore certificate.Store, podName string, component string, dnsSANs []string, ipSANs []net.IP, namespace string, clusterConfig *virtconfig.ClusterConfig) *CertificateRequestCertificateManagerConfig {
	return &CertificateRequestCertificateManagerConfig{
		Template: &x509.CertificateRequest{
			Subject: pkix.Name{
				Organization: []string{"kubevirt.io:system"},
				CommonName:   "kubevirt.io:system:client:" + podName,
			},
			DNSNames:    dnsSANs,
			IPAddresses: ipSANs,
		},
		CertificateStore: certStore,
		Usages: []cmapi.KeyUsage{
			cmapi.UsageDigitalSignature,
			cmapi.UsageKeyEncipherment,
			cmapi.UsageClientAuth,
		},
		Namespace:     namespace,
		Name:          podName,
		ClusterConfig: clusterConfig,
	}
}

// LoadCertConfigForService is used virt-handler
// It will request certificates with the pod name in the common name.
// For example a pod named virt-handler-b5xm5 will request a certificate with CN = kubevirt.io:system:node:virt-handler-b5xm5
func LoadCertConfigForNode(certStore certificate.Store, nodeName string, component string, dnsSANs []string, ipSANs []net.IP, namespace string, clusterConfig *virtconfig.ClusterConfig) *CertificateRequestCertificateManagerConfig {
	return &CertificateRequestCertificateManagerConfig{
		Template: &x509.CertificateRequest{
			Subject: pkix.Name{
				Organization: []string{"kubevirt.io:system"},
				CommonName:   "kubevirt.io:system:node:" + nodeName,
			},
			DNSNames:    dnsSANs,
			IPAddresses: ipSANs,
		},
		CertificateStore: certStore,
		Usages: []cmapi.KeyUsage{
			cmapi.UsageDigitalSignature,
			cmapi.UsageKeyEncipherment,
			cmapi.UsageServerAuth,
		},
		Namespace:     namespace,
		Name:          nodeName,
		ClusterConfig: clusterConfig,
	}
}
