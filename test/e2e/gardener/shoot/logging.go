// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"k8s.io/apimachinery/pkg/labels"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/seed"
	. "github.com/gardener/gardener/test/e2e/gardener/shoot/internal/observability"
)

var (
	valiLabels = map[string]string{
		"app":  "vali",
		"role": "logging",
	}

	randomLength   = 11
	shootLogsCount = 100
	loggerAppName  = "logger-app.yaml.tpl"
)

// ShootLoggingVali checks that the logging stack for shoots works as expected using Vali.
// It deploys two logger applications: one with Gardener labels and one without.
// It's expected that the one with Gardener labels has its logs collected in Vali by
// the log shipper (valitail). While the one without label does not.
// Function is exposed so that it can be called from the context of another
// e2e test. This is done so that we can minimize the number of shoot clusters
// that we create.
//
// The test expects that the RemoveVali feature gate is turned off
func ShootLoggingVali(tc *ShootContext) {
	itShouldSetUpShootAndSeedContext(tc)

	gardenerLoggerName := "gardener-logger" + "-" + utilrand.String(randomLength)
	nonGardenerLoggerName := "non-gardener-logger" + "-" + utilrand.String(randomLength)

	loggerParams := map[string]any{
		"LoggerName":          gardenerLoggerName,
		"HelmDeployNamespace": "kube-system",
		"AppLabel":            gardenerLoggerName,
		"LogsCount":           shootLogsCount,
		"LogsDuration":        "20s",
		"AdditionalLabels": map[string]string{
			"origin":                              "gardener",
			"gardener.cloud/role":                 "system-component",
			"resources.gardener.cloud/managed-by": "gardener",
		},
	}
	ItShouldRenderAndDeployTemplateToShoot(tc, loggerAppName, loggerParams)

	loggerParams = map[string]any{
		"LoggerName":          nonGardenerLoggerName,
		"HelmDeployNamespace": "kube-system",
		"AppLabel":            nonGardenerLoggerName,
		"LogsCount":           shootLogsCount,
		"LogsDuration":        "20s",
	}
	ItShouldRenderAndDeployTemplateToShoot(tc, loggerAppName, loggerParams)

	ItShouldWaitForPodsInShootToBeReady(tc, "kube-system", labels.SelectorFromSet(map[string]string{"app": gardenerLoggerName}))
	ItShouldWaitForPodsInShootToBeReady(tc, "kube-system", labels.SelectorFromSet(map[string]string{"app": nonGardenerLoggerName}))

	ItShouldWaitForLogsCountWithLabelToBeInVali(tc, valiLabels, "pod_name", gardenerLoggerName+".*", shootLogsCount)
	ItShouldWaitForLogsWithLabelToNotBeInVali(tc, valiLabels, "pod_name", nonGardenerLoggerName+".*")

	ItShouldWaitForLogsWithLabelToBeInVali(tc, valiLabels, "unit", "containerd.service")
	ItShouldWaitForLogsWithLabelToBeInVali(tc, valiLabels, "unit", "kubelet.service")
}

