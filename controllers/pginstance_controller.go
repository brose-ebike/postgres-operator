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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
	"github.com/brose-ebike/postgres-operator/pkg/pgapi"
	"github.com/brose-ebike/postgres-operator/pkg/services"
)

// PgInstanceReconciler reconciles a PgInstance object
type PgInstanceReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	PgConnectionFactory PgConnectionFactory
}

//+kubebuilder:rbac:groups=postgres.brose.bike,resources=pginstances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=postgres.brose.bike,resources=pginstances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=postgres.brose.bike,resources=pginstances/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PgInstance object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *PgInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	logger := log.FromContext(ctx)

	var instance apiV1.PgInstance
	exists, err := getResource(ctx, r, req.NamespacedName, &instance)
	if err != nil {
		logger.Error(err, "Unable to fetch PgInstance", "instance", req.NamespacedName.String())
		return ctrl.Result{}, err
	}
	// Handle deletion
	if !exists {
		logger.Info("Deleted PgInstance", "instance", req.NamespacedName.String())
		return ctrl.Result{}, nil
	}

	// Create PgServerApi from instance
	pgApi, err := r.createPgApi(ctx, &instance)
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// Test Connection explicitly
	if err := pgApi.TestConnection(); err != nil {
		logger.Error(err, "Unable to connect", "instance", instance.Namespace+"/"+instance.Name)
		// Update connection status
		if err := setCondition(ctx, r.Status(), &instance, apiV1.PgConnectedConditionType, false, apiV1.PgConnectedConditionReasonConFailed, err.Error()); err != nil {
			logger.Error(err, "Unable to update condition", "instance", req.NamespacedName.String())
			return ctrl.Result{RequeueAfter: time.Minute}, err
		}
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	logger.Info("Processed instance", "instance", req.NamespacedName.String())

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PgInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Register Factory Method
	r.PgConnectionFactory = func(ctx context.Context, r client.Reader, instance *apiV1.PgInstance) (pgapi.PgConnector, error) {
		return services.NewPgInstanceAPI(ctx, r, instance)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&apiV1.PgInstance{}).
		Complete(r)
}

func (r *PgInstanceReconciler) createPgApi(ctx context.Context, instance *apiV1.PgInstance) (pgapi.PgConnector, error) {
	logger := log.FromContext(ctx)

	// Connect to Instance
	pgApi, err := r.PgConnectionFactory(ctx, r, instance)
	if err != nil {
		logger.Error(err, "Unable to connect", "instance", instance.Namespace+"/"+instance.Name)
		// Update connection status
		if err := setCondition(ctx, r.Status(), instance, apiV1.PgConnectedConditionType, false, apiV1.PgConnectedConditionReasonConFailed, err.Error()); err != nil {
			logger.Error(err, "Unable to update condition", "instance", instance.Namespace+"/"+instance.Name)
			return nil, err
		}
		return nil, err
	}

	// Update connection status
	if err := setCondition(ctx, r.Status(), instance, apiV1.PgConnectedConditionType, true, apiV1.PgConnectedConditionReasonConSucceeded, "-"); err != nil {
		logger.Error(err, "Unable to update condition", "instance", instance.Namespace+"/"+instance.Name)
		return nil, err
	}
	return pgApi, nil
}
