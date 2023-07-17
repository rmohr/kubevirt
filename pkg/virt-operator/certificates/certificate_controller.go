// Package certificates implements an abstract controller that is useful for
// building controllers that manage CSRs
package certificates

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"
	"k8s.io/kube-aggregator/pkg/controllers"

	"kubevirt.io/client-go/log"

	"kubevirt.io/kubevirt/pkg/controller"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	certificateslisters "k8s.io/client-go/listers/certificates/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
)

type CertificateController struct {
	ctx        context.Context
	kubeClient clientset.Interface
	cmClient   *cmclient.Clientset
	namespace  string

	crLister  certificateslisters.CertificateSigningRequestLister
	crStore   cache.Store
	crsSynced cache.InformerSynced

	handler func(*cmapi.CertificateRequest) error

	queue workqueue.RateLimitingInterface
}

func NewCertificateController(
	ctx context.Context,
	kubeClient clientset.Interface,
	cmClient *cmclient.Clientset,
	namespace string,
	crInformer cache.SharedIndexInformer,
	handler func(*cmapi.CertificateRequest) error,
) *CertificateController {
	// Send events to the apiserver
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	cc := &CertificateController{
		ctx:        ctx,
		kubeClient: kubeClient,
		cmClient:   cmClient,
		namespace:  namespace,
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(200*time.Millisecond, 1000*time.Second),
			// 10 qps, 100 bucket size.  This is only for retry speed and its only the overall factor (not per item)
			&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
		), "certificate"),
		handler: handler,
	}

	// Manage the addition/update of certificate requests
	crInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			cc.enqueueCertificateRequest(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			cc.enqueueCertificateRequest(new)
		},
		DeleteFunc: func(obj interface{}) {
			_, ok := obj.(*cmapi.CertificateRequest)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				_, ok = tombstone.Obj.(*cmapi.CertificateRequest)
				if !ok {
					return
				}
			}
			cc.enqueueCertificateRequest(obj)
		},
	})
	cc.crStore = crInformer.GetStore()
	cc.crsSynced = crInformer.HasSynced
	return cc
}

// Run the main goroutine responsible for watching and syncing jobs.
func (cc *CertificateController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer cc.queue.ShutDown()

	log.Log.Infof("Starting certificate controller")
	defer log.Log.Infof("Shutting down certificate controller")

	if !controllers.WaitForCacheSync("certificate", stopCh, cc.crsSynced) {
		log.Log.Infof("certificate controller has not synced")
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(cc.worker, time.Second, stopCh)
	}

	<-stopCh
}

// worker runs a thread that dequeues CSRs, handles them, and marks them done.
func (cc *CertificateController) worker() {
	for cc.processNextWorkItem() {
	}
}

// processNextWorkItem deals with one key off the queue.  It returns false when it's time to quit.
func (cc *CertificateController) processNextWorkItem() bool {
	cKey, quit := cc.queue.Get()
	if quit {
		return false
	}
	defer cc.queue.Done(cKey)

	if err := cc.syncFunc(cKey.(string)); err != nil {
		cc.queue.AddRateLimited(cKey)
		if _, ignorable := err.(ignorableError); !ignorable {
			utilruntime.HandleError(fmt.Errorf("Sync %v failed with : %v", cKey, err))
		} else {
			log.Log.V(4).Infof("Sync %v failed with : %v", cKey, err)
		}
		return true
	}

	cc.queue.Forget(cKey)
	return true

}

func (cc *CertificateController) enqueueCertificateRequest(obj interface{}) {
	key, err := controller.KeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %+v: %v", obj, err))
		return
	}
	cc.queue.Add(key)
}

// maybeSignCertificate will inspect the certificate request and, if it has
// been approved and meets policy expectations, generate an X509 cert using the
// cluster CA assets. If successful it will update the CSR approve subresource
// with the signed certificate.
func (cc *CertificateController) syncFunc(key string) error {
	startTime := time.Now()
	defer func() {
		log.Log.V(4).Infof("Finished syncing certificate request %q (%v)", key, time.Since(startTime))
	}()
	obj, exists, err := cc.crStore.GetByKey(key)
	if !exists {
		log.Log.Infof("cr has been deleted: %v", key)
		return nil
	}
	if err != nil {
		return err
	}

	cr := obj.(*cmapi.CertificateRequest)

	if cr.Status.Certificate != nil {
		// no need to do anything because it already has a cert
		return nil
	}

	// need to operate on a copy so we don't mutate the csr in the shared cache
	cr = cr.DeepCopy()

	return cc.handler(cr)
}

// IgnorableError returns an error that we shouldn't handle (i.e. log) because
// it's spammy and usually user error. Instead we will log these errors at a
// higher log level. We still need to throw these errors to signal that the
// sync should be retried.
func IgnorableError(s string, args ...interface{}) ignorableError {
	return ignorableError(fmt.Sprintf(s, args...))
}

type ignorableError string

func (e ignorableError) Error() string {
	return string(e)
}
