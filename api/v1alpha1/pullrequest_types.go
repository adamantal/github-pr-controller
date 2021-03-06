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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PullRequestSpec defines the desired state of PullRequest
type PullRequestSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The details of the repository this pull request belongs to
	Repository RepositoryDetail `json:"repository"`

	// The git ref to the head of the pull request
	HeadRef string `json:"headRef"`

	// The git ref to the base of the pull request
	BaseRef string `json:"baseRef"`

	// The ID of the pull request
	ID int64 `json:"id"`

	// The number of the pull request
	Number int `json:"number"`
}

type RepositoryDetail struct {
	// The owner of the repository
	Owner string `json:"owner"`

	// The name of the repository
	Name string `json:"name"`
}

// PullRequestState describes the current state of the GitHub pull request.
// +kubebuilder:validation:Enum=Open;Closed
type PullRequestState string

const (
	// Open means that the GitHub pull request is currently open.
	Open PullRequestState = "Open"

	// Closed represents a closed (either merged or unmerged) GitHub pull request state.
	Closed PullRequestState = "Closed"
)

type WorkflowRunStatus struct {
	// The ID of the workflow run
	ID int64 `json:"id"`

	// The status of the run
	Status string `json:"status"`

	// The SHA sum of the HEAD that the workflow refers
	HeadSHA string `json:"headSHA"` // nolint:tagliatelle

	// The conclusion of the run
	Conclusion string `json:"conclusion,omitempty"`
}

// PullRequestStatus defines the observed state of PullRequest
type PullRequestStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The current state of the GitHub pull request.
	State PullRequestState `json:"state"`

	// The labels of the pull request
	Labels []string `json:"labels,omitempty"`

	// Whether there is a workflow in progress at the head of the pull request
	Workflows []WorkflowRunStatus `json:"workflowFinished,omitempty"`
}

func (s *PullRequestStatus) IsEqual(other *PullRequestStatus) bool {
	if s.State != other.State {
		return false
	}

	if !areStringSlicesEqual(s.Labels, other.Labels) {
		return false
	}

	if !areWorkflowRunStatusSlicesEqual(s.Workflows, other.Workflows) {
		return false
	}

	return true
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PullRequest is the Schema for the pullrequests API
type PullRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PullRequestSpec   `json:"spec,omitempty"`
	Status PullRequestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PullRequestList contains a list of PullRequest
type PullRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PullRequest `json:"items"`
}

func init() { // nolint:gochecknoinits
	SchemeBuilder.Register(&PullRequest{}, &PullRequestList{})
}
