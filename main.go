/*
Copyright 2022.

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

	githubv1alpha1 "colossyan.com/github-pr-controller/api/v1alpha1"
	"colossyan.com/github-pr-controller/controllers"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	k8s_zap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()        // nolint:gochecknoglobals
	setupLog = ctrl.Log.WithName("setup") // nolint:gochecknoglobals
)

const (
	defaultPort = 9443
)

func init() { // nolint:gochecknoinits
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(githubv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	var repositoryParams controllers.RepositoryReconcilerParameters
	flag.StringVar(&repositoryParams.ReconcilePeriod, "reconcile-period", "1m", "The period to reconcile the repository")
	flag.StringVar(&repositoryParams.DefaultToken, "default-token", "",
		"The default OAuth token to use as default secret when connecting to the Github server")

	opts := k8s_zap.Options{
		Development: true,
		Encoder:     zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(k8s_zap.New(k8s_zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   defaultPort,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "71d0a307.colossyan.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := addControllers(mgr, repositoryParams); err != nil {
		os.Exit(1)
	}

	if err := addHealthChecks(mgr); err != nil {
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func addControllers(mgr manager.Manager, repositoryParameters controllers.RepositoryReconcilerParameters) error {
	if err := (&controllers.PullRequestReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PullRequest")
		return err
	}

	repositoryReconciler, err := controllers.NewRepositoryReconciler(
		mgr.GetClient(), mgr.GetScheme(), repositoryParameters)
	if err != nil {
		return errors.Wrap(err, "failed to create new repository reconciler")
	}
	if err := repositoryReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Repository")
		return err
	}
	//+kubebuilder:scaffold:builder

	return nil
}

func addHealthChecks(mgr manager.Manager) error {
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		return err
	}
	return nil
}
