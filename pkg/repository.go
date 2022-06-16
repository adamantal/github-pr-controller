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
	client *github.Client // lazy initialized
}

type RepositorySyncInput struct {
	Repository githubv1alpha1.Repository
	Secret     *v1.Secret
}

type RepositorySyncOutput struct {
	Input        *RepositorySyncInput
	PullRequests []*github.PullRequest
}

func NewRepositorySyncer(logger logr.Logger) RepositorySyncer {
	return RepositorySyncer{
		logger: logger,
	}
}

func (rs *RepositorySyncer) Run(ctx context.Context, req RepositorySyncInput) (*RepositorySyncOutput, error) {
	if err := rs.checkAccess(ctx, req); err != nil {
		return nil, errors.Wrap(err, "failed to check access of repository")
	}

	prs, err := rs.sync(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to synchronize repository")
	}

	return &RepositorySyncOutput{
		Input:        &req,
		PullRequests: prs,
	}, nil
}

func (rs *RepositorySyncer) checkAccess(ctx context.Context, req RepositorySyncInput) error {
	rs.logger.Info("checking access to repository")
	if req.Repository.Status.Accessed {
		return nil
	}

	client := rs.getGithubClient(ctx, req.Secret)

	ghRepository, _, err := client.Repositories.Get(ctx, req.Repository.Spec.Owner, req.Repository.Spec.Name)
	if err != nil {
		return errors.Wrap(err, "failed to get repository from github API")
	}

	rs.logger.Info("getting repository details successful", "fullName", ghRepository.FullName)

	return nil
}

func (rs *RepositorySyncer) getGithubClient(ctx context.Context, secret *v1.Secret) *github.Client {
	if rs.client != nil {
		return rs.client
	}
	rs.client = createGithubClient(ctx, secret)
	return rs.client
}

func (rs *RepositorySyncer) sync(ctx context.Context, req RepositorySyncInput) ([]*github.PullRequest, error) {
	rs.logger.Info("syncing repository")

	if !req.Repository.Spec.SyncPullRequests {
		rs.logger.Info("skipping syncing repository")
		return nil, nil
	}

	client := rs.getGithubClient(ctx, req.Secret)
	prs, _, err := client.PullRequests.List(ctx, req.Repository.Spec.Owner, req.Repository.Spec.Name, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list pull requests")
	}

	return prs, nil
}

func createGithubClient(ctx context.Context, secret *v1.Secret) *github.Client {
	var httpClient *http.Client
	if secret != nil {
		bytes := secret.Data[tokenName]
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: string(bytes)},
		)
		httpClient = oauth2.NewClient(ctx, ts)
	}

	return github.NewClient(httpClient)
}
