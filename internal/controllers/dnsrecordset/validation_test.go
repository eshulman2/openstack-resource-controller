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

package dnsrecordset

import (
	"testing"

	"k8s.io/utils/ptr"

	orcv1alpha1 "github.com/k-orc/openstack-resource-controller/v2/api/v1alpha1"
)

func TestValidateDNSRecordset(t *testing.T) {
	tests := []struct {
		name        string
		obj         *orcv1alpha1.DNSRecordset
		zoneSuffix  string
		wantErr     bool
		wantRecords []string
	}{
		{
			name: "Valid A Record",
			obj: &orcv1alpha1.DNSRecordset{
				Spec: orcv1alpha1.DNSRecordsetSpec{
					Resource: &orcv1alpha1.DNSRecordsetResourceSpec{
						Name:    ptr.To[orcv1alpha1.OpenStackName]("www.example.com."),
						Type:    "A",
						Records: []string{"192.168.1.1", "10.0.0.1"},
					},
				},
			},
			zoneSuffix: "example.com.",
			wantErr:    false,
		},
		{
			name: "Invalid A Record format",
			obj: &orcv1alpha1.DNSRecordset{
				Spec: orcv1alpha1.DNSRecordsetSpec{
					Resource: &orcv1alpha1.DNSRecordsetResourceSpec{
						Name:    ptr.To[orcv1alpha1.OpenStackName]("www.example.com."),
						Type:    "A",
						Records: []string{"192.168.1.300"},
					},
				},
			},
			zoneSuffix: "example.com.",
			wantErr:    true,
		},
		{
			name: "Valid AAAA Record",
			obj: &orcv1alpha1.DNSRecordset{
				Spec: orcv1alpha1.DNSRecordsetSpec{
					Resource: &orcv1alpha1.DNSRecordsetResourceSpec{
						Name:    ptr.To[orcv1alpha1.OpenStackName]("www.example.com."),
						Type:    "AAAA",
						Records: []string{"2001:db8::1"},
					},
				},
			},
			zoneSuffix: "example.com.",
			wantErr:    false,
		},
		{
			name: "Invalid AAAA Record format",
			obj: &orcv1alpha1.DNSRecordset{
				Spec: orcv1alpha1.DNSRecordsetSpec{
					Resource: &orcv1alpha1.DNSRecordsetResourceSpec{
						Name:    ptr.To[orcv1alpha1.OpenStackName]("www.example.com."),
						Type:    "AAAA",
						Records: []string{"1.2.3.4"},
					},
				},
			},
			zoneSuffix: "example.com.",
			wantErr:    true,
		},
		{
			name: "Valid CNAME Record",
			obj: &orcv1alpha1.DNSRecordset{
				Spec: orcv1alpha1.DNSRecordsetSpec{
					Resource: &orcv1alpha1.DNSRecordsetResourceSpec{
						Name:    ptr.To[orcv1alpha1.OpenStackName]("www.example.com."),
						Type:    "CNAME",
						Records: []string{"target.example.com."},
					},
				},
			},
			zoneSuffix: "example.com.",
			wantErr:    false,
		},
		{
			name: "Invalid CNAME Record (missing trailing dot)",
			obj: &orcv1alpha1.DNSRecordset{
				Spec: orcv1alpha1.DNSRecordsetSpec{
					Resource: &orcv1alpha1.DNSRecordsetResourceSpec{
						Name:    ptr.To[orcv1alpha1.OpenStackName]("www.example.com."),
						Type:    "CNAME",
						Records: []string{"target.example.com"},
					},
				},
			},
			zoneSuffix: "example.com.",
			wantErr:    true,
		},
		{
			name: "Valid TXT Record already quoted",
			obj: &orcv1alpha1.DNSRecordset{
				Spec: orcv1alpha1.DNSRecordsetSpec{
					Resource: &orcv1alpha1.DNSRecordsetResourceSpec{
						Name:    ptr.To[orcv1alpha1.OpenStackName]("www.example.com."),
						Type:    "TXT",
						Records: []string{`"hello"`},
					},
				},
			},
			zoneSuffix:  "example.com.",
			wantErr:     false,
			wantRecords: []string{`"hello"`},
		},
		{
			name: "Valid TXT Record needing quotes",
			obj: &orcv1alpha1.DNSRecordset{
				Spec: orcv1alpha1.DNSRecordsetSpec{
					Resource: &orcv1alpha1.DNSRecordsetResourceSpec{
						Name:    ptr.To[orcv1alpha1.OpenStackName]("www.example.com."),
						Type:    "TXT",
						Records: []string{"hello"},
					},
				},
			},
			zoneSuffix:  "example.com.",
			wantErr:     false,
			wantRecords: []string{`"hello"`},
		},
		{
			name: "Invalid TXT Record (mismatched prefix quote)",
			obj: &orcv1alpha1.DNSRecordset{
				Spec: orcv1alpha1.DNSRecordsetSpec{
					Resource: &orcv1alpha1.DNSRecordsetResourceSpec{
						Name:    ptr.To[orcv1alpha1.OpenStackName]("www.example.com."),
						Type:    "TXT",
						Records: []string{`"hello`},
					},
				},
			},
			zoneSuffix: "example.com.",
			wantErr:    true,
		},
		{
			name: "Invalid TXT Record (mismatched suffix quote)",
			obj: &orcv1alpha1.DNSRecordset{
				Spec: orcv1alpha1.DNSRecordsetSpec{
					Resource: &orcv1alpha1.DNSRecordsetResourceSpec{
						Name:    ptr.To[orcv1alpha1.OpenStackName]("www.example.com."),
						Type:    "TXT",
						Records: []string{`hello"`},
					},
				},
			},
			zoneSuffix: "example.com.",
			wantErr:    true,
		},
		{
			name: "Invalid name suffix matching",
			obj: &orcv1alpha1.DNSRecordset{
				Spec: orcv1alpha1.DNSRecordsetSpec{
					Resource: &orcv1alpha1.DNSRecordsetResourceSpec{
						Name:    ptr.To[orcv1alpha1.OpenStackName]("www.otherdomain.com."),
						Type:    "A",
						Records: []string{"192.168.1.1"},
					},
				},
			},
			zoneSuffix: "example.com.",
			wantErr:    true,
		},
		{
			name: "Suffix collision but not subdomain",
			obj: &orcv1alpha1.DNSRecordset{
				Spec: orcv1alpha1.DNSRecordsetSpec{
					Resource: &orcv1alpha1.DNSRecordsetResourceSpec{
						Name:    ptr.To[orcv1alpha1.OpenStackName]("notexample.com."),
						Type:    "A",
						Records: []string{"192.168.1.1"},
					},
				},
			},
			zoneSuffix: "example.com.",
			wantErr:    true,
		},
		{
			name: "Exact match of zone suffix (Apex record)",
			obj: &orcv1alpha1.DNSRecordset{
				Spec: orcv1alpha1.DNSRecordsetSpec{
					Resource: &orcv1alpha1.DNSRecordsetResourceSpec{
						Name:    ptr.To[orcv1alpha1.OpenStackName]("example.com."),
						Type:    "A",
						Records: []string{"192.168.1.1"},
					},
				},
			},
			zoneSuffix: "example.com.",
			wantErr:    false,
		},
		{
			name: "Empty records list",
			obj: &orcv1alpha1.DNSRecordset{
				Spec: orcv1alpha1.DNSRecordsetSpec{
					Resource: &orcv1alpha1.DNSRecordsetResourceSpec{
						Name:    ptr.To[orcv1alpha1.OpenStackName]("www.example.com."),
						Type:    "A",
						Records: []string{},
					},
				},
			},
			zoneSuffix: "example.com.",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDNSRecordset(tt.obj, tt.zoneSuffix)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDNSRecordset() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && tt.wantRecords != nil {
				normalized := getNormalizedRecords(tt.obj.Spec.Resource.Type, tt.obj.Spec.Resource.Records)
				for i, r := range normalized {
					if r != tt.wantRecords[i] {
						t.Errorf("getNormalizedRecords() record = %q, want %q", r, tt.wantRecords[i])
					}
				}
			}
		})
	}
}