// ShootLoggingVl checks the shoot logging stack of VictoriaLogs. It deploys two loggers
// to the shoot's kube-system namespace: one with Gardener labels (expects logs in shoot
// VictoriaLogs) and one without (expects no logs in shoot VictoriaLogs). It also verifies
// shoot VictoriaLogs ingress auth and systemd unit log ingestion.
//
// Function is exposed so that it can be called from the context of another e2e test.
// This is done so that we can minimize the number of shoot clusters that we create.
//
// The test expects that the VictoriaLogsBackend and OpenTelemetryCollector are true
func ShootLoggingVl(tc *ShootContext) {
	itShouldSetUpShootAndSeedContext(tc)

	gardenerLoggerName := "gardener-logger" + "-" + utilrand.String(randomLength)
	nonGardenerLoggerName := "non-gardener-logger" + "-" + utilrand.String(randomLength)

	loggerParams := map[string]any{
		"LoggerName":          gardenerLoggerName,
		"HelmDeployNamespace": "kube-system",
		"AppLabel":            gardenerLoggerName,
		"LogsCount":           shootLogsCount,
		"LogsDuration":        "20s",
		"AdditionalLabels": map[string]string{
			"origin":                              "gardener",
			"gardener.cloud/role":                 "system-component",
			"resources.gardener.cloud/managed-by": "gardener",
		},
	}
	ItShouldRenderAndDeployTemplateToShoot(tc, loggerAppName, loggerParams)

	loggerParams = map[string]any{
		"LoggerName":          nonGardenerLoggerName,
		"HelmDeployNamespace": "kube-system",
		"AppLabel":            nonGardenerLoggerName,
		"LogsCount":           shootLogsCount,
		"LogsDuration":        "20s",
	}
	ItShouldRenderAndDeployTemplateToShoot(tc, loggerAppName, loggerParams)

	ItShouldWaitForPodsInShootToBeReady(tc, "kube-system", labels.SelectorFromSet(map[string]string{"app": gardenerLoggerName}))
	ItShouldWaitForPodsInShootToBeReady(tc, "kube-system", labels.SelectorFromSet(map[string]string{"app": nonGardenerLoggerName}))

	ItShouldHaveVictoriaLogsReady(tc)
	ItShouldHaveVictoriaLogsReadyInGarden(tc.SeedContext)
	ItShouldRejectUnauthenticatedVictoriaLogsRequestInShoot(tc)
	ItShouldRejectWrongCredentialsVictoriaLogsRequestInShoot(tc)

	ItShouldWaitForLogsCountByKeyInVictoriaLogsInShoot(tc, "k8s.pod.name", gardenerLoggerName+".*", shootLogsCount)
	ItShouldWaitForLogsNotByKeyInVictoriaLogsInShoot(tc, "k8s.pod.name", nonGardenerLoggerName+".*")

	ItShouldWaitForLogsByKeyInVictoriaLogsInShoot(tc, "unit", "containerd.service")
	ItShouldWaitForLogsByKeyInVictoriaLogsInShoot(tc, "unit", "kubelet.service")
	ItShouldWaitForLogsByKeyInVictoriaLogsInShoot(tc, "k8s.pod.name", "event-logger"+".*")

	ItShouldWaitForLogsNotByKeyInVictoriaLogsInGarden(tc.SeedContext, "k8s.pod.name", gardenerLoggerName+".*")
	ItShouldWaitForLogsNotByKeyInVictoriaLogsInGarden(tc.SeedContext, "k8s.pod.name", nonGardenerLoggerName+".*")
}

// SeedLoggingVl checks the seed-side routing of the shoot logging stack of VictoriaLogs.
// It deploys two logger applications. One in the shoot's control-plane namespace on the
// seed, and another in the seed's garden namespace. Both loggers are asserted to appear
// only in their respective VictoriaLogs instance and not in the other.
// Garden VictoriaLogs ingress auth is also verified.
//
// Function is exposed so that it can be called from the context of another e2e test.
// This is done so that we can minimize the number of shoot clusters that we create.
//
// The test expects that the VictoriaLogsBackend and OpenTelemetryCollector are true.
func SeedLoggingVl(tc *ShootContext) {
	itShouldSetUpShootAndSeedContext(tc)

	controlPlaneLoggerName := "seed-cp-logger" + "-" + utilrand.String(randomLength)
	gardenNamespaceLoggerName := "seed-garden-ns-logger" + "-" + utilrand.String(randomLength)

	// We omit "HelmDeployNamespace" - because its known only at runtime
	cpLoggerParams := map[string]any{
		"LoggerName":   controlPlaneLoggerName,
		"AppLabel":     controlPlaneLoggerName,
		"LogsCount":    shootLogsCount,
		"LogsDuration": "20s",
	}
	ItShouldRenderAndDeployTemplateToShootControlPlane(tc, loggerAppName, cpLoggerParams)

	cpLoggerLabels := labels.SelectorFromSet(map[string]string{"app": controlPlaneLoggerName})
	ItShouldWaitForPodsInShootControlPlaneToBeReady(tc, cpLoggerLabels)

	gardenNsLoggerParams := map[string]any{
		"LoggerName":          gardenNamespaceLoggerName,
		"HelmDeployNamespace": "garden",
		"AppLabel":            gardenNamespaceLoggerName,
		"LogsCount":           shootLogsCount,
		"LogsDuration":        "20s",
	}
	seed.ItShouldRenderAndDeployTemplateToGarden(tc.SeedContext, loggerAppName, gardenNsLoggerParams)

	gardenNsLoggerLabels := labels.SelectorFromSet(map[string]string{"app": gardenNamespaceLoggerName})
	seed.ItShouldWaitForPodsInGardenNamespaceToBeReady(tc.SeedContext, gardenNsLoggerLabels)

	ItShouldHaveVictoriaLogsReady(tc)
	ItShouldHaveVictoriaLogsReadyInGarden(tc.SeedContext)
	ItShouldRejectUnauthenticatedVictoriaLogsRequestInGarden(tc.SeedContext)
	ItShouldRejectWrongCredentialsVictoriaLogsRequestInGarden(tc.SeedContext)

	ItShouldWaitForLogsCountByKeyInVictoriaLogsInShoot(tc, "k8s.pod.name", controlPlaneLoggerName+".*", shootLogsCount)
	ItShouldWaitForLogsNotByKeyInVictoriaLogsInShoot(tc, "k8s.pod.name", gardenNamespaceLoggerName+".*")

	ItShouldWaitForLogsCountByKeyInVictoriaLogsInGarden(tc.SeedContext, "k8s.pod.name", gardenNamespaceLoggerName+".*", shootLogsCount)
	ItShouldWaitForLogsNotByKeyInVictoriaLogsInGarden(tc.SeedContext, "k8s.pod.name", controlPlaneLoggerName+".*")

	// cleaning up the garden, everything else is cleaned up when the shoot is deleted
	seed.ItShouldDeleteTemplateFromGarden(tc.SeedContext, loggerAppName, gardenNsLoggerParams)
}

