/*
Copyright 2026 Yamaha Motor eBike Systems GmbH.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	postgresv1 "github.com/brose-ebike/postgres-operator/api/v1"
	"github.com/brose-ebike/postgres-operator/internal/controller"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(postgresv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var backupWorkerImage string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8443", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&backupWorkerImage, "backup-worker-image", "",
		"Container image reference used for PgBackupPolicy dump/cleanup CronJobs. "+
			"If unset, resolved automatically from this Pod's own \"manager\" container "+
			"image via the POD_NAME/POD_NAMESPACE downward-API env vars - set this flag "+
			"explicitly only for local development outside a Pod.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Disable HTTP/2 on the metrics server's TLS listener to mitigate the
	// HTTP/2 Stream Cancellation and Rapid Reset CVEs (GHSA-qppj-fm5r-hxr3,
	// GHSA-4374-p667-p6c8).
	disableHTTP2 := func(c *tls.Config) {
		c.NextProtos = []string{"http/1.1"}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:    metricsAddr,
			SecureServing:  true,
			FilterProvider: filters.WithAuthenticationAndAuthorization,
			TLSOpts:        []func(*tls.Config){disableHTTP2},
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "d8580bd9.postgres.oebc.tools",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controller.PgInstanceReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PgInstance")
		os.Exit(1)
	}
	if err = (&controller.PgDatabaseReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PgDatabase")
		os.Exit(1)
	}
	if err = (&controller.PgUserReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PgUser")
		os.Exit(1)
	}

	resolvedBackupWorkerImage := resolveBackupWorkerImage(context.Background(), mgr.GetAPIReader(), backupWorkerImage)
	if resolvedBackupWorkerImage == "" {
		setupLog.Info("backup worker image not configured - PgBackupPolicy reconciliation will report " +
			"not-ready until --backup-worker-image is set or this Pod's own image can be resolved " +
			"via POD_NAME/POD_NAMESPACE")
	}
	if err = (&controller.PgBackupPolicyReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Image:  resolvedBackupWorkerImage,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PgBackupPolicy")
		os.Exit(1)
	}
	if err = (&controller.PgBackupInstanceReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PgBackupInstance")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// resolveBackupWorkerImage decides which image reference PgBackupPolicy's
// CronJobs should use for the backup-worker container. An explicit flag
// value always wins (useful for local development outside a Pod, where
// POD_NAME/POD_NAMESPACE aren't set). Otherwise it reads this Pod's own
// "manager" container image via the Kubernetes API - necessarily the exact
// image this process is running as, including the /backup-worker binary
// built into the same multi-stage image - rather than relying on any
// kustomize manifest-level string substitution, which only rewrites a
// container's own image: field, never arbitrary env/arg strings. Returns
// "" if neither resolves; callers must not silently create CronJobs with
// an empty image.
func resolveBackupWorkerImage(ctx context.Context, r client.Reader, flagValue string) string {
	if flagValue != "" {
		return flagValue
	}

	podName := os.Getenv("POD_NAME")
	podNamespace := os.Getenv("POD_NAMESPACE")
	if podName == "" || podNamespace == "" {
		return ""
	}

	var pod coreV1.Pod
	if err := r.Get(ctx, types.NamespacedName{Namespace: podNamespace, Name: podName}, &pod); err != nil {
		setupLog.Error(err, "unable to resolve own Pod for backup-worker image self-lookup", "pod", podNamespace+"/"+podName)
		return ""
	}
	for _, container := range pod.Spec.Containers {
		if container.Name == "manager" {
			return container.Image
		}
	}
	setupLog.Info("own Pod has no container named \"manager\"; cannot resolve backup-worker image", "pod", podNamespace+"/"+podName)
	return ""
}
