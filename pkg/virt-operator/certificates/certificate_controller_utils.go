package certificates

import (
	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"

	log "kubevirt.io/client-go/log"
)

// Clock is defined as a package var so it can be stubbed out during tests.
var Clock clock.Clock = clock.RealClock{}

// IsCertificateRequestApproved returns true if a certificate request has the
// "Approved" condition and no "Denied" conditions; false otherwise.
func IsCertificateRequestApproved(cr *cmapi.CertificateRequest) bool {
	approved, denied := GetCertApprovalCondition(&cr.Status)
	return approved && !denied
}

func GetCertApprovalCondition(status *cmapi.CertificateRequestStatus) (approved bool, denied bool) {
	for _, c := range status.Conditions {
		if c.Type == cmapi.CertificateRequestConditionApproved {
			approved = true
		}
		if c.Type == cmapi.CertificateRequestConditionDenied {
			denied = true
		}
	}
	return
}

func SetCertificateRequestCondition(cr *cmapi.CertificateRequest, conditionType cmapi.CertificateRequestConditionType, status cmmeta.ConditionStatus, reason, message string) {
	newCondition := cmapi.CertificateRequestCondition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}

	nowTime := metav1.NewTime(Clock.Now())
	newCondition.LastTransitionTime = &nowTime

	// Search through existing conditions
	for idx, cond := range cr.Status.Conditions {
		// Skip unrelated conditions
		if cond.Type != conditionType {
			continue
		}

		// If this update doesn't contain a state transition, we don't update
		// the conditions LastTransitionTime to Now()
		if cond.Status == status {
			newCondition.LastTransitionTime = cond.LastTransitionTime
		} else {
			log.Log.Infof("Found status change for CertificateRequest %q condition %q: %q -> %q; setting lastTransitionTime to %v", cr.Name, conditionType, cond.Status, status, nowTime.Time)
		}

		// Overwrite the existing condition
		cr.Status.Conditions[idx] = newCondition
		return
	}

	// If we've not found an existing condition of this type, we simply insert
	// the new condition into the slice.
	cr.Status.Conditions = append(cr.Status.Conditions, newCondition)
	log.Log.Infof("Setting lastTransitionTime for CertificateRequest %q condition %q to %v", cr.Name, conditionType, nowTime.Time)
}
