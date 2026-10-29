// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/onsi/gomega"
	istiov1 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

const (
	valiLogging = "vali"
	valiPort    = 3100
)

var (
	errNoRunningPodsFound = errors.New("no running pods were found")
)

// SearchResponse represents the response from a search query to vali
type SearchResponse struct {
	Data struct {
		Result []struct {
			Value []any `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// SearchResponseVL represents a custom responce for the logsql query
type statsCountResponseVL struct {
	Total string `json:"total"`
}

// GetPodsByLabels fetches all pods with the desired set of labels <labelsMap>
func GetPodsByLabels(ctx context.Context, labelsSelector labels.Selector, c kubernetes.Interface, namespace string) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	err := c.Client().List(ctx, podList,
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: labelsSelector})
	if err != nil {
		return nil, err
	}
	return podList, nil
}

// GetFirstRunningPodWithLabels fetches the first running pod with the desired set of labels <labelsMap>
func GetFirstRunningPodWithLabels(ctx context.Context, labelsMap labels.Selector, namespace string, client kubernetes.Interface) (*corev1.Pod, error) {
	var (
		podList *corev1.PodList
		err     error
	)
	podList, err = GetPodsByLabels(ctx, labelsMap, client, namespace)
	if err != nil {
		return nil, err
	}
	if len(podList.Items) == 0 {
		return nil, errNoRunningPodsFound
	}

	for _, pod := range podList.Items {
		if health.IsPodReady(&pod) {
			return &pod, nil
		}
	}

	return nil, errNoRunningPodsFound
}

// PodExecByLabel executes a command inside pods filtered by label
func PodExecByLabel(ctx context.Context, client kubernetes.Interface, namespace string, podLabels labels.Selector, podContainer string, command ...string) (io.Reader, io.Reader, error) {
	pod, err := GetFirstRunningPodWithLabels(ctx, podLabels, namespace, client)
	if err != nil {
		return nil, nil, err
	}

	return client.PodExecutor().Execute(ctx, pod.Namespace, pod.Name, podContainer, command...)
}

// GetLogCountFromSearchResponse extracts the log count from the search response.
func GetLogCountFromSearchResponse(search *SearchResponse) (int, error) {
	var actual int
	for _, result := range search.Data.Result {
		if len(result.Value) < 2 {
			return 0, fmt.Errorf("Result.Value expects at least 2 elements, but has %d", len(result.Value))
		}

		currentStr, ok := result.Value[1].(string)
		if !ok {
			return 0, errors.New("Data.Result.Value[1] is not a string")
		}
		current, err := strconv.Atoi(currentStr)
		if err != nil {
			return 0, errors.New("Data.Result.Value[1] string is not parsable to integer")
		}
		actual += current
	}
	return actual, nil
}

// GetValiLogs gets logs from the last 1 hour for <key>, <value> from the vali instance in <valiNamespace>
func GetValiLogs(ctx context.Context, valiLabels map[string]string, valiNamespace, key, value string, client kubernetes.Interface) (*SearchResponse, error) {
	valiLabelsSelector := labels.SelectorFromSet(valiLabels)

	query := fmt.Sprintf("query=count_over_time({%s=~\"%s\"}[1h])", key, value)

	var stdout io.Reader
	var err error

	// Retry potentially multiple times as vali may be evicted by VPA.
	Eventually(ctx, func() error {
		stdout, _, err = PodExecByLabel(ctx, client, valiNamespace, valiLabelsSelector, valiLogging,
			"wget", "http://localhost:"+strconv.Itoa(valiPort)+"/vali/api/v1/query", "-O-", "--post-data="+query,
		)
		return err
	}, 45*time.Second, 1*time.Second).Should(Succeed())

	search := &SearchResponse{}

	if err := json.NewDecoder(stdout).Decode(search); err != nil {
		return nil, err
	}

	return search, nil
}

// victoriaLogsConn holds the resolved connection parameters for a VictoriaLogs ingress endpoint.
type victoriaLogsConn struct {
	host       string
	httpClient *http.Client
	username   string
	password   string
}

// resolveVictoriaLogsConn resolves the ingress host, CA pool, and credentials for the given VirtualService.
func resolveVictoriaLogsConn(ctx context.Context, namespace, vsName string, k8sClient kubernetes.Interface) (*victoriaLogsConn, error) {
	vs := &istiov1.VirtualService{}
	if err := k8sClient.Client().Get(ctx, types.NamespacedName{Name: vsName, Namespace: namespace}, vs); err != nil {
		return nil, fmt.Errorf("failed to get %s VirtualService: %w", vsName, err)
	}
	if len(vs.Spec.Hosts) == 0 {
		return nil, fmt.Errorf("%s VirtualService has no hosts", vsName)
	}

	authSecretName := vs.Labels[v1beta1constants.LabelBasicAuthSecretName]
	if authSecretName == "" {
		return nil, fmt.Errorf("%s VirtualService has no %s label", vsName, v1beta1constants.LabelBasicAuthSecretName)
	}

	username, password, err := victoriaLogsIngressCredentials(ctx, namespace, authSecretName, k8sClient)
	if err != nil {
		return nil, err
	}

	caPool, err := victoriaLogsIngressCAPool(ctx, namespace, caSecretNameForAuthSecret(authSecretName), k8sClient)
	if err != nil {
		return nil, err
	}

	return &victoriaLogsConn{
		host:       vs.Spec.Hosts[0],
		httpClient: &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: caPool}}},
		username:   username,
		password:   password,
	}, nil
}

// GetVictoriaLogs gets logs from the last <minutes> minutes for <key>=~<value> from a VictoriaLogs
// instance in <namespace>, using <vsName> as the VirtualService name to resolve the ingress host.
// The auth secret name and CA secret name are read from the VirtualService labels.
func GetVictoriaLogs(ctx context.Context, minutes int, namespace, vsName, key, value string, k8sClient kubernetes.Interface) (int, error) {
	conn, err := resolveVictoriaLogsConn(ctx, namespace, vsName, k8sClient)
	if err != nil {
		return 0, err
	}

	params := url.Values{}
	params.Set("query", fmt.Sprintf("%s:~\"%s\" _time:%dm | stats count() as total", key, value, minutes))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://"+conn.host+"/select/logsql/query",
		strings.NewReader(params.Encode()),
	)
	if err != nil {
		return 0, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(conn.username, conn.password)
	resp, err := conn.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return decodeVictoriaLogsNDJSON(body)
}

// QueryVictoriaLogsStatus resolves the ingress host and CA pool for <vsName> in <namespace>, sends
// a single query request with the given credentials (pass empty strings for no auth), and returns
// the HTTP status code.
func QueryVictoriaLogsStatus(ctx context.Context, namespace, vsName, username, password string, k8sClient kubernetes.Interface) (int, error) {
	conn, err := resolveVictoriaLogsConn(ctx, namespace, vsName, k8sClient)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://"+conn.host+"/select/logsql/query",
		strings.NewReader("query=*&start=now-1m&limit=1"),
	)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}
	resp, err := conn.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

// caSecretNameForAuthSecret returns the secrets-manager name label for the CA secret that corresponds to the given auth secret.
func caSecretNameForAuthSecret(authSecretName string) string {
	switch {
	case authSecretName == v1beta1constants.SecretNameObservabilityIngress:
		return operatorv1alpha1.SecretNameCARuntime
	case strings.HasPrefix(authSecretName, "seed-"):
		return v1beta1constants.SecretNameCASeed
	default:
		return v1beta1constants.SecretNameCACluster
	}
}

// victoriaLogsIngressCredentials returns the username and password from the named auth secret.
func victoriaLogsIngressCredentials(ctx context.Context, namespace, authSecretName string, k8sClient kubernetes.Interface) (username, password string, err error) {
	secretList := &corev1.SecretList{}
	if err := k8sClient.Client().List(ctx, secretList,
		client.InNamespace(namespace),
		client.MatchingLabels{"name": authSecretName},
	); err != nil {
		return "", "", fmt.Errorf("failed to list %s secrets: %w", authSecretName, err)
	}
	if len(secretList.Items) == 0 {
		return "", "", fmt.Errorf("%s secret not found", authSecretName)
	}
	secret := secretList.Items[0]
	return string(secret.Data["username"]), string(secret.Data["password"]), nil
}

// victoriaLogsIngressCAPool returns an x509 cert pool loaded from the named CA secret.
func victoriaLogsIngressCAPool(ctx context.Context, namespace, caSecretName string, k8sClient kubernetes.Interface) (*x509.CertPool, error) {
	secretList := &corev1.SecretList{}
	if err := k8sClient.Client().List(ctx, secretList,
		client.InNamespace(namespace),
		client.MatchingLabels{"name": caSecretName},
	); err != nil {
		return nil, fmt.Errorf("failed to list %s secrets: %w", caSecretName, err)
	}
	if len(secretList.Items) == 0 {
		return nil, fmt.Errorf("%s secret not found", caSecretName)
	}
	caCert := secretList.Items[0].Data["ca.crt"]
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, errors.New("failed to parse CA certificate")
	}
	return pool, nil
}

// decodeVictoriaLogsNDJSON decodes a newline-delimited JSON response body into a slice of SearchResponseVL entries.
func decodeVictoriaLogsNDJSON(body []byte) (int, error) {
	var entry statsCountResponseVL

	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&entry); err != nil {
		return 0, err
	}

	return strconv.Atoi(entry.Total)
}
