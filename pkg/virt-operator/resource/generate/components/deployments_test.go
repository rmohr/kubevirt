package components

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Deployments", func() {
	It("should create Prometheus service that is headless", func() {
		service := NewPrometheusService("mynamespace")
		Expect(service.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
		Expect(service.Spec.ClusterIP).To(Equal(corev1.ClusterIPNone))
	})

	It("should preserve extra args on virt-controller", func() {
		deployment, err := NewControllerDeployment("a",
			"b",
			"c",
			"d",
			"d",
			"d",
			"d",
			"d",
			"d",
			"d",
			"d",
			"d",
			"d",
			"d",
			"d",
			nil,
			"d",
			map[string]string{
				"a": "b",
			},
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(ContainElements(corev1.EnvVar{Name: "a", Value: "b"}))
		Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(HaveLen(3))
	})

	It("should preserve extra args on virt-api", func() {
		deployment, err := NewApiServerDeployment("a",
			"d",
			"d",
			"d",
			"d",
			"d",
			"d",
			"d",
			"d",
			nil,
			"d",
			map[string]string{
				"a": "b",
			},
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(ContainElements(corev1.EnvVar{Name: "a", Value: "b"}))
		Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(HaveLen(3))
	})
})
