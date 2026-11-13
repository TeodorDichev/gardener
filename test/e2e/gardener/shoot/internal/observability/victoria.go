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
	"k8s.io/apimachinery/pkg/types"

	"github.com/gardener/gardener/pkg/client/kubernetes"
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

// ItShouldWaitForLogsCountByKeyToBeInShootVictoriaLogs waits for a specific number of logs matching key=~value to be present in shoot VictoriaLogs.
func ItShouldWaitForLogsCountByKeyToBeInShootVictoriaLogs(tc *ShootContext, key, value string, expectedLogCount int) {
	GinkgoHelper()

	It("Wait for exact number of logs by key to appear in shoot VictoriaLogs", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			return checkLogsCount(ctx, tc.ControlPlaneNamespace, "victoria-logs", key, value, expectedLogCount, tc.SeedClientSet)
		}).WithTimeout(time.Minute).WithPolling(time.Second).Should(Succeed())
	}, SpecTimeout(2*time.Minute))
}

// ItShouldWaitForLogsCountByKeyToBeInGardenVictoriaLogs waits for a specific number of logs matching key=~value to be present in garden VictoriaLogs.
func ItShouldWaitForLogsCountByKeyToBeInGardenVictoriaLogs(tc *SeedContext, key, value string, expectedLogCount int) {
	GinkgoHelper()

	It("Wait for exact number of logs by key to appear in garden VictoriaLogs", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			return checkLogsCount(ctx, "garden", "victoria-logs-garden", key, value, expectedLogCount, tc.SeedClientSet)
		}).WithTimeout(time.Minute).WithPolling(time.Second).Should(Succeed())
	}, SpecTimeout(2*time.Minute))
}

// ItShouldWaitForLogsByKeyToBeInShootVictoriaLogs waits for any logs matching key=~value to be present in shoot VictoriaLogs.
func ItShouldWaitForLogsByKeyToBeInShootVictoriaLogs(tc *ShootContext, key, value string) {
	GinkgoHelper()

	It("Wait for logs by key to appear in shoot VictoriaLogs", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			return checkLogsPresent(ctx, tc.ControlPlaneNamespace, "victoria-logs", key, value, tc.SeedClientSet)
		}).WithTimeout(time.Minute).WithPolling(time.Second).Should(Succeed())
	}, SpecTimeout(2*time.Minute))
}

// ItShouldWaitForLogsByKeyNotToBeInShootVictoriaLogs checks that logs matching key=~value are NOT present in shoot VictoriaLogs.
func ItShouldWaitForLogsByKeyNotToBeInShootVictoriaLogs(tc *ShootContext, key, value string) {
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

// ItShouldWaitForLogsByKeyNotToBeInGardenVictoriaLogs checks that logs matching key=~value are NOT present in garden VictoriaLogs.
func ItShouldWaitForLogsByKeyNotToBeInGardenVictoriaLogs(tc *SeedContext, key, value string) {
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
