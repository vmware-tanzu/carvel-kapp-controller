// Copyright 2021 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"github.com/go-logr/logr"
	"github.com/vmware-tanzu/carvel-kapp-controller/pkg/reftracker"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type SecretHandler struct {
	log             logr.Logger
	appRefTacker    *reftracker.AppRefTracker
	appUpdateStatus *reftracker.AppUpdateStatus
}

var _ handler.EventHandler = &SecretHandler{}

func NewSecretHandler(log logr.Logger, as *reftracker.AppRefTracker, aus *reftracker.AppUpdateStatus) *SecretHandler {
	return &SecretHandler{log, as, aus}
}

func (sch *SecretHandler) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	sch.enqueueAppsForUpdate(evt.Meta.GetName(), evt.Meta.GetNamespace(), q)
}

func (sch *SecretHandler) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	sch.enqueueAppsForUpdate(evt.MetaNew.GetName(), evt.MetaNew.GetNamespace(), q)
}

func (sch *SecretHandler) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	sch.enqueueAppsForUpdate(evt.Meta.GetName(), evt.Meta.GetNamespace(), q)
	sch.appRefTacker.RemoveRef(reftracker.NewRefKey("secret", evt.Meta.GetName(), evt.Meta.GetNamespace()))
}

func (sch *SecretHandler) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {}

func (sch *SecretHandler) enqueueAppsForUpdate(secretName, secretNamespace string, q workqueue.RateLimitingInterface) error {
	apps, err := sch.appRefTacker.AppsForRef(reftracker.NewRefKey("secret", secretName, secretNamespace))
	if err != nil {
		return err
	}

	for appKey := range apps {
		sch.log.Info("enqueueing App " + appKey.Description() + " from update to secret " + secretName)
		sch.appUpdateStatus.MarkNeedsUpdate(appKey)
		q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
			Name:      appKey.RefName(),
			Namespace: secretNamespace,
		}})
	}

	return nil
}
