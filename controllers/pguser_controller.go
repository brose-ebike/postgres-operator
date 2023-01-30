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
	"strconv"
	"time"

	coreV1 "k8s.io/api/core/v1"
	kErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
	"github.com/brose-ebike/postgres-operator/pkg/pgapi"
	"github.com/brose-ebike/postgres-operator/pkg/security"
	"github.com/brose-ebike/postgres-operator/pkg/services"

	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch

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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	logger := log.FromContext(ctx)

	var user apiV1.PgUser
	exists, err := getResource(ctx, r, req.NamespacedName, &user)
	if err != nil {
		logger.Error(err, "Unable to fetch PgUser", "user", req.NamespacedName.String())
		return ctrl.Result{}, err
	}
	// Handle deleted
	if !exists {
		logger.Info("Deleted PgUser", "user", req.NamespacedName.String())
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
			logger.Info("Unable to finalize", "user", req.NamespacedName.String(), "instance", user.GetInstanceIdString())
			return ctrl.Result{RequeueAfter: time.Minute}, err
		}
		// Exit and do not reconcile anymore
		return ctrl.Result{}, nil
	}

	// Handle create / update
	if err := r.createLoginRoleIfNotExists(ctx, pgApi, &user); err != nil {
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}
	// Update Login Role Exists Condition
	if err := setCondition(ctx, r, &user, apiV1.PgUserExistsConditionType, true, "-", "-"); err != nil {
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// create update k8s secret
	password, err := r.createOrUpdateSecret(ctx, pgApi, &user)
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// update login role with password in postgres instance
	if err := pgApi.UpdateUserPassword(user.Name, password); err != nil {
		logger.Error(err, "Unable to update role password for role "+user.Name+" on instance "+user.GetInstanceIdString())
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// update ownership and permissions for databases
	if err := r.updateDatabaseOwnershipAndPrivileges(ctx, pgApi, &user); err != nil {
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// Check if finalizer exists
	if !controllerutil.ContainsFinalizer(&user, apiV1.DefaultFinalizerPgUser) {
		controllerutil.AddFinalizer(&user, apiV1.DefaultFinalizerPgUser)
		err = r.Update(ctx, &user)
		if err != nil {
			return ctrl.Result{RequeueAfter: time.Minute}, err
		}
	}

	logger.Info("Processed user", "user", user.ToNamespacedName(), "instance", user.GetInstanceIdString())

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PgUserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Register Factory Method
	r.PgRoleAPIFactory = func(ctx context.Context, r client.Reader, instance *apiV1.PgInstance) (PgRoleAPI, error) {
		return services.NewPgInstanceAPI(ctx, r, instance)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&apiV1.PgUser{}).
		Complete(r)
}

func (r *PgUserReconciler) createPgApi(ctx context.Context, user *apiV1.PgUser) (PgRoleAPI, error) {
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

func (r *PgUserReconciler) finalize(ctx context.Context, user *apiV1.PgUser, pgApi pgapi.PgRoleAPI) error {
	logger := log.FromContext(ctx)

	if err := pgApi.DeleteRole(user.Name); err != nil {
		logger.Error(err, "Unable to remove login role "+user.Name+" from "+user.GetInstanceIdString())
		return err
	}

	// Update Login Role Exists Condition
	if err := setCondition(ctx, r, user, apiV1.PgUserExistsConditionType, false, "-", "-"); err != nil {
		logger.Error(err, "Unable to update condition")
		return err
	}

	// Delete Secret
	roleSecret := coreV1.Secret{
		ObjectMeta: metaV1.ObjectMeta{
			Namespace: user.Namespace,
			Name:      user.Spec.Secret.Name,
		},
	}
	if err := r.Delete(ctx, &roleSecret); err != nil {
		logger.Error(err, "Unable to delete Secret")
		return err
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(user, apiV1.DefaultFinalizerPgUser)
	err := r.Update(ctx, user)
	if err != nil {
		return err
	}

	// Exit after finalizer was removed
	return nil
}

func (r *PgUserReconciler) createLoginRoleIfNotExists(ctx context.Context, pgApi pgapi.PgRoleAPI, user *apiV1.PgUser) error {
	logger := log.FromContext(ctx)
	roleName := user.Name

	exists, err := pgApi.IsRoleExisting(roleName)
	if err != nil {
		logger.Error(err, "Unable to query login role "+roleName)
		return err
	}

	// create roles
	if !exists {
		if err := pgApi.CreateRole(roleName); err != nil {
			logger.Error(err, "Unable to create login role "+roleName)
			return err
		}
		logger.Info("Created login role " + roleName)
	}
	return nil
}

func (r *PgUserReconciler) createOrUpdateSecret(ctx context.Context, pgApi PgRoleAPI, user *apiV1.PgUser) (string, error) {
	logger := log.FromContext(ctx)
	roleName := user.Name
	password := ""

	var roleSecret coreV1.Secret
	secretKey := types.NamespacedName{
		Namespace: user.Namespace,
		Name:      user.Spec.Secret.Name,
	}
	err := r.Get(ctx, secretKey, &roleSecret)
	if err != nil && !kErrors.IsNotFound(err) {
		logger.Error(err, "Unable to fetch role secret for login role "+roleName)
		return "", err
	} else if err != nil && kErrors.IsNotFound(err) { // Create Secret
		password = security.GeneratePassword()
		roleSecret = coreV1.Secret{
			ObjectMeta: metaV1.ObjectMeta{
				Namespace: secretKey.Namespace,
				Name:      secretKey.Name,
				OwnerReferences: []metaV1.OwnerReference{
					{
						APIVersion:         apiV1.GroupVersion.String(),
						BlockOwnerDeletion: &cTrue,
						Controller:         &cTrue,
						Kind:               apiV1.PgUserKind(),
						Name:               user.Name,
						UID:                user.UID,
					},
				},
			},
			Data: r.generateSecretData(pgApi, user, password),
		}
		if err := r.Create(ctx, &roleSecret); err != nil {
			logger.Error(err, "Unable to create role secret for login role "+roleName)
			return "", err
		}
	} else { // Update Secret
		// Update Owner Reference
		roleSecret.ObjectMeta.OwnerReferences = []metaV1.OwnerReference{
			{
				APIVersion:         apiV1.GroupVersion.String(),
				BlockOwnerDeletion: &cTrue,
				Controller:         &cTrue,
				Kind:               apiV1.PgUserKind(),
				Name:               user.Name,
				UID:                user.UID,
			},
		}
		// Update Data
		password = string(roleSecret.Data["password"])
		roleSecret.Data = r.generateSecretData(pgApi, user, password)
		err = r.Update(ctx, &roleSecret)
		if err != nil {
			logger.Error(err, "Unable to update role secret for login role "+roleName)
			return "", err
		}
	}
	return password, nil
}

func (r *PgUserReconciler) generateSecretData(pgApi PgRoleAPI, user *apiV1.PgUser, password string) map[string][]byte {
	data := map[string]string{}
	connStr := pgApi.ConnectionString()
	portStr := strconv.Itoa(connStr.Port())
	data["host"] = connStr.Hostname()
	data["port"] = portStr
	data["user"] = user.Name
	data["password"] = password
	// Generate Connection Strings for Databases
	for _, database := range user.Spec.Databases {
		data["database."+database.Name+".uri"] = connStr.Hostname() + ":" + portStr + "/" + database.Name + "?sslmode=" + connStr.SSLMode()
		data["database."+database.Name+".connection_string"] = "postgres://" + user.Name + ":" + password + "@" + connStr.Hostname() + ":" + portStr + "/" + database.Name + "?sslmode=" + connStr.SSLMode()
		data["database."+database.Name+".jdbc_connection_string"] = "jdbc:postgresql://" + connStr.Hostname() + ":" + portStr + "/" + database.Name + "?sslmode=" + connStr.SSLMode()
	}
	binaryData := map[string][]byte{}
	for key, element := range data {
		binaryData[key] = []byte(element)
	}
	return binaryData
}

func (r *PgUserReconciler) updateDatabaseOwnershipAndPrivileges(ctx context.Context, pgApi PgRoleAPI, user *apiV1.PgUser) error {
	logger := log.FromContext(ctx)
	for _, database := range user.Spec.Databases {
		exists, err := pgApi.IsDatabaseExisting(database.Name)
		if err != nil {
			logger.Error(err, "Unable to query for the database "+database.Name)
			return err
		}
		if !exists {
			err = errors.New("Database " + database.Name + " does not exists")
			logger.Error(err, "Database "+database.Name+" does not exists")
			return err
		}

		// Update ownership
		currentOwner, err := pgApi.GetDatabaseOwner(database.Name)
		if err != nil {
			logger.Error(err, "Unable to query for the database "+database.Name)
			return err
		}
		// Case 1: Login Role should be owner of database and is currently owner of database  => Do nothing
		// Case 2: Login Role should not be owner of database and is currently not owner of database => Do nothing
		if currentOwner != user.Name && database.Owner { // Case 3: Login Role should be owner of database and is currently not owner of database
			if err := pgApi.UpdateDatabaseOwner(database.Name, user.Name); err != nil {
				logger.Error(err, "Unable to update database owner")
				return err
			}
		} else if currentOwner == user.Name && !database.Owner { // Case 4: Login Role should not be owner of database and is currently owner of database
			// Reset owner on database to admin
			err = pgApi.ResetDatabaseOwner(database.Name)
			if err != nil {
				logger.Error(err, "Unable to reset database owner")
				return err
			}
		}

		// Update database privileges
		if !database.Owner {
			if err := pgApi.UpdateDatabasePrivileges(database.Name, user.Name, database.Privileges); err != nil {
				logger.Error(err, "Unable to update database privileges")
				return err
			}
		}
	}
	return nil
}
