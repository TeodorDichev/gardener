// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"fmt"
	"net/http"
	"time"

	victoriametricsv1 "github.com/VictoriaMetrics/operator/api/operator/v1"
	victoriametricsv1beta1 "github.com/VictoriaMetrics/operator/api/operator/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	otelcollectorconstants "github.com/gardener/gardener/pkg/component/observability/opentelemetry/collector/constants"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/utils/logging"
)

// ItShouldHaveVictoriaLogsReady waits for the VLSingle CR in the shoot's control plane namespace to reach operational status.
func ItShouldHaveVictoriaLogsReady(tc *ShootContext) {
	GinkgoHelper()

	It("Wait for VictoriaLogs to be ready", func(ctx SpecContext) {
		vl := &victoriametricsv1.VLSingle{}
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.SeedClient.Get(ctx, types.NamespacedName{Name: "victoria-logs", Namespace: tc.ControlPlaneNamespace}, vl)).To(Succeed())
			g.Expect(vl.Status.UpdateStatus).To(Equal(victoriametricsv1beta1.UpdateStatusOperational))
		}).Should(Succeed())
	}, SpecTimeout(5*time.Minute))
}

// ItShouldHaveVictoriaLogsReadyInGarden waits for the VLSingle CR in the garden namespace to reach operational status.
func ItShouldHaveVictoriaLogsReadyInGarden(tc *SeedContext) {
	GinkgoHelper()

	It("Wait for garden VictoriaLogs to be ready", func(ctx SpecContext) {
		vl := &victoriametricsv1.VLSingle{}
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.SeedClient.Get(ctx, types.NamespacedName{Name: "victoria-logs", Namespace: "garden"}, vl)).To(Succeed())
			g.Expect(vl.Status.UpdateStatus).To(Equal(victoriametricsv1beta1.UpdateStatusOperational))
		}).Should(Succeed())
	}, SpecTimeout(5*time.Minute))
}

// ItShouldWaitForLogsCountByKeyInVictoriaLogsInShoot waits for a specific number of logs matching key=~value to be present in shoot VictoriaLogs.
func ItShouldWaitForLogsCountByKeyInVictoriaLogsInShoot(tc *ShootContext, key, value string, expectedLogCount int) {
	GinkgoHelper()

	It("Wait for exact number of logs by key to appear in shoot VictoriaLogs", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			return checkLogsCount(ctx, tc.ControlPlaneNamespace, "victoria-logs", key, value, expectedLogCount, tc.SeedClientSet)
		}).WithTimeout(time.Minute).WithPolling(time.Second).Should(Succeed())
	}, SpecTimeout(2*time.Minute))
}

// ItShouldWaitForLogsCountByKeyInVictoriaLogsInGarden waits for a specific number of logs matching key=~value to be present in garden VictoriaLogs.
func ItShouldWaitForLogsCountByKeyInVictoriaLogsInGarden(tc *SeedContext, key, value string, expectedLogCount int) {
	GinkgoHelper()

	It("Wait for exact number of logs by key to appear in garden VictoriaLogs", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			return checkLogsCount(ctx, "garden", "victoria-logs-garden", key, value, expectedLogCount, tc.SeedClientSet)
		}).WithTimeout(time.Minute).WithPolling(time.Second).Should(Succeed())
	}, SpecTimeout(2*time.Minute))
}

// ItShouldWaitForLogsByKeyInVictoriaLogsInShoot waits for any logs matching key=~value to be present in shoot VictoriaLogs.
func ItShouldWaitForLogsByKeyInVictoriaLogsInShoot(tc *ShootContext, key, value string) {
	GinkgoHelper()

	It("Wait for logs by key to appear in shoot VictoriaLogs", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			return checkLogsPresent(ctx, tc.ControlPlaneNamespace, "victoria-logs", key, value, tc.SeedClientSet)
		}).WithTimeout(time.Minute).WithPolling(time.Second).Should(Succeed())
	}, SpecTimeout(2*time.Minute))
}

// ItShouldWaitForLogsByKeyInVictoriaLogsInGarden waits for any logs matching key=~value to be present in garden VictoriaLogs.
func ItShouldWaitForLogsByKeyInVictoriaLogsInGarden(tc *SeedContext, key, value string) {
	GinkgoHelper()

	It("Wait for logs by key to appear in garden VictoriaLogs", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			return checkLogsPresent(ctx, "garden", "victoria-logs-garden", key, value, tc.SeedClientSet)
		}).WithTimeout(time.Minute).WithPolling(time.Second).Should(Succeed())
	}, SpecTimeout(2*time.Minute))
}

