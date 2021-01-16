// Copyright 2020 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package installedpkg

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/vmware-tanzu/carvel-kapp-controller/pkg/apis/kappctrl/v1alpha1"
	kcv1alpha1 "github.com/vmware-tanzu/carvel-kapp-controller/pkg/apis/kappctrl/v1alpha1"
	kcclient "github.com/vmware-tanzu/carvel-kapp-controller/pkg/client/clientset/versioned"
	"github.com/vmware-tanzu/carvel-vendir/pkg/vendir/versions"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type InstalledPackageCR struct {
	model *kcv1alpha1.InstalledPkg

	log    logr.Logger
	client kcclient.Interface
}

func NewInstalledPkgCR(model *kcv1alpha1.InstalledPkg, log logr.Logger,
	client kcclient.Interface) *InstalledPackageCR {

	return &InstalledPackageCR{model: model, log: log, client: client}
}

func (ip *InstalledPackageCR) Reconcile() (reconcile.Result, error) {
	ip.log.Info(fmt.Sprintf("Reconciling InstalledPkg '%s/%s'", ip.model.Namespace, ip.model.Name))

	if ip.model.DeletionTimestamp != nil {
		return reconcile.Result{}, nil // Nothing to do
	}

	pkg, err := ip.referencedPkg()
	if err != nil {
		return reconcile.Result{Requeue: true}, err
	}

	existingApp, err := ip.client.KappctrlV1alpha1().Apps(ip.model.Namespace).Get(ip.model.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return ip.createAppFromPackage(pkg)
		}
		return reconcile.Result{Requeue: true}, err
	}

	return ip.reconcileAppWithPackage(existingApp, pkg)
}

func (ip *InstalledPackageCR) createAppFromPackage(pkg kcv1alpha1.Pkg) (reconcile.Result, error) {
	desiredApp, err := NewApp(&v1alpha1.App{}, ip.model, pkg)
	if err != nil {
		return reconcile.Result{Requeue: true}, err
	}

	_, err = ip.client.KappctrlV1alpha1().Apps(desiredApp.Namespace).Create(desiredApp)
	if err != nil {
		return reconcile.Result{Requeue: true}, err
	}

	return reconcile.Result{}, nil
}

func (ip *InstalledPackageCR) reconcileAppWithPackage(existingApp *kcv1alpha1.App, pkg kcv1alpha1.Pkg) (reconcile.Result, error) {
	desiredApp, err := NewApp(existingApp, ip.model, pkg)
	if err != nil {
		return reconcile.Result{Requeue: true}, err
	}

	if !equality.Semantic.DeepEqual(desiredApp, existingApp) {
		_, err = ip.client.KappctrlV1alpha1().Apps(desiredApp.Namespace).Update(desiredApp)
		if err != nil {
			return reconcile.Result{Requeue: true}, err
		}
	}

	return reconcile.Result{}, nil
}

func (ip *InstalledPackageCR) referencedPkg() (kcv1alpha1.Pkg, error) {
	var semverConfig *versions.VersionSelectionSemver

	switch {
	case ip.model.Spec.PkgRef.Version != "" && ip.model.Spec.PkgRef.VersionSelection != nil:
		return v1alpha1.Pkg{}, fmt.Errorf("Cannot use 'version' with 'versionSelection'")
	case ip.model.Spec.PkgRef.Version != "":
		semverConfig = &versions.VersionSelectionSemver{Constraints: ip.model.Spec.PkgRef.Version}
	case ip.model.Spec.PkgRef.VersionSelection != nil:
		semverConfig = ip.model.Spec.PkgRef.VersionSelection
	}

	pkgList, err := ip.client.KappctrlV1alpha1().Pkgs().List(metav1.ListOptions{})
	if err != nil {
		return kcv1alpha1.Pkg{}, err
	}

	var versionStrs []string
	versionToPkg := map[string]kcv1alpha1.Pkg{}

	for _, pkg := range pkgList.Items {
		versionStrs = append(versionStrs, pkg.Spec.Version)
		versionToPkg[pkg.Spec.Version] = pkg
	}

	verConfig := versions.VersionSelection{Semver: semverConfig}

	selectedVersion, err := versions.HighestConstrainedVersion(versionStrs, verConfig)
	if err != nil {
		return kcv1alpha1.Pkg{}, err
	}

	if pkg, found := versionToPkg[selectedVersion]; found {
		return pkg, nil
	}

	return kcv1alpha1.Pkg{}, fmt.Errorf("Could not find package with name '%s' and version '%s'",
		ip.model.Spec.PkgRef.PublicName, selectedVersion)
}
