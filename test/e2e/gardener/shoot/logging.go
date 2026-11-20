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
func ShootLoggingVali(tc *ShootContext) {
	itShouldSetUpShootAndSeedContext(tc)

	gardenerLoggerName := "gardener-logger" + "-" + utilrand.String(randomLength)
	nonGardenerLoggerName := "non-gardener-logger" + "-" + utilrand.String(randomLength)

	ItShouldRenderAndDeployTemplateToShootDataPlane(tc, "kube-system", loggerAppName, gardenerLoggerParams(gardenerLoggerName))
	ItShouldWaitForPodsInShootToBeReady(tc, "kube-system", labels.SelectorFromSet(map[string]string{"app": gardenerLoggerName}))

	ItShouldRenderAndDeployTemplateToShootDataPlane(tc, "kube-system", loggerAppName, nonGardenerLoggerParams(nonGardenerLoggerName))
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
func ShootLoggingVl(tc *ShootContext) {
	itShouldSetUpShootAndSeedContext(tc)

	ItShouldHaveShootVictoriaLogsReady(tc)
	ItShouldHaveGardenVictoriaLogsReady(tc.SeedContext)

	gardenerLoggerName := "gardener-logger" + "-" + utilrand.String(randomLength)
	nonGardenerLoggerName := "non-gardener-logger" + "-" + utilrand.String(randomLength)

	ItShouldRenderAndDeployTemplateToShootDataPlane(tc, "kube-system", loggerAppName, gardenerLoggerParams(gardenerLoggerName))
	ItShouldWaitForPodsInShootToBeReady(tc, "kube-system", labels.SelectorFromSet(map[string]string{"app": gardenerLoggerName}))

	ItShouldRenderAndDeployTemplateToShootDataPlane(tc, "kube-system", loggerAppName, nonGardenerLoggerParams(nonGardenerLoggerName))

	ItShouldWaitForPodsInShootToBeReady(tc, "kube-system", labels.SelectorFromSet(map[string]string{"app": nonGardenerLoggerName}))

	ItShouldRejectUnauthenticatedVictoriaLogsRequestInShoot(tc)
	ItShouldRejectWrongCredentialsVictoriaLogsRequestInShoot(tc)

	ItShouldWaitForLogsCountByKeyToBeInShootVictoriaLogs(tc, "k8s.pod.name", gardenerLoggerName+".*", shootLogsCount)
	ItShouldWaitForLogsByKeyNotToBeInShootVictoriaLogs(tc, "k8s.pod.name", nonGardenerLoggerName+".*")

	ItShouldWaitForLogsByKeyToBeInShootVictoriaLogs(tc, "unit", "containerd.service")
	ItShouldWaitForLogsByKeyToBeInShootVictoriaLogs(tc, "unit", "kubelet.service")
	ItShouldWaitForLogsByKeyToBeInShootVictoriaLogs(tc, "k8s.pod.name", "event-logger"+".*")
}

// SeedLoggingVl checks the seed-side routing of the shoot logging stack of VictoriaLogs.
// It deploys two logger applications. One in the shoot's control-plane namespace on the
// seed, and another in the seed's garden namespace. Both loggers are asserted to appear
// only in their respective VictoriaLogs instance and not in the other.
// Garden VictoriaLogs ingress auth is also verified.
//
// Function is exposed so that it can be called from the context of another e2e test.
// This is done so that we can minimize the number of shoot clusters that we create.
func SeedLoggingVl(tc *ShootContext) {
	itShouldSetUpShootAndSeedContext(tc)

	ItShouldHaveShootVictoriaLogsReady(tc)
	ItShouldHaveGardenVictoriaLogsReady(tc.SeedContext)

	controlPlaneLoggerName := "seed-cp-logger" + "-" + utilrand.String(randomLength)
	gardenNamespaceLoggerName := "seed-garden-ns-logger" + "-" + utilrand.String(randomLength)

	ItShouldRenderAndDeployTemplateToShootControlPlane(tc, loggerAppName, nonGardenerLoggerParams(controlPlaneLoggerName))
	ItShouldWaitForPodsInShootControlPlaneToBeReady(tc, labels.SelectorFromSet(map[string]string{"app": controlPlaneLoggerName}))

	seed.ItShouldRenderAndDeployTemplateToSeed(tc.SeedContext, "garden", loggerAppName, nonGardenerLoggerParams(gardenNamespaceLoggerName))
	seed.ItShouldWaitForPodsInGardenNamespaceToBeReady(tc.SeedContext, labels.SelectorFromSet(map[string]string{"app": gardenNamespaceLoggerName}))

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
func ShootLogShipper(tc *ShootContext) {
	itShouldSetUpShootAndSeedContext(tc)

	ItShouldHaveShootVictoriaLogsReady(tc)

	brokenLoggerName := "broken-logger" + "-" + utilrand.String(randomLength)
	recoveredLoggerName := "recovered-logger" + "-" + utilrand.String(randomLength)

	// The shoot-side token secret has a 720h lifespan; GRM reconciles at 80%.
	// Patching it directly is safe - no restoration will happen during the test.
	ItShouldBreakOpenTelemetryCollectorToken(tc)
	ItShouldRestartGNA(tc)
	ItShouldWaitForInvalidTokenOnNodes(tc)
	ItShouldRestartOpenTelemetryCollectorOnNodes(tc)

	ItShouldRenderAndDeployTemplateToShootDataPlane(tc, "kube-system", loggerAppName, gardenerLoggerParams(brokenLoggerName))
	ItShouldWaitForPodsInShootToBeReady(tc, "kube-system", labels.SelectorFromSet(map[string]string{"app": brokenLoggerName}))
	ItShouldWaitForLogsByKeyNotToBeInShootVictoriaLogs(tc, "k8s.pod.name", brokenLoggerName+".*")

	ItShouldRestoreOpenTelemetryCollectorToken(tc)
	ItShouldRestartGNA(tc)
	ItShouldWaitForValidTokenOnNodes(tc)

	ItShouldRenderAndDeployTemplateToShootDataPlane(tc, "kube-system", loggerAppName, gardenerLoggerParams(recoveredLoggerName))
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

func gardenerLoggerParams(name string) map[string]any {
	return map[string]any{
		"LoggerName": name,

		// Will be set by the function where this is called due to how Ginkgo works
		// The empty string is to avoid copying the whole map
		"HelmDeployNamespace": "",
		"AppLabel":            name,
		"LogsCount":           shootLogsCount,
		"LogsDuration":        "5s",
		"AdditionalLabels": map[string]string{
			"origin":                              "gardener",
			"gardener.cloud/role":                 "system-component",
			"resources.gardener.cloud/managed-by": "gardener",
		},
	}
}

func nonGardenerLoggerParams(name string) map[string]any {
	return map[string]any{
		"LoggerName": name,

		// Will be set by the function where this is called due to how Ginkgo works
		// The empty string is to avoid copying the whole map
		"HelmDeployNamespace": "",
		"AppLabel":            name,
		"LogsCount":           shootLogsCount,
		"LogsDuration":        "5s",
	}
}