// ItShouldWaitForLogsNotByKeyInVictoriaLogsInShoot checks that logs matching key=~value are NOT present in shoot VictoriaLogs.
func ItShouldWaitForLogsNotByKeyInVictoriaLogsInShoot(tc *ShootContext, key, value string) {
	GinkgoHelper()

	// Consistently is used because the only way to assert absence is to observe
	// no logs over a window of time, not just at a single point.
	It("Ensure logs do not exist in shoot VictoriaLogs", func(ctx SpecContext) {
		Consistently(ctx, func() error {
			logCount, err := logging.GetVictoriaLogs(ctx, 60, tc.ControlPlaneNamespace, "victoria-logs", key, value, tc.SeedClientSet)
			if err != nil {
				return err
			}
			if logCount > 0 {
				StopTrying(fmt.Sprintf("found unexpected logs for %s=%s: got %d", key, value, logCount)).Now()
			}
			return nil
		}, time.Minute).WithPolling(5 * time.Second).Should(Succeed())
	}, SpecTimeout(2*time.Minute))
}

// ItShouldWaitForLogsNotByKeyInVictoriaLogsInGarden checks that logs matching key=~value are NOT present in garden VictoriaLogs.
func ItShouldWaitForLogsNotByKeyInVictoriaLogsInGarden(tc *SeedContext, key, value string) {
	GinkgoHelper()

	// Consistently is used because the only way to assert absence is to observe
	// no logs over a window of time, not just at a single point.
	It("Ensure logs do not exist in garden VictoriaLogs", func(ctx SpecContext) {
		Consistently(ctx, func() error {
			logCount, err := logging.GetVictoriaLogs(ctx, 60, "garden", "victoria-logs-garden", key, value, tc.SeedClientSet)
			if err != nil {
				return err
			}
			if logCount > 0 {
				StopTrying(fmt.Sprintf("found unexpected logs for %s=%s: got %d", key, value, logCount)).Now()
			}
			return nil
		}, time.Minute).WithPolling(5 * time.Second).Should(Succeed())
	}, SpecTimeout(2*time.Minute))
}

// ItShouldRejectUnauthenticatedVictoriaLogsRequestInShoot asserts that a request to the shoot VictoriaLogs ingress without credentials is rejected with HTTP 401.
func ItShouldRejectUnauthenticatedVictoriaLogsRequestInShoot(tc *ShootContext) {
	GinkgoHelper()

	It("shoot VictoriaLogs ingress should reject unauthenticated request with 401", func(ctx SpecContext) {
		status, err := logging.QueryVictoriaLogsStatus(ctx, tc.ControlPlaneNamespace, "victoria-logs", "", "", tc.SeedClientSet)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal(http.StatusUnauthorized))
	}, SpecTimeout(time.Minute))
}

// ItShouldRejectUnauthenticatedVictoriaLogsRequestInGarden asserts that a request to the garden VictoriaLogs ingress without credentials is rejected with HTTP 401.
func ItShouldRejectUnauthenticatedVictoriaLogsRequestInGarden(tc *SeedContext) {
	GinkgoHelper()

	It("garden VictoriaLogs ingress should reject unauthenticated request with 401", func(ctx SpecContext) {
		status, err := logging.QueryVictoriaLogsStatus(ctx, "garden", "victoria-logs-garden", "", "", tc.SeedClientSet)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal(http.StatusUnauthorized))
	}, SpecTimeout(time.Minute))
}

// ItShouldRejectWrongCredentialsVictoriaLogsRequestInShoot asserts that a request to the shoot VictoriaLogs ingress with invalid credentials is rejected with HTTP 401.
func ItShouldRejectWrongCredentialsVictoriaLogsRequestInShoot(tc *ShootContext) {
	GinkgoHelper()

	It("shoot VictoriaLogs ingress should reject wrong credentials with 401", func(ctx SpecContext) {
		status, err := logging.QueryVictoriaLogsStatus(ctx, tc.ControlPlaneNamespace, "victoria-logs", "wrong-user", "wrong-password", tc.SeedClientSet)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal(http.StatusUnauthorized))
	}, SpecTimeout(time.Minute))
}

// ItShouldRejectWrongCredentialsVictoriaLogsRequestInGarden asserts that a request to the garden VictoriaLogs ingress with invalid credentials is rejected with HTTP 401.
func ItShouldRejectWrongCredentialsVictoriaLogsRequestInGarden(tc *SeedContext) {
	GinkgoHelper()

	It("garden VictoriaLogs ingress should reject wrong credentials with 401", func(ctx SpecContext) {
		status, err := logging.QueryVictoriaLogsStatus(ctx, "garden", "victoria-logs-garden", "wrong-user", "wrong-password", tc.SeedClientSet)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal(http.StatusUnauthorized))
	}, SpecTimeout(time.Minute))
}

const (
	annotationRestartSystemdServices = "worker.gardener.cloud/restart-systemd-services"
	invalidToken                     = "invalid-token"
)

