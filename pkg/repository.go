package pkg

import (
	"context"

	githubv1alpha1 "colossyan.com/github-pr-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
)

type RepositorySyncer struct {
	logger logr.Logger
}

func NewRepositorySyncer(logger logr.Logger) RepositorySyncer {
	return RepositorySyncer{
		logger: logger,
	}
}

func (rs *RepositorySyncer) Run(ctx context.Context, repository githubv1alpha1.Repository) error {
	if err := rs.checkAccess(ctx, repository); err != nil {
		return errors.Wrap(err, "failed to check access of repository")
	}

	// check whether it should be synced

	// list all pull requests

	// create pull request CRDs

	return nil
}

func (rs *RepositorySyncer) checkAccess(ctx context.Context, repository githubv1alpha1.Repository) error {
	if repository.Status.Accessed {
		return nil
	}

	client := github.NewClient(nil)

	ghRepository, _, err := client.Repositories.Get(ctx, repository.Spec.Owner, repository.Spec.Name)
	if err != nil {
		return errors.Wrap(err, "failed to get repository from github API")
	}

	rs.logger.Info("getting repository details successful", "fullName", ghRepository.FullName)

	return nil
}
