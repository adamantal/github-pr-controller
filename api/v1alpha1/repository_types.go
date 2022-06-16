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

// Rules to synchronize pull requests in the repository
type SyncPullRequests struct {
	// Whether the controller should synchronize the pull requests
	Enabled bool `json:"enabled"`

	// Ignores pull requests
	IgnoreLabels []string `json:"ignoreLabels,omitempty"`
}

// RepositorySpec defines the desired state of Repository
type RepositorySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The owner of the repository
	Owner string `json:"owner"`

	// The name of the repository
	Name string `json:"name"`

	// The name of the Kubernetes secret containing the OAuth token required
	// for the controller to access private repositories
	SecretName string `json:"secretName,omitempty"`

	// Whether the controller should sync all the pull requests belonging to the repository
	SyncPullRequests SyncPullRequests `json:"syncPullRequests"`
}

// RepositoryStatus defines the observed state of Repository
type RepositoryStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Whether the Repository has been accessed and the controller has the authorization to get details.
	Accessed bool `json:"accessed"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Repository is the Schema for the repositories API
type Repository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RepositorySpec   `json:"spec,omitempty"`
	Status RepositoryStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RepositoryList contains a list of Repository
type RepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Repository `json:"items"`
}

func init() { // nolint:gochecknoinits
	SchemeBuilder.Register(&Repository{}, &RepositoryList{})
}
