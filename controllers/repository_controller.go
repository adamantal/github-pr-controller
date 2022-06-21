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
	"time"

	"github.com/adamantal/github-pr-controller/api/v1alpha1"
	"github.com/adamantal/github-pr-controller/pkg"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// RepositoryReconciler reconciles a Repository object
type RepositoryReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	reconcilePeriod time.Duration
	Parameters      RepositoryReconcilerParameters
	cache           *pkg.RepositorySyncerCache
}

func NewRepositoryReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	params RepositoryReconcilerParameters,
) (*RepositoryReconciler, error) {
	reconcilePeriod, err := time.ParseDuration(params.ReconcilePeriod)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse duration for reconciler")
	}

	cache := pkg.NewRepositorySyncerCache()

	return &RepositoryReconciler{
		Client:          client,
		Scheme:          scheme,
		reconcilePeriod: reconcilePeriod,
		cache:           cache,
	}, nil
}

type RepositoryReconcilerParameters struct {
	ReconcilePeriod string `json:"reconcilePeriod"`
	DefaultToken    string `json:"defaultToken"`
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
	repository := v1alpha1.Repository{}
	if err := r.Client.Get(ctx, req.NamespacedName, &repository); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var secret *v1.Secret
	if repository.Spec.SecretName != "" {
		secretName := repository.Spec.SecretName
		logger.Info("obtaining secret", "secretName", secretName)
		secret = &v1.Secret{}
		if err := r.Client.Get(ctx, types.NamespacedName{
			Name:      secretName,
			Namespace: req.Namespace,
		}, secret); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "could not find secret for repository")
		}
	}

	logger.Info("syncing repository")
	syncer := pkg.NewRepositorySyncer(logger, r.cache)
	repRequest := pkg.RepositorySyncInput{
		Repository: repository,
		Secret:     secret,
	}
	resp, err := syncer.Run(ctx, repRequest)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to sync repository")
	}

	logger.Info("syncing pull requests")
	if err := r.reconcilePullRequests(ctx, logger, resp); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to reconcile pull requests")
	}

	logger.Info("checking status")
	newStatus := v1alpha1.RepositoryStatus{
		Accessed: true,
	}
	if err := r.updateStatus(ctx, logger, repository, newStatus); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to update status")
	}

	logger.V(1).Info("reconcile finsihed")
	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: r.reconcilePeriod,
	}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Repository{}).
		Complete(r)
}

