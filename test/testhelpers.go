package test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/pivotal/kpack/pkg/logs"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func eventually(t *testing.T, fun func() bool, interval time.Duration, duration time.Duration) {
	t.Helper()
	endTime := time.Now().Add(duration)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for currentTime := range ticker.C {
		if endTime.Before(currentTime) {
			t.Fatal("time is up")
		}
		if fun() {
			return
		}
	}
}

func printObject(t *testing.T, obj interface{}) {
	data, err := yaml.Marshal(obj)
	require.NoError(t, err)

	t.Log(string(data))
}

func dumpK8s(t *testing.T, ctx context.Context, clients *clients, namespace string) {
	const header = "=================%v=================\n"
	t.Logf(header, "ClusterLifecycles")
	clusterLifecycles, err := clients.client.KpackV1alpha2().ClusterLifecycles().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	for _, cl := range clusterLifecycles.Items {
		printObject(t, cl)
	}

	t.Logf(header, "ClusterBuilders")
	clusterBuilders, err := clients.client.KpackV1alpha2().ClusterBuilders().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	for _, cb := range clusterBuilders.Items {
		printObject(t, cb)
	}

	t.Logf(header, "Builders")
	builders, err := clients.client.KpackV1alpha2().Builders(namespace).List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	for _, b := range builders.Items {
		printObject(t, b)
	}

	t.Logf(header, "Images")
	images, err := clients.client.KpackV1alpha2().Images(namespace).List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	for _, i := range images.Items {
		printObject(t, i)
	}

	t.Logf(header, "SourceResolvers")
	sourceResolvers, err := clients.client.KpackV1alpha2().SourceResolvers(namespace).List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	for _, sr := range sourceResolvers.Items {
		printObject(t, sr)
	}

	t.Logf(header, "Pods")
	pods, err := clients.k8sClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	for _, p := range pods.Items {
		printObject(t, p)
	}

	t.Logf(header, "Image logs")
	for _, i := range images.Items {
		err = logs.NewBuildLogsClient(clients.k8sClient).GetImageLogs(ctx, os.Stdout, i.Name, i.Namespace)
		require.NoError(t, err)
	}
}
