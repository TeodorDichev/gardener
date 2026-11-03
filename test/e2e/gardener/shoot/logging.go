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

	ItShouldHaveVictoriaLogsReady(tc)
	ItShouldHaveVictoriaLogsReadyInGarden(tc.SeedContext)

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

	ItShouldRejectUnauthenticatedVictoriaLogsRequestInShoot(tc)
	ItShouldRejectWrongCredentialsVictoriaLogsRequestInShoot(tc)

	ItShouldWaitForLogsCountByKeyToBeInShootVictoriaLogs(tc, "k8s.pod.name", gardenerLoggerName+".*", shootLogsCount)
	ItShouldWaitForLogsByKeyNotToBeInShootVictoriaLogs(tc, "k8s.pod.name", nonGardenerLoggerName+".*")

	ItShouldWaitForLogsByKeyToBeInShootVictoriaLogs(tc, "unit", "containerd.service")
	ItShouldWaitForLogsByKeyToBeInShootVictoriaLogs(tc, "unit", "kubelet.service")
	ItShouldWaitForLogsByKeyToBeInShootVictoriaLogs(tc, "k8s.pod.name", "event-logger"+".*")

	ItShouldWaitForLogsByKeyNotToBeInGardenVictoriaLogs(tc.SeedContext, "k8s.pod.name", gardenerLoggerName+".*")
	ItShouldWaitForLogsByKeyNotToBeInGardenVictoriaLogs(tc.SeedContext, "k8s.pod.name", nonGardenerLoggerName+".*")
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

	ItShouldHaveVictoriaLogsReady(tc)
	ItShouldHaveVictoriaLogsReadyInGarden(tc.SeedContext)

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

	ItShouldRejectUnauthenticatedVictoriaLogsRequestInGarden(tc.SeedContext)
	ItShouldRejectWrongCredentialsVictoriaLogsRequestInGarden(tc.SeedContext)

	ItShouldWaitForLogsCountByKeyToBeInShootVictoriaLogs(tc, "k8s.pod.name", controlPlaneLoggerName+".*", shootLogsCount)
	ItShouldWaitForLogsByKeyNotToBeInShootVictoriaLogs(tc, "k8s.pod.name", gardenNamespaceLoggerName+".*")

	ItShouldWaitForLogsCountByKeyToBeInGardenVictoriaLogs(tc.SeedContext, "k8s.pod.name", gardenNamespaceLoggerName+".*", shootLogsCount)
	ItShouldWaitForLogsByKeyNotToBeInGardenVictoriaLogs(tc.SeedContext, "k8s.pod.name", controlPlaneLoggerName+".*")
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

	ItShouldHaveVictoriaLogsReady(tc)

	brokenLoggerName := "broken-logger" + "-" + utilrand.String(randomLength)
	recoveredLoggerName := "recovered-logger" + "-" + utilrand.String(randomLength)

	brokenLoggerParamsFor := map[string]any{
		"LoggerName":          brokenLoggerName,
		"HelmDeployNamespace": "kube-system",
		"AppLabel":            brokenLoggerName,
		"LogsCount":           shootLogsCount,
		"LogsDuration":        "20s",
		"AdditionalLabels": map[string]string{
			"origin":                              "gardener",
			"gardener.cloud/role":                 "system-component",
			"resources.gardener.cloud/managed-by": "gardener",
		},
	}

	recoveredLoggerParamsFor := map[string]any{
		"LoggerName":          recoveredLoggerName,
		"HelmDeployNamespace": "kube-system",
		"AppLabel":            recoveredLoggerName,
		"LogsCount":           shootLogsCount,
		"LogsDuration":        "20s",
		"AdditionalLabels": map[string]string{
			"origin":                              "gardener",
			"gardener.cloud/role":                 "system-component",
			"resources.gardener.cloud/managed-by": "gardener",
		},
	}

	ItShouldHaveVictoriaLogsReady(tc)

	// The shoot-side token secret has a 720h lifespan; GRM reconciles at 80%.
	// Patching it directly is safe - no restoration will happen during the test.
	ItShouldBreakOpenTelemetryCollectorToken(tc)
	ItShouldWaitForInvalidTokenOnNodes(tc)

	ItShouldRenderAndDeployTemplateToShoot(tc, loggerAppName, brokenLoggerParamsFor)
	ItShouldWaitForPodsInShootToBeReady(tc, "kube-system", labels.SelectorFromSet(map[string]string{"app": brokenLoggerName}))
	ItShouldWaitForLogsByKeyNotToBeInShootVictoriaLogs(tc, "k8s.pod.name", brokenLoggerName+".*")

	ItShouldRestoreOpenTelemetryCollectorToken(tc)
	ItShouldWaitForValidTokenOnNodes(tc)

	ItShouldRenderAndDeployTemplateToShoot(tc, loggerAppName, recoveredLoggerParamsFor)
	ItShouldWaitForPodsInShootToBeReady(tc, "kube-system", labels.SelectorFromSet(map[string]string{"app": recoveredLoggerName}))
	ItShouldWaitForLogsCountByKeyToBeInShootVictoriaLogs(tc, "k8s.pod.name", recoveredLoggerName+".*", shootLogsCount)
}

func itShouldSetUpShootAndSeedContext(tc *ShootContext) {
	ItShouldWaitForShootToBeReconciledAndHealthy(tc)
	ItShouldInitializeShootClient(tc)
	ItShouldGetResponsibleSeed(tc)
	seed.ItShouldInitializeSeedClient(tc.SeedContext)
	ItShouldComputeControlPlaneNamespace(tc)
}
