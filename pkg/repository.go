package pkg

import (
	"context"
	"net/http"

	githubv1alpha1 "colossyan.com/github-pr-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	v1 "k8s.io/api/core/v1"
)

const (
	tokenName = "token"
)

type RepositorySyncer struct {
	logger logr.Logger
}

type RepositoryRequest struct {
	Repository githubv1alpha1.Repository
	Secret     *v1.Secret
}

func NewRepositorySyncer(logger logr.Logger) RepositorySyncer {
	return RepositorySyncer{
		logger: logger,
	}
}

func (rs *RepositorySyncer) Run(ctx context.Context, req RepositoryRequest) error {
	if err := rs.checkAccess(ctx, req); err != nil {
		return errors.Wrap(err, "failed to check access of repository")
	}

	// check whether it should be synced

	// list all pull requests

	// create pull request CRDs

	return nil
}

func (rs *RepositorySyncer) checkAccess(ctx context.Context, req RepositoryRequest) error {
	if req.Repository.Status.Accessed {
		return nil
	}

	var httpClient *http.Client
	if req.Repository.Spec.SecretName != "" {
		bytes := req.Secret.Data[tokenName]
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: string(bytes)},
		)
		httpClient = oauth2.NewClient(ctx, ts)
	}

	githubClient := github.NewClient(httpClient)

	ghRepository, _, err := githubClient.Repositories.Get(ctx, req.Repository.Spec.Owner, req.Repository.Spec.Name)
	if err != nil {
		return errors.Wrap(err, "failed to get repository from github API")
	}

	rs.logger.Info("getting repository details successful", "fullName", ghRepository.FullName)

	return nil
}
