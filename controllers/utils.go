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

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getResource fetches the resource and writes it to the given reference
// if the resource exists, true is returned, false if not
func getResource(ctx context.Context, r client.Reader, name client.ObjectKey, obj client.Object, opts ...client.GetOption) (bool, error) {
	err := r.Get(ctx, name, obj)
	if err == nil {
		return true, nil
	}
	if errors.IsNotFound(err) {
		return false, nil
	}
	return false, err
}

type ObjectWithConditions interface {
	client.Object
	GetConditions() []metaV1.Condition
	SetConditions(conditions []metaV1.Condition)
}

// setCondition
func setCondition(
	ctx context.Context,
	r client.StatusWriter,
	obj ObjectWithConditions,
	conditionType string,
	status bool,
	reason string,
	message string,
) error {
	statusString := metaV1.ConditionFalse
	if status {
		statusString = metaV1.ConditionTrue
	}
	conditions := obj.GetConditions()
	if meta.IsStatusConditionPresentAndEqual(conditions, conditionType, statusString) {
		return nil
	}
	condition := metaV1.Condition{
		Type:               apiV1.PgConnectedConditionType,
		Status:             statusString,
		ObservedGeneration: obj.GetGeneration(),
		LastTransitionTime: metaV1.Time{Time: time.Time{}},
		Reason:             reason,
		Message:            message,
	}
	meta.SetStatusCondition(&conditions, condition)
	obj.SetConditions(conditions)
	return r.Update(ctx, obj)
}
