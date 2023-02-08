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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
	"github.com/brose-ebike/postgres-operator/pkg/services"
)

// PgDatabaseReconciler reconciles a PgDatabase object
type PgDatabaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	PgDatabaseAPIFactory
}

//+kubebuilder:rbac:groups=postgres.brose.bike,resources=pgdatabases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=postgres.brose.bike,resources=pgdatabases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=postgres.brose.bike,resources=pgdatabases/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PgDatabase object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *PgDatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	logger := log.FromContext(ctx)

	var database apiV1.PgDatabase
	exists, err := getResource(ctx, r, req.NamespacedName, &database)
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
	pgApi, err := r.createPgApi(ctx, &database)
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// Handle finalizing
	if database.DeletionTimestamp != nil {
		if err := r.finalize(ctx, &database, pgApi); err != nil {
			logger.Info("Unable to finalize", "database", req.NamespacedName.String(), "instance", database.GetInstanceIdString())
			return ctrl.Result{RequeueAfter: time.Minute}, err
		}
		// Exit and do not reconcile anymore
		return ctrl.Result{}, nil
	}

	// Create Database if not exist
	if err := r.createDatabaseIfNotExists(ctx, pgApi, &database); err != nil {
		logger.Error(err, "Unable to create Database", "database", database.Name, "instance", database.GetInstanceIdString())
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// Update Database Exists Condition
	if err := setCondition(ctx, r.Status(), &database, apiV1.PgDatabaseExistsConditionType, true, "DatabaseExists", "-"); err != nil {
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// Install Extensions if missing
	if err := r.handleExtensions(ctx, pgApi, &database); err != nil {
		logger.Error(err, "Unable to create extensions", "database", database.Name, "instance", database.GetInstanceIdString())
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// Update Default Privileges
	if err := r.handleDefaultPrivileges(ctx, pgApi, &database); err != nil {
		logger.Error(err, "Unable to update default privileges", "database", database.Name, "instance", database.GetInstanceIdString())
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// Revoke Public Privileges if needed
	if err := r.handlePublicPrivileges(ctx, pgApi, &database); err != nil {
		logger.Error(err, "Unable to update public privileges", "database", database.ToNamespacedName(), "instance", database.GetInstanceIdString())
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// Drop Public Schema if needed
	if err := r.handlePublicSchema(ctx, pgApi, &database); err != nil {
		logger.Error(err, "Unable to update public schema", "database", database.ToNamespacedName(), "instance", database.GetInstanceIdString())
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// Check if finalizer exists
	if !controllerutil.ContainsFinalizer(&database, apiV1.DefaultFinalizerPgDatabase) {
		controllerutil.AddFinalizer(&database, apiV1.DefaultFinalizerPgDatabase)
		err = r.Update(ctx, &database)
		if err != nil {
			logger.Error(err, "Failed to update finalizers", "database", database.ToNamespacedName(), "instance", database.GetInstanceIdString())
			return ctrl.Result{RequeueAfter: time.Second}, err
		}
	}

	logger.Info("Processed database", "database", database.ToNamespacedName(), "instance", database.GetInstanceIdString())

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PgDatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Register Factory Method
	r.PgDatabaseAPIFactory = func(ctx context.Context, r client.Reader, instance *apiV1.PgInstance) (PgDatabaseAPI, error) {
		return services.NewPgInstanceAPI(ctx, r, instance)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&apiV1.PgDatabase{}).
		Complete(r)
}

func (r *PgDatabaseReconciler) createPgApi(ctx context.Context, database *apiV1.PgDatabase) (PgDatabaseAPI, error) {
	logger := log.FromContext(ctx)

	// Fetch Instance
	instanceId := database.GetInstanceId()
	var instance apiV1.PgInstance
	exists, err := getResource(ctx, r, instanceId, &instance)
	if !exists || err != nil {
		logger.Error(err, "Unable to fetch PgInstance", "instance", instanceId.String())
		return nil, err
	}

	// Connect to Instance
	pgApi, err := r.PgDatabaseAPIFactory(ctx, r, &instance)
	if err != nil {
		logger.Error(err, "Unable to connect", "instance", instanceId)
		// Update connection status
		if err := setCondition(ctx, r.Status(), database, apiV1.PgConnectedConditionType, false, apiV1.PgConnectedConditionReasonConFailed, err.Error()); err != nil {
			logger.Error(err, "Unable to update condition", "database", database.ToNamespacedName())
			return nil, err
		}
		return nil, err
	}

	// Update connection status
	if err := setCondition(ctx, r.Status(), database, apiV1.PgConnectedConditionType, true, apiV1.PgConnectedConditionReasonConSucceeded, "-"); err != nil {
		logger.Error(err, "Unable to update condition", "database", database.ToNamespacedName())
		return nil, err
	}
	return pgApi, nil
}

func (r *PgDatabaseReconciler) finalize(ctx context.Context, database *apiV1.PgDatabase, pgApi PgDatabaseAPI) error {
	logger := log.FromContext(ctx)

	if database.Spec.DeletionBehavior.Drop {
		exists, err := pgApi.IsDatabaseExisting(database.Name)
		if err != nil {
			logger.Error(err, "Unable to query database", "database", database.Name, "instance", database.GetInstanceIdString())
			return err
		}
		if exists {
			if err := pgApi.DeleteDatabase(database.Name); err != nil {
				logger.Error(err, "Unable to remove database", "database", database.Name, "instance", database.GetInstanceIdString())
				return err
			}
		}
		// Update Database Exists Condition
		if err := setCondition(ctx, r.Status(), database, apiV1.PgDatabaseExistsConditionType, false, "DatabaseMissing", "Database was deleted"); err != nil {
			return err
		}
	}
	if database.Spec.DeletionBehavior.Wait {
		exists, err := pgApi.IsDatabaseExisting(database.Name)
		if err != nil {
			logger.Error(err, "Unable to query database", "database", database.Name)
			return err
		}
		if exists {
			logger.Info("Database still exists, waiting for database to be dropped", "database", database.Name)
			return nil
		}
	}
	// Remove finalizer
	controllerutil.RemoveFinalizer(database, apiV1.DefaultFinalizerPgDatabase)
	err := r.Update(ctx, database)
	if err != nil {
		logger.Error(err, "Failed to update finalizers")
		return err
	}
	logger.Info("Removed finalizer, database resource can now be deleted")
	// Exit after finalizer was removed
	return nil
}

func (r *PgDatabaseReconciler) createDatabaseIfNotExists(ctx context.Context, pgApi PgDatabaseAPI, database *apiV1.PgDatabase) error {
	logger := log.FromContext(ctx)
	databaseName := database.Name

	exists, err := pgApi.IsDatabaseExisting(databaseName)
	if err != nil {
		logger.Error(err, "Unable to query database "+databaseName)
		return err
	}

	// create database
	if !exists {
		if err := pgApi.CreateDatabase(databaseName); err != nil {
			logger.Error(err, "Unable to create database "+databaseName)
			return err
		}
		logger.Info("Created database " + databaseName)
	}
	return nil
}

func (r *PgDatabaseReconciler) handleExtensions(ctx context.Context, pgApi PgDatabaseAPI, database *apiV1.PgDatabase) error {
	for _, extension := range database.Spec.Extensions {
		exists, err := pgApi.IsDatabaseExtensionPresent(database.Name, extension)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if err := pgApi.CreateDatabaseExtension(database.Name, extension); err != nil {
			reason := "MissingExtension-" + extension
			message := "The database extension " + extension + " cannot be created\n" + err.Error()
			setCondition(ctx, r.Status(), database, apiV1.PgDatabaseExtensionsConditionType, false, reason, message)
			return err
		}
	}
	// Update Database Extension Exists Condition
	return setCondition(ctx, r.Status(), database, apiV1.PgDatabaseExtensionsConditionType, true, "AllExtensionsArePresent", "-")
}

func (r *PgDatabaseReconciler) handleDefaultPrivileges(ctx context.Context, pgApi PgDatabaseAPI, database *apiV1.PgDatabase) error {
	for _, schema := range database.Spec.DefaultPrivileges {
		// Check schema existence
		exists, err := pgApi.IsSchemaInDatabase(database.Name, schema.Name)
		if err != nil {
			return err
		}
		if !exists {
			return errors.New("Schema " + schema.Name + " does not exist in database " + database.Name)
		}
		// Check schema permissions
		usable, err := pgApi.IsSchemaUsable(database.Name, schema.Name)
		if err != nil {
			return err
		}
		if !usable {
			if err := pgApi.MakeSchemaUseable(database.Name, schema.Name); err != nil {
				return err
			}
		}
		// Update Privileges
		for _, role := range schema.Roles {
			// Update schema privileges
			if err := pgApi.UpdateSchemaPrivileges(database.Name, schema.Name, role, schema.PrivilegesStr()); err != nil {
				return err
			}
			// Update table privileges
			if err := pgApi.UpdateDefaultPrivileges(database.Name, schema.Name, role, "TABLES", schema.TablePrivilegesStr()); err != nil {
				return err
			}
			if err := pgApi.UpdatePrivilegesOnAllObjects(database.Name, schema.Name, role, "TABLES", schema.TablePrivilegesStr()); err != nil {
				return err
			}
			// Update sequence privileges
			if err := pgApi.UpdateDefaultPrivileges(database.Name, schema.Name, role, "SEQUENCES", schema.SequencePrivilegesStr()); err != nil {
				return err
			}
			if err := pgApi.UpdatePrivilegesOnAllObjects(database.Name, schema.Name, role, "SEQUENCES", schema.SequencePrivilegesStr()); err != nil {
				return err
			}
			// Update function privileges
			if err := pgApi.UpdateDefaultPrivileges(database.Name, schema.Name, role, "FUNCTIONS", schema.FunctionPrivilegesStr()); err != nil {
				return err
			}
			if err := pgApi.UpdatePrivilegesOnAllObjects(database.Name, schema.Name, role, "FUNCTIONS", schema.FunctionPrivilegesStr()); err != nil {
				return err
			}
			// Update type privileges
			if err := pgApi.UpdateDefaultPrivileges(database.Name, schema.Name, role, "TYPES", schema.TypePrivilegesStr()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *PgDatabaseReconciler) handlePublicPrivileges(ctx context.Context, pgApi PgDatabaseAPI, database *apiV1.PgDatabase) error {
	// TODO update public privileges if needed
	if !database.Spec.PublicPrivileges.Revoke {
		return nil
	}
	// Revoke all privileges for public on database
	if err := pgApi.UpdateDatabasePrivileges(database.Name, "public", []string{}); err != nil {
		return err
	}

	exists, err := pgApi.IsSchemaInDatabase(database.Name, "public")
	if err != nil {
		return err
	}
	if exists {
		// Revoke all privileges for public on schema
		if err := pgApi.DeleteAllPrivilegesOnSchema(database.Name, "public", "public"); err != nil {
			return err
		}
	}
	return nil
}

func (r *PgDatabaseReconciler) handlePublicSchema(ctx context.Context, pgApi PgDatabaseAPI, database *apiV1.PgDatabase) error {
	if !database.Spec.PublicSchema.Drop {
		return nil
	}
	exists, err := pgApi.IsSchemaInDatabase(database.Name, "public")
	if err != nil {
		return err
	}
	if exists {
		if err := pgApi.DeleteSchema(database.Name, "public"); err != nil {
			return err
		}
	}
	return nil
}
