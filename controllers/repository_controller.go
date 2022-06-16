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

	"colossyan.com/github-pr-controller/api/v1alpha1"
	"colossyan.com/github-pr-controller/pkg"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	reconcilePeriod = 1 * time.Minute
)

// RepositoryReconciler reconciles a Repository object
type RepositoryReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	reconcilePeriod time.Duration
	Parameters      RepositoryReconcilerParameters
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

	return &RepositoryReconciler{
		Client:          client,
		Scheme:          scheme,
		reconcilePeriod: reconcilePeriod,
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
	syncer := pkg.NewRepositorySyncer(logger)
	repRequest := pkg.RepositorySyncInput{
		Repository: repository,
		Secret:     secret,
	}
	resp, err := syncer.Run(ctx, repRequest)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to sync repository")
	}

	logger.Info("syncing pull requests")
	if err := r.reconcilePullRequests(ctx, resp); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to reconcile pull requests")
	}

	logger.Info("checking status")
	newStatus := v1alpha1.RepositoryStatus{
		Accessed: true,
	}
	if err := r.updateStatus(ctx, logger, repository, newStatus); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to update status")
	}

	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: reconcilePeriod,
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

func (r *RepositoryReconciler) reconcilePullRequests(ctx context.Context, output *pkg.RepositorySyncOutput) error {
	if len(output.PullRequests) == 0 {
		return nil
	}

	pullRequestCrs := make([]v1alpha1.PullRequest, 0, len(output.PullRequests))
	for _, pr := range output.PullRequests {
		pullRequestCrs = append(pullRequestCrs,
			pkg.PullRequestToCr(pr, output.Input.Repository.GetNamespace(), output.WorkflowRuns))
	}

	for _, pullRequestCr := range pullRequestCrs {
		if err := r.reconcilePullRequest(ctx, output.Input.Repository, pullRequestCr); err != nil {
			return errors.Wrap(err, "failed to reconcile pullrequest resource")
		}
	}

	return nil
}

func (r *RepositoryReconciler) reconcilePullRequest(
	ctx context.Context,
	repository v1alpha1.Repository,
	pullRequestCr v1alpha1.PullRequest,
) error {
	existingCr := v1alpha1.PullRequest{}
	objKey := client.ObjectKeyFromObject(&pullRequestCr)
	err := r.Client.Get(ctx, objKey, &existingCr)
	foundOwnerRef := checkOwnerRef(repository, &existingCr)

	switch {
	case err == nil:
		if existingCr.Spec != pullRequestCr.Spec || !foundOwnerRef {
			existingCr.Spec = pullRequestCr.Spec
			if err := r.Client.Update(ctx, &existingCr); err != nil {
				return errors.Wrap(err, "failed to reconcile pullrequest resource")
			}
		}
		if !existingCr.Status.IsEqual(&pullRequestCr.Status) {
			existingCr.Status = pullRequestCr.Status
			if err := r.Client.Status().Update(ctx, &existingCr); err != nil {
				return errors.Wrap(err, "failed to reconcile status subresource of pullrequest")
			}
		}
		return nil

	case apierrors.IsNotFound(err):
		if err := r.Client.Create(ctx, &pullRequestCr); err != nil {
			return errors.Wrap(err, "failed to create pullrequest resource")
		}
		return nil

	default:
		return errors.Wrap(err, "failed to check whether pullrequest exists")
	}
}

func checkOwnerRef(repository v1alpha1.Repository, existingCr *v1alpha1.PullRequest) bool {
	ownerRefs := existingCr.GetOwnerReferences()
	foundOwnerRef := false
	for _, ownerRef := range ownerRefs {
		if repository.GetObjectKind().GroupVersionKind().Kind == ownerRef.Kind &&
			repository.GetName() == ownerRef.Name {
			foundOwnerRef = true
		}
	}
	if !foundOwnerRef {
		ownerRefs = append(ownerRefs, metav1.OwnerReference{
			APIVersion: repository.APIVersion,
			Kind:       repository.GetObjectKind().GroupVersionKind().Kind,
			Name:       repository.GetName(),
			UID:        repository.GetUID(),
		})
		existingCr.SetOwnerReferences(ownerRefs)
	}
	return foundOwnerRef
}
