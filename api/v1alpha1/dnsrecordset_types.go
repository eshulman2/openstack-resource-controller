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

// DNSRecordsetResourceSpec specifies the desired state of the resource.
type DNSRecordsetResourceSpec struct {
	// name will be the name of the created resource. If not specified, the
	// name of the ORC object will be used.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	// +kubebuilder:validation:XValidation:rule="self.endsWith('.')",message="name must end with a period"
	// +optional
	Name *OpenStackName `json:"name,omitempty"`

	// type is the type of the recordset (e.g., A, AAAA, CNAME, MX, TXT, etc.).
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="type is immutable"
	// +kubebuilder:validation:MaxLength:=255
	// +required
	Type string `json:"type"`

	// records are the DNS records of the recordset.
	// +kubebuilder:validation:MinItems:=1
	// +kubebuilder:validation:MaxItems:=1024
	// +kubebuilder:validation:items:MaxLength:=1024
	// +listType=atomic
	// +required
	Records []string `json:"records,omitempty"`

	// ttl is the Time To Live for the recordset.
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=2147483647
	// +optional
	TTL *int32 `json:"ttl,omitempty"`

	// description is a human-readable description for the resource.
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=255
	// +optional
	Description *string `json:"description,omitempty"`

	// dnsZoneRef is a reference to the ORC DNSZone this recordset is associated with.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="dnsZoneRef is immutable"
	// +required
	DNSZoneRef KubernetesNameRef `json:"dnsZoneRef,omitempty"`
}

// DNSRecordsetFilter defines an existing resource by its properties.
// +kubebuilder:validation:MinProperties:=1
type DNSRecordsetFilter struct {
	// dnsZoneRef is a reference to the ORC DNSZone this recordset is associated with.
	// +required
	DNSZoneRef KubernetesNameRef `json:"dnsZoneRef,omitempty"`

	// name of the existing resource.
	// +kubebuilder:validation:XValidation:rule="self.endsWith('.')",message="name must end with a period"
	// +optional
	Name *OpenStackName `json:"name,omitempty"`

	// type of the existing resource.
	// +kubebuilder:validation:MaxLength:=255
	// +optional
	Type *string `json:"type,omitempty"`

	// ttl of the existing resource.
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=2147483647
	// +optional
	TTL *int32 `json:"ttl,omitempty"`

	// description of the existing resource.
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=255
	// +optional
	Description *string `json:"description,omitempty"`
}

// DNSRecordsetImport specifies an existing resource which will be imported instead of
// creating a new one.
// +kubebuilder:validation:MinProperties:=1
// +kubebuilder:validation:MaxProperties:=1
type DNSRecordsetImport struct {
	// filter contains a resource query which is expected to return a single
	// result. The controller will continue to retry if filter returns no
	// results. If filter returns multiple results the controller will set an
	// error state and will not continue to retry.
	// +required
	Filter *DNSRecordsetFilter `json:"filter,omitempty"` //nolint:kubeapilinter // Filter is a required pointer field because DNSRecordsetImport has no other fields, and required struct fields are represented as pointers across ORC.
}

// DNSRecordsetResourceStatus represents the observed state of the resource.
type DNSRecordsetResourceStatus struct {
	// name is a human-readable name for the resource.
	// +kubebuilder:validation:MaxLength=1024
	// +optional
	Name string `json:"name,omitempty"`

	// type is the type of the recordset.
	// +kubebuilder:validation:MaxLength=255
	// +optional
	Type string `json:"type,omitempty"`

	// records are the DNS records of the recordset.
	// +kubebuilder:validation:MaxItems:=1024
	// +kubebuilder:validation:items:MaxLength:=1024
	// +listType=atomic
	// +optional
	Records []string `json:"records,omitempty"`

	// ttl is the Time To Live for the recordset in seconds.
	// +optional
	TTL *int32 `json:"ttl,omitempty"`

	// description is a human-readable description for the resource.
	// +kubebuilder:validation:MaxLength=1024
	// +optional
	Description string `json:"description,omitempty"`

	// status is the status of the resource.
	// +kubebuilder:validation:MaxLength=255
	// +optional
	Status string `json:"status,omitempty"`
}
