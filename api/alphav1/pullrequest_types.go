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

package alphav1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PullRequestSpec defines the desired state of PullRequest
type PullRequestSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// SourceBranch is the source branch of the pull request
	SourceBranch string `json:"sourceBranch"`

	// TargetBranch is the target branch of the pull request
	TargetBranch string `json:"targetBranch"`
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

// PullRequestStatus defines the observed state of PullRequest
type PullRequestStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The current state of the GitHub pull request.
	State PullRequestState `json:"state"`
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

func init() {
	SchemeBuilder.Register(&PullRequest{}, &PullRequestList{})
}
