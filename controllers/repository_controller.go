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

package controllers

import (
	"context"

	githubv1alpha1 "colossyan.com/github-pr-controller/api/v1alpha1"
	"colossyan.com/github-pr-controller/pkg"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// RepositoryReconciler reconciles a Repository object
type RepositoryReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=github.colossyan.com,resources=repositories,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=github.colossyan.com,resources=repositories/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=github.colossyan.com,resources=repositories/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *RepositoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("getting repository from request")
	repository := githubv1alpha1.Repository{}
	if err := r.Client.Get(ctx, req.NamespacedName, &repository); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("syncing repository")
	syncer := pkg.NewRepositorySyncer(logger)
	if err := syncer.Run(ctx, repository); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to sync repository")
	}

	logger.Info("checking status")
	newStatus := githubv1alpha1.RepositoryStatus{
		Accessed: true,
	}
	if err := r.updateStatus(ctx, logger, repository, newStatus); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to update status")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&githubv1alpha1.Repository{}).
		Complete(r)
}

func (r *RepositoryReconciler) updateStatus(
	ctx context.Context,
	logger logr.Logger,
	repository githubv1alpha1.Repository,
	newStatus githubv1alpha1.RepositoryStatus,
) error {
	if repository.Status == newStatus {
		return nil
	}

	updatedRepository := repository.DeepCopy()
	updatedRepository.Status = newStatus

	logger.Info("updating status", "newStatus", newStatus)
	if err := r.Client.Status().Update(ctx, updatedRepository); err != nil {
		return errors.Wrap(err, "failed to update Repository resource")
	}

	return nil
}
