// Copyright 2020 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"fmt"
	"net/http"         // Pprof related
	_ "net/http/pprof" // Pprof related
	"os"
	"time"

	"github.com/go-logr/logr"
	kcv1alpha1 "github.com/vmware-tanzu/carvel-kapp-controller/pkg/apis/kappctrl/v1alpha1"
	kcclient "github.com/vmware-tanzu/carvel-kapp-controller/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	PprofListenAddr = "0.0.0.0:6060"
)

type Options struct {
	Concurrency       int
	Namespace         string
	EnablePprof       bool
	APIRequestTimeout time.Duration
}

// Based on https://github.com/kubernetes-sigs/controller-runtime/blob/8f633b179e1c704a6e40440b528252f147a3362a/examples/builtins/main.go
func Run(opts Options, runLog logr.Logger) {
	runLog.Info("start controller")
	runLog.Info("setting up manager")

	restConfig := config.GetConfigOrDie()

	if opts.APIRequestTimeout != 0 {
		restConfig.Timeout = opts.APIRequestTimeout
	}

	mgr, err := manager.New(restConfig, manager.Options{Namespace: opts.Namespace})
	if err != nil {
		runLog.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	logProxies(runLog)

	runLog.Info("setting up controller")

	coreClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		runLog.Error(err, "building core client")
		os.Exit(1)
	}

	kcClient, err := kcclient.NewForConfig(restConfig)
	if err != nil {
		runLog.Error(err, "building app client")
		os.Exit(1)
	}

	appFactory := AppFactory{
		coreClient: coreClient,
		appClient:  kcClient,
	}

	{ // add controller for apps
		ctrlAppOpts := controller.Options{
			Reconciler: NewUniqueReconciler(&ErrReconciler{
				delegate: &AppsReconciler{
					appClient:  kcClient,
					appFactory: appFactory,
					log:        runLog.WithName("ar"),
				},
				log: runLog.WithName("pr"),
			}),
			MaxConcurrentReconciles: opts.Concurrency,
		}

		ctrlApp, err := controller.New("kapp-controller-app", mgr, ctrlAppOpts)
		if err != nil {
			runLog.Error(err, "unable to set up kapp-controller-app")
			os.Exit(1)
		}

		err = ctrlApp.Watch(&source.Kind{Type: &kcv1alpha1.App{}}, &handler.EnqueueRequestForObject{})
		if err != nil {
			runLog.Error(err, "unable to watch *kcv1alpha1.App")
			os.Exit(1)
		}
	}

	{ // add controller for installedPkgs
		installedPkgsCtrlOpts := controller.Options{
			Reconciler: &InstalledPkgReconciler{
				intalledPkgClient: kcClient,
				log:               runLog.WithName("ipr"),
			},
			MaxConcurrentReconciles: opts.Concurrency,
		}

		installedPkgCtrl, err := controller.New("kapp-controller-installed-pkg", mgr, installedPkgsCtrlOpts)
		if err != nil {
			runLog.Error(err, "unable to set up kapp-controller-installed-pkg")
			os.Exit(1)
		}

		err = installedPkgCtrl.Watch(&source.Kind{Type: &kcv1alpha1.InstalledPkg{}}, &handler.EnqueueRequestForObject{})
		if err != nil {
			runLog.Error(err, "unable to watch *kcv1alpha1.InstalledPkg")
			os.Exit(1)
		}
	}

	runLog.Info("starting manager")

	if opts.EnablePprof {
		runLog.Info("DANGEROUS in production setting -- pprof running", "listen-addr", PprofListenAddr)
		go func() {
			runLog.Error(http.ListenAndServe(PprofListenAddr, nil), "serving pprof")
		}()
	}

	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		runLog.Error(err, "unable to run manager")
		os.Exit(1)
	}

	runLog.Info("Exiting")
	os.Exit(0)
}

func logProxies(runLog logr.Logger) {
	if proxyVal := os.Getenv("http_proxy"); proxyVal != "" {
		runLog.Info(fmt.Sprintf("Using http proxy '%s'", proxyVal))
	}

	if proxyVal := os.Getenv("https_proxy"); proxyVal != "" {
		runLog.Info(fmt.Sprintf("Using https proxy '%s'", proxyVal))
	}

	if noProxyVal := os.Getenv("no_proxy"); noProxyVal != "" {
		runLog.Info(fmt.Sprintf("No proxy set for: %s", noProxyVal))
	}
}