func (r *RepositoryReconciler) updateStatus(
	ctx context.Context,
	logger logr.Logger,
	repository v1alpha1.Repository,
	newStatus v1alpha1.RepositoryStatus,
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

func (r *RepositoryReconciler) reconcilePullRequests(
	ctx context.Context,
	logger logr.Logger,
	output *pkg.RepositorySyncOutput,
) error {
	existingPrs := v1alpha1.PullRequestList{}
	err := r.Client.List(ctx, &existingPrs, client.InNamespace(output.Input.Repository.GetNamespace()))
	if err != nil {
		return errors.Wrap(err, "failed to list pullrequests")
	}

	prsOwnedByRepository := []v1alpha1.PullRequest{}
	for _, existingPr := range existingPrs.Items {
		ownerRefs := existingPr.GetOwnerReferences()
		for _, ownerRef := range ownerRefs {
			if output.Input.Repository.GetObjectKind().GroupVersionKind().Kind == ownerRef.Kind &&
				output.Input.Repository.GetName() == ownerRef.Name {
				prsOwnedByRepository = append(prsOwnedByRepository, existingPr)
			}
		}
	}

	pullRequestCrs := make([]v1alpha1.PullRequest, 0, len(output.PullRequests))
	for _, pr := range output.PullRequests {
		pullRequestCrs = append(pullRequestCrs, pkg.PullRequestToCr(pr, output))
	}

	if err := r.handleThreeWayDiff(ctx, logger, output, prsOwnedByRepository, pullRequestCrs); err != nil {
		return errors.Wrap(err, "handling three way diff failed")
	}

	return nil
}

func (r *RepositoryReconciler) handleThreeWayDiff(
	ctx context.Context,
	logger logr.Logger,
	output *pkg.RepositorySyncOutput,
	prsOwnedByRepository, pullRequestCrs []v1alpha1.PullRequest,
) error {
	newPrs, updateablePrs, deleteablePrs := groupPullRequests(
		output.Input.Repository, prsOwnedByRepository, pullRequestCrs)
	logger.Info("3-way diff calculated on resource",
		"new", len(newPrs),
		"update", len(updateablePrs),
		"delete", len(deleteablePrs))

	for i := range newPrs {
		if err := r.Client.Create(ctx, &newPrs[i]); err != nil {
			return errors.Wrap(err, "failed to create pullrequest resource")
		}
	}

	for index := range updateablePrs {
		prStatus := updateablePrs[index].Status.DeepCopy()
		if err := r.Client.Update(ctx, &updateablePrs[index]); err != nil {
			return errors.Wrap(err, "failed to update pullrequest resource")
		}
		updateablePrs[index].Status = *prStatus
		if err := r.Client.Status().Update(ctx, &updateablePrs[index]); err != nil {
			return errors.Wrap(err, "failed to update pullrequeststatus resource")
		}
	}

	for i := range deleteablePrs {
		if err := r.Client.Delete(ctx, &deleteablePrs[i]); err != nil {
			return errors.Wrap(err, "failed to delete pullrequest resource")
		}
	}

	return nil
}

func groupPullRequests(
	repository v1alpha1.Repository,
	existingPrs []v1alpha1.PullRequest,
	expectedPrs []v1alpha1.PullRequest,
) ([]v1alpha1.PullRequest, []v1alpha1.PullRequest, []v1alpha1.PullRequest) {
	existingPrMap := make(map[string]*v1alpha1.PullRequest)
	for i, existingPr := range existingPrs {
		existingPrMap[existingPr.GetName()] = &existingPrs[i]
	}
	var newprs, updateprs, delprs []v1alpha1.PullRequest
	for _, expectedPr := range expectedPrs {
		existingPr := existingPrMap[expectedPr.GetName()]
		if existingPr != nil {
			if shouldUpdatePullRequest(repository, *existingPr, expectedPr) {
				// keep metadata, update ownerref, spec and status fields
				updateablePr := existingPr.DeepCopy()

				ensureOwnerRef(repository, updateablePr)
				updateablePr.Spec = expectedPr.Spec
				updateablePr.Status = expectedPr.Status
				updateprs = append(updateprs, *updateablePr)
			}
		} else {
			newprs = append(newprs, expectedPr)
		}
		delete(existingPrMap, expectedPr.GetName())
	}

	if len(existingPrMap) != 0 {
		for _, existingPr := range existingPrMap {
			delprs = append(delprs, *existingPr)
		}
	}

	return newprs, updateprs, delprs
}

func shouldUpdatePullRequest(
	repository v1alpha1.Repository,
	existingCr v1alpha1.PullRequest,
	pullRequestCr v1alpha1.PullRequest,
) bool {
	return existingCr.Spec != pullRequestCr.Spec ||
		!checkOwnerRef(repository, &existingCr) ||
		!existingCr.Status.IsEqual(&pullRequestCr.Status)
}

func checkOwnerRef(repository v1alpha1.Repository, existingCr *v1alpha1.PullRequest) bool {
	ownerRefs := existingCr.GetOwnerReferences()
	for _, ownerRef := range ownerRefs {
		if repository.GetObjectKind().GroupVersionKind().Kind == ownerRef.Kind &&
			repository.GetName() == ownerRef.Name {
			return true
		}
	}
	return false
}

func ensureOwnerRef(repository v1alpha1.Repository, pullRequest *v1alpha1.PullRequest) {
	ownerRefs := pullRequest.GetOwnerReferences()
	foundOwnerRef := checkOwnerRef(repository, pullRequest)

	if !foundOwnerRef {
		ownerRefs = append(ownerRefs, metav1.OwnerReference{
			APIVersion: repository.APIVersion,
			Kind:       repository.GetObjectKind().GroupVersionKind().Kind,
			Name:       repository.GetName(),
			UID:        repository.GetUID(),
		})
		pullRequest.SetOwnerReferences(ownerRefs)
	}
}