// ItShouldBreakOpenTelemetryCollectorToken corrupts the opentelemetry-collector auth token by patching the
// secret in the shoot to an invalid value and then forcing GNA to immediately sync the token to disk via annotation.
func ItShouldBreakOpenTelemetryCollectorToken(tc *ShootContext) {
	GinkgoHelper()

	It("Break the opentelemetry-collector auth token", func(ctx SpecContext) {
		secret := &corev1.Secret{}
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.ShootClient.Get(ctx, types.NamespacedName{
				Namespace: metav1.NamespaceSystem,
				Name:      otelcollectorconstants.OpenTelemetryCollectorSecretName,
			}, secret)).To(Succeed())

			patch := client.MergeFrom(secret.DeepCopy())
			if secret.Data == nil {
				secret.Data = map[string][]byte{}
			}
			secret.Data["token"] = []byte(invalidToken)
			g.Expect(tc.ShootClient.Patch(ctx, secret, patch)).To(Succeed())
		}).WithPolling(1 * time.Second).Should(Succeed())

		restartGNA(ctx, tc)
	}, SpecTimeout(3*time.Minute))
}

// ItShouldRestoreOpenTelemetryCollectorToken forces the token-requestor to re-issue a fresh valid token into the shoot's
// kube-system/gardener-opentelemetry-collector secret
func ItShouldRestoreOpenTelemetryCollectorToken(tc *ShootContext) {
	GinkgoHelper()

	It("Restore the opentelemetry-collector auth token", func(ctx SpecContext) {
		accessSecret := &corev1.Secret{}
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.SeedClient.Get(ctx, types.NamespacedName{
				Namespace: tc.ControlPlaneNamespace,
				Name:      "shoot-access-opentelemetry-collector",
			}, accessSecret)).To(Succeed())

			patch := client.MergeFrom(accessSecret.DeepCopy())
			if accessSecret.Annotations == nil {
				accessSecret.Annotations = map[string]string{}
			}

			accessSecret.Annotations[resourcesv1alpha1.ServiceAccountTokenRenewTimestamp] = "1970-01-01T00:00:00Z"
			g.Expect(tc.SeedClient.Patch(ctx, accessSecret, patch)).To(Succeed())
		}).WithPolling(time.Second).Should(Succeed())

		Eventually(ctx, func(g Gomega) {
			freshSecret := &corev1.Secret{}
			g.Expect(tc.ShootClient.Get(ctx, types.NamespacedName{
				Namespace: metav1.NamespaceSystem,
				Name:      otelcollectorconstants.OpenTelemetryCollectorSecretName,
			}, freshSecret)).To(Succeed())

			g.Expect(freshSecret.Data).To(HaveKeyWithValue("token", Not(Or(
				BeEmpty(),
				Equal([]byte(invalidToken)),
			))), "the invalid token has not yet been replaced")

		}).WithPolling(time.Second).Should(Succeed())

		restartGNA(ctx, tc)
	}, SpecTimeout(3*time.Minute))
}

// restartGNA annotates every shoot node with the restart-systemd-services annotation for
// gardener-node-agent and waits until GNA has consumed the annotation.
func restartGNA(ctx SpecContext, tc *ShootContext) {
	GinkgoHelper()

	Eventually(ctx, func(g Gomega) {
		nodeList := &corev1.NodeList{}
		g.Expect(tc.ShootClient.List(ctx, nodeList)).To(Succeed())
		g.Expect(nodeList.Items).NotTo(BeEmpty())
		for i := range nodeList.Items {
			patch := client.MergeFrom(nodeList.Items[i].DeepCopy())
			if nodeList.Items[i].Annotations == nil {
				nodeList.Items[i].Annotations = map[string]string{}
			}

			nodeList.Items[i].Annotations[annotationRestartSystemdServices] = "gardener-node-agent"
			g.Expect(tc.ShootClient.Patch(ctx, &nodeList.Items[i], patch)).To(Succeed())
		}
	}).WithPolling(time.Second).Should(Succeed())

	Eventually(ctx, func(g Gomega) {
		nodeList := &corev1.NodeList{}
		g.Expect(tc.ShootClient.List(ctx, nodeList)).To(Succeed())

		for _, node := range nodeList.Items {
			g.Expect(node.Annotations[annotationRestartSystemdServices]).To(BeEmpty(),
				"restart annotation still present on node %s", node.Name)
		}
	}).WithPolling(time.Second).Should(Succeed())
}

func checkLogsCount(ctx SpecContext, namespace, vsName, key, value string, expectedLogCount int, k8sClient kubernetes.Interface) error {
	logCount, err := logging.GetVictoriaLogs(ctx, 60, namespace, vsName, key, value, k8sClient)
	if err != nil {
		return err
	}
	if logCount > expectedLogCount {
		StopTrying(fmt.Sprintf("received too many logs for %s=%s: got %d, expected %d", key, value, logCount, expectedLogCount)).Now()
	}
	if logCount < expectedLogCount {
		return fmt.Errorf("waiting for logs for %s=%s: got %d, expected %d", key, value, logCount, expectedLogCount)
	}
	return nil
}

func checkLogsPresent(ctx SpecContext, namespace, vsName, key, value string, k8sClient kubernetes.Interface) error {
	logCount, err := logging.GetVictoriaLogs(ctx, 60, namespace, vsName, key, value, k8sClient)
	if err != nil {
		return err
	}
	if logCount == 0 {
		return fmt.Errorf("no logs for %s=%s", key, value)
	}
	return nil
}