// ShootLogShipper verifies that the shoot's systemd OpenTelemetry collector log shipper self-heals after its
// auth token is broken and later restored.
//
// Function is exposed so that it can be called from the context of another e2e test. This is done so that we can
// minimize the number of shoot clusters that we create.
//
// The test expects that the VictoriaLogsBackend and OpenTelemetryCollector feature gates are true and that shoot node
// logging is enabled.
func ShootLogShipper(tc *ShootContext) {
	itShouldSetUpShootAndSeedContext(tc)

	brokenLoggerName := "broken-logger" + "-" + utilrand.String(randomLength)
	recoveredLoggerName := "recovered-logger" + "-" + utilrand.String(randomLength)

	loggerParamsFor := func(name string) map[string]any {
		return map[string]any{
			"LoggerName":          name,
			"HelmDeployNamespace": "kube-system",
			"AppLabel":            name,
			"LogsCount":           shootLogsCount,
			"LogsDuration":        "20s",
			"AdditionalLabels": map[string]string{
				"origin":                              "gardener",
				"gardener.cloud/role":                 "system-component",
				"resources.gardener.cloud/managed-by": "gardener",
			},
		}
	}

	ItShouldHaveVictoriaLogsReady(tc)

	ItShouldBreakOpenTelemetryCollectorToken(tc)

	ItShouldRenderAndDeployTemplateToShoot(tc, loggerAppName, loggerParamsFor(brokenLoggerName))
	ItShouldWaitForPodsInShootToBeReady(tc, "kube-system", labels.SelectorFromSet(map[string]string{"app": brokenLoggerName}))
	ItShouldWaitForLogsNotByKeyInVictoriaLogsInShoot(tc, "k8s.pod.name", brokenLoggerName+".*")

	ItShouldRestoreOpenTelemetryCollectorToken(tc)

	ItShouldRenderAndDeployTemplateToShoot(tc, loggerAppName, loggerParamsFor(recoveredLoggerName))
	ItShouldWaitForPodsInShootToBeReady(tc, "kube-system", labels.SelectorFromSet(map[string]string{"app": recoveredLoggerName}))
	ItShouldWaitForLogsCountByKeyInVictoriaLogsInShoot(tc, "k8s.pod.name", recoveredLoggerName+".*", shootLogsCount)
}

func itShouldSetUpShootAndSeedContext(tc *ShootContext) {
	ItShouldWaitForShootToBeReconciledAndHealthy(tc)
	ItShouldInitializeShootClient(tc)
	ItShouldGetResponsibleSeed(tc)
	seed.ItShouldInitializeSeedClient(tc.SeedContext)
	ItShouldComputeControlPlaneNamespace(tc)
}
