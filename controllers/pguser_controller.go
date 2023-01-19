/*
Copyright 2023 Brose Fahrzeugteile SE & Co. KG, Bamberg.

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

package controllers

import (
	"context"
	"errors"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiV1 "github.com/brose-ebike/postgres-controller/api/v1"
	"github.com/brose-ebike/postgres-controller/pkg/pgapi"
)

// PgUserReconciler reconciles a PgUser object
type PgUserReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	PgRoleAPIFactory
}

//+kubebuilder:rbac:groups=postgres.brose.bike,resources=pgusers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=postgres.brose.bike,resources=pgusers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=postgres.brose.bike,resources=pgusers/finalizers,verbs=update
//+kubebuilder:rbac:groups=,resources=secrets,verbs=get
//+kubebuilder:rbac:groups=,resources=configmaps,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PgUser object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *PgUserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var user apiV1.PgUser
	exists, err := getResource(ctx, r, req.NamespacedName, &user)
	if err != nil {
		logger.Error(err, "Unable to fetch PgDatabase", "database", req.NamespacedName.String())
		return ctrl.Result{}, err
	}
	// Handle deleted
	if !exists {
		logger.Info("Deleted PgDatabase", "database", req.NamespacedName.String())
		return ctrl.Result{}, nil
	}

	// Create PgServerApi from instance
	pgApi, err := r.createPgApi(ctx, &user)
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// Handle finalizing
	if user.DeletionTimestamp != nil {
		if err := r.finalize(ctx, &user, pgApi); err != nil {
			logger.Info("Unable to finalize", "database", req.NamespacedName.String(), "instance", user.GetInstanceIdString())
			return ctrl.Result{RequeueAfter: time.Minute}, err
		}
		// Exit and do not reconcile anymore
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PgUserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiV1.PgUser{}).
		Complete(r)
}

func (r *PgUserReconciler) createPgApi(ctx context.Context, user *apiV1.PgUser) (pgapi.PgRoleAPI, error) {
	logger := log.FromContext(ctx)

	// Fetch Instance
	instanceId := user.GetInstanceId()
	var instance apiV1.PgInstance
	exists, err := getResource(ctx, r, instanceId, &instance)
	if !exists || err != nil {
		logger.Error(err, "Unable to fetch PgInstance", "instance", instanceId.String())
		return nil, err
	}

	// Connect to Instance
	pgApi, err := r.PgRoleAPIFactory(ctx, r, &instance)
	if err != nil {
		logger.Error(err, "Unable to connect", "instance", instance.Namespace+"/"+instance.Name)
		// Update connection status
		if err := setCondition(ctx, r.Status(), user, apiV1.PgConnectedConditionType, false, apiV1.PgConnectedConditionReasonConFailed, err.Error()); err != nil {
			logger.Error(err, "Unable to update condition", "instance", instance.Namespace+"/"+instance.Name)
			return nil, err
		}
		return nil, err
	}

	// Update connection status
	if err := setCondition(ctx, r.Status(), user, apiV1.PgConnectedConditionType, true, apiV1.PgConnectedConditionReasonConSucceeded, "-"); err != nil {
		logger.Error(err, "Unable to update condition", "instance", instance.Namespace+"/"+instance.Name)
		return nil, err
	}
	return pgApi, nil
}

func (r *PgUserReconciler) finalize(ctx context.Context, database *apiV1.PgUser, pgApi pgapi.PgRoleAPI) error {
	logger := log.FromContext(ctx)

	if &logger != nil {
		return errors.New("not implemented")
	}
	return nil
}
