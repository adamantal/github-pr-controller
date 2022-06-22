package pkg

import (
	"context"
	"net/http"
	"time"

	githubv1alpha1 "github.com/adamantal/github-pr-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/google/go-github/v45/github"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	v1 "k8s.io/api/core/v1"
)

const (
	tokenName                   = "token"
	pullRequestWorkflowFileName = "pull-request.yml"
	maxPages                    = 20 // let's not rate-limit ourselves

	day               = 24 * time.Hour
	week              = 7 * day
	maxLookBackWindow = 4 * week
)

type RepositorySyncer struct {
	logger logr.Logger
	client *github.Client         // lazy initialized
	cache  *RepositorySyncerCache // cache
}

type RepositorySyncerCache struct {
	workflows map[int64]*github.WorkflowRun
}

type RepositorySyncInput struct {
	Repository githubv1alpha1.Repository
	Secret     *v1.Secret
}

type RepositorySyncOutput struct {
	Input        *RepositorySyncInput
	PullRequests []*github.PullRequest
	WorkflowRuns []*github.WorkflowRun
}

func NewRepositorySyncer(logger logr.Logger, cache *RepositorySyncerCache) RepositorySyncer {
	return RepositorySyncer{
		logger: logger,
		cache:  cache,
	}
}

func NewRepositorySyncerCache() *RepositorySyncerCache {
	return &RepositorySyncerCache{
		workflows: make(map[int64]*github.WorkflowRun),
	}
}

func (rsc *RepositorySyncerCache) SaveInCache(workflowRuns []*github.WorkflowRun) bool {
	shouldBreak := false
	for _, workflowRun := range workflowRuns {
		_, existed := rsc.workflows[*workflowRun.ID]
		rsc.workflows[*workflowRun.ID] = workflowRun
		if existed {
			shouldBreak = true
		}
	}
	return shouldBreak
}

func (rsc *RepositorySyncerCache) GetAllRuns() []*github.WorkflowRun {
	runs := make([]*github.WorkflowRun, 0, len(rsc.workflows))
	for _, run := range rsc.workflows {
		runs = append(runs, run)
	}
	return runs
}

func (rs *RepositorySyncer) Run(ctx context.Context, req RepositorySyncInput) (*RepositorySyncOutput, error) {
	if err := rs.checkAccess(ctx, req); err != nil {
		return nil, errors.Wrap(err, "failed to check access of repository")
	}

	prs, workflowRuns, err := rs.sync(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to synchronize repository")
	}

	return &RepositorySyncOutput{
		Input:        &req,
		PullRequests: prs,
		WorkflowRuns: workflowRuns,
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

func (rs *RepositorySyncer) sync(
	ctx context.Context,
	req RepositorySyncInput,
) ([]*github.PullRequest, []*github.WorkflowRun, error) {
	rs.logger.Info("syncing repository")

	if !req.Repository.Spec.SyncPullRequests.Enabled {
		rs.logger.Info("skipping syncing repository")
		return nil, nil, nil
	}

	client := rs.getGithubClient(ctx, req.Secret)
	prs, _, err := client.PullRequests.List(ctx, req.Repository.Spec.Owner, req.Repository.Spec.Name, nil)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to list pull requests")
	}
	prs = ignorePrsByLabels(prs, req)

	earliestTS := getEarliestPullRequestTS(prs)
	if earliestTS.Before(time.Now().Add(-1 * maxLookBackWindow)) {
		earliestTS = time.Now().Add(-1 * maxLookBackWindow)
	}
	rs.logger.V(1).Info("determined earliest timestamp", "ts", earliestTS)

	page := 1
	for ; page < maxPages; page++ {
		opts := github.ListWorkflowRunsOptions{
			Event: "pull_request",
			ListOptions: github.ListOptions{
				Page: page,
			},
		}

		workflowRuns, _, err := client.Actions.ListWorkflowRunsByFileName(
			ctx, req.Repository.Spec.Owner, req.Repository.Spec.Name, pullRequestWorkflowFileName, &opts)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to list workflows in repo")
		}

		cacheHit := rs.cache.SaveInCache(workflowRuns.WorkflowRuns)

		if workflowRuns.WorkflowRuns[len(workflowRuns.WorkflowRuns)-1].CreatedAt.Before(earliestTS) || cacheHit {
			break
		}
	}
	rs.logger.V(1).Info("paginated workflow runs collected", "pages", page)

	return prs, rs.cache.GetAllRuns(), nil
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

func getEarliestPullRequestTS(prs []*github.PullRequest) time.Time {
	earliestTS := time.Now()
	for _, pr := range prs {
		if earliestTS.After(*pr.CreatedAt) {
			earliestTS = *pr.CreatedAt
		}
	}
	return earliestTS
}

func ignorePrsByLabels(prs []*github.PullRequest, req RepositorySyncInput) []*github.PullRequest {
	if len(req.Repository.Spec.SyncPullRequests.IgnoreLabels) == 0 {
		return prs
	}

	filtered := make([]*github.PullRequest, 0, len(prs))
	for _, pr := range prs {
		if !containsAny(pr.Labels, req.Repository.Spec.SyncPullRequests.IgnoreLabels) {
			filtered = append(filtered, pr)
		}
	}

	return filtered
}

func containsAny(labels []*github.Label, ignoreLabels []string) bool {
	for _, ignoreLabel := range ignoreLabels {
		for _, label := range labels {
			if *label.Name == ignoreLabel {
				return true
			}
		}
	}
	return false
}
