/*
Copyright 2022 the original author or authors.

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
	"flag"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	servicebindingv1alpha3 "github.com/servicebinding/runtime/apis/v1alpha3"
	servicebindingv1beta1 "github.com/servicebinding/runtime/apis/v1beta1"
	"github.com/servicebinding/runtime/controllers"
	"github.com/servicebinding/runtime/lifecycle"
	"github.com/servicebinding/runtime/lifecycle/vmware"
	"github.com/servicebinding/runtime/rbac"
	//+kubebuilder:scaffold:imports
)

var (
	scheme     = runtime.NewScheme()
	setupLog   = ctrl.Log.WithName("setup")
	syncPeriod = 10 * time.Hour
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(servicebindingv1alpha3.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var migrateFromVMware bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&migrateFromVMware, "migrate-from-vmware", false,
		"Enable migration from the VMware implementation.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: metricsAddr,
		},
		WebhookServer: &webhook.DefaultServer{
			Options: webhook.Options{
				Port: 9443,
			},
		},
		Cache: cache.Options{
			SyncPeriod: &syncPeriod,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "a359ffaf.servicebinding.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()
	config := reconcilers.NewConfig(mgr, &servicebindingv1beta1.ServiceBinding{}, syncPeriod)
	accessChecker := rbac.NewAccessChecker(config, 5*time.Minute)

	hooks := lifecycle.ServiceBindingHooks{}
	if migrateFromVMware {
		setupLog.Info("Enabling VMware migration hooks.")
		setupLog.Info("Use migration hooks only while migrating implementations. Leaving migration hooks on permanently incurs a performance penalty.")
		hooks = vmware.InstallMigrationHooks(hooks)
	}
	// add additional migration hooks here

	serviceBindingController, err := controllers.ServiceBindingReconciler(
		config,
		hooks,
	).SetupWithManagerYieldingController(ctx, mgr)
	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ServiceBinding")
		os.Exit(1)
	}
	if err = (&servicebindingv1beta1.ServiceBinding{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ServiceBinding v1beta1")
		os.Exit(1)
	}
	if err = (&servicebindingv1alpha3.ServiceBinding{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ServiceBinding v1alpha3")
		os.Exit(1)
	}
	if err = (&servicebindingv1beta1.ClusterWorkloadResourceMapping{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ClusterWorkloadResourceMapping v1beta1")
		os.Exit(1)
	}
	if err = (&servicebindingv1alpha3.ClusterWorkloadResourceMapping{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ClusterWorkloadResourceMapping v1alpha3")
		os.Exit(1)
	}

	if err = controllers.AdmissionProjectorReconciler(
		config,
		// TODO inject from env
		"servicebinding-admission-projector",
		accessChecker.WithVerb("update"),
	).SetupWithManager(ctx, mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "AdmissionProjector")
		os.Exit(1)
	}
	mgr.GetWebhookServer().Register("/interceptor", controllers.AdmissionProjectorWebhook(config, hooks).Build())

	if err = controllers.TriggerReconciler(
		config,
		// TODO inject from env
		"servicebinding-trigger",
		accessChecker.WithVerb("get"),
	).SetupWithManager(ctx, mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Trigger")
		os.Exit(1)
	}
	mgr.GetWebhookServer().Register("/trigger", controllers.TriggerWebhook(config, serviceBindingController).Build())

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
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
