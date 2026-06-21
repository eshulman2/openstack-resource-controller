/*
Copyright The ORC Authors.

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

// SwiftContainerName is the name of a Swift container. It must be between 1
// and 256 characters long and must not contain forward slashes.
// +kubebuilder:validation:MinLength:=1
// +kubebuilder:validation:MaxLength:=256
// +kubebuilder:validation:Pattern:=`^[^/]+$`
// +kubebuilder:validation:XValidation:rule="self.size() <= 256",message="name must not exceed 256 UTF-8 bytes"
type SwiftContainerName string

// SwiftContainerMetadata defines a key-value pair to be set as a Swift
// container metadata header (X-Container-Meta-<key>: <value>).
type SwiftContainerMetadata struct {
	// key is the name of the metadata item. It will be used as the suffix of
	// the X-Container-Meta-* header.
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=255
	// +required
	Key string `json:"key"`

	// value is the value of the metadata item.
	// +kubebuilder:validation:MaxLength:=255
	// +required
	Value string `json:"value"`
}

// SwiftContainerMetadataStatus represents an observed metadata key-value pair
// on a Swift container.
type SwiftContainerMetadataStatus struct {
	// key is the name of the metadata item.
	// +kubebuilder:validation:MaxLength:=255
	// +optional
	Key string `json:"key,omitempty"`

	// value is the value of the metadata item.
	// +kubebuilder:validation:MaxLength:=255
	// +optional
	Value string `json:"value,omitempty"`
}

// SwiftContainerResourceSpec contains the desired state of a Swift container.
type SwiftContainerResourceSpec struct {
	// name will be the name of the created Swift container. If not specified,
	// the name of the ORC object will be used. The name must be unique within
	// the account and must not contain forward slashes.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name *SwiftContainerName `json:"name,omitempty"`

	// metadata is a list of key-value pairs which will be set as
	// X-Container-Meta-* headers on the Swift container.
	// +kubebuilder:validation:MaxItems:=64
	// +listType=atomic
	// +optional
	Metadata []SwiftContainerMetadata `json:"metadata,omitempty"`

	// containerRead sets the X-Container-Read ACL header which defines who
	// can read objects in the container. Common values include ".r:*" for
	// public read access or a comma-separated list of account/container
	// combinations.
	// +kubebuilder:validation:MaxLength:=256
	// +optional
	ContainerRead *string `json:"containerRead,omitempty"`

	// containerWrite sets the X-Container-Write ACL header which defines who
	// can write objects to the container. Common values include a
	// comma-separated list of account/container combinations.
	// +kubebuilder:validation:MaxLength:=256
	// +optional
	ContainerWrite *string `json:"containerWrite,omitempty"`

	// storagePolicy is the name of the storage policy to use for this
	// container. If not specified, the cluster's default storage policy will
	// be used. This field is immutable after creation.
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=255
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="storagePolicy is immutable"
	StoragePolicy *string `json:"storagePolicy,omitempty"`
}
