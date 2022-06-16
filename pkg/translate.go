package pkg

import (
	"fmt"
	"strings"

	"github.com/adamantal/github-pr-controller/api/v1alpha1"
	"github.com/google/go-github/v45/github"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	nameFmt = "%s-%s"
)

func PullRequestToCr(
	pullRequest *github.PullRequest,
	namespace string,
	workflowRuns []*github.WorkflowRun,
) v1alpha1.PullRequest {
	return v1alpha1.PullRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getName(pullRequest),
			Namespace: namespace,
		},
		Spec: v1alpha1.PullRequestSpec{
			BaseRef: *pullRequest.Base.Ref,
			HeadRef: *pullRequest.Head.Ref,
		},
		Status: v1alpha1.PullRequestStatus{
			State:     v1alpha1.PullRequestState(cases.Title(language.Und).String(*pullRequest.State)),
			Labels:    getLabels(pullRequest),
			Workflows: getWorkflowRuns(*pullRequest.ID, workflowRuns),
		},
	}
}

func getName(pullRequest *github.PullRequest) string {
	return fmt.Sprintf(nameFmt,
		blendString(*pullRequest.Base.Ref),
		blendString(*pullRequest.Head.Ref))
}

func blendString(str string) string {
	str = strings.ToLower(str)
	runes := []rune{'\\', '/', '.', '_'}
	for _, r := range runes {
		str = strings.ReplaceAll(str, string(r), "-")
	}
	return str
}

func getLabels(pullRequest *github.PullRequest) []string {
	labels := make([]string, 0, len(pullRequest.Labels))
	for _, label := range pullRequest.Labels {
		labels = append(labels, *label.Name)
	}
	return labels
}

func getWorkflowRuns(prID int64, workflowRuns []*github.WorkflowRun) []v1alpha1.WorkflowRunStatus {
	var statuses []v1alpha1.WorkflowRunStatus
	for _, workflowRun := range workflowRuns {
		for _, pr := range workflowRun.PullRequests {
			if *pr.ID == prID {
				statuses = append(statuses, v1alpha1.WorkflowRunStatus{
					Status:     *workflowRun.Status,
					Conclusion: *workflowRun.Conclusion,
				})
				break
			}
		}
	}
	return statuses
}
