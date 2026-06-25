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
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud/v2/openstack/dns/v2/recordsets"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	orcv1alpha1 "github.com/k-orc/openstack-resource-controller/v2/api/v1alpha1"
	orcerrors "github.com/k-orc/openstack-resource-controller/v2/internal/util/errors"
	orcapplyconfigv1alpha1 "github.com/k-orc/openstack-resource-controller/v2/pkg/clients/applyconfiguration/api/v1alpha1"
)

func TestResourceAvailableStatus(t *testing.T) {
	writer := dnsRecordsetStatusWriter{}

	tests := []struct {
		name           string
		orcObject      *orcv1alpha1.DNSRecordset
		osResource     *recordsets.RecordSet
		expectedStatus metav1.ConditionStatus
		expectRequeue  time.Duration
		expectTerminal bool
	}{
		{
			name: "osResource is nil, Status.ID is nil",
			orcObject: &orcv1alpha1.DNSRecordset{
				Status: orcv1alpha1.DNSRecordsetStatus{
					ID: nil,
				},
			},
			osResource:     nil,
			expectedStatus: metav1.ConditionFalse,
			expectRequeue:  0,
			expectTerminal: false,
		},
		{
			name: "osResource is nil, Status.ID is set",
			orcObject: &orcv1alpha1.DNSRecordset{
				Status: orcv1alpha1.DNSRecordsetStatus{
					ID: ptr.To("some-id"),
				},
			},
			osResource:     nil,
			expectedStatus: metav1.ConditionUnknown,
			expectRequeue:  0,
			expectTerminal: false,
		},
		{
			name:           "recordset is ACTIVE",
			orcObject:      &orcv1alpha1.DNSRecordset{},
			osResource:     &recordsets.RecordSet{Status: "ACTIVE"},
			expectedStatus: metav1.ConditionTrue,
			expectRequeue:  0,
			expectTerminal: false,
		},
		{
			name:           "recordset is PENDING",
			orcObject:      &orcv1alpha1.DNSRecordset{},
			osResource:     &recordsets.RecordSet{Status: "PENDING"},
			expectedStatus: metav1.ConditionFalse,
			expectRequeue:  15 * time.Second,
			expectTerminal: false,
		},
		{
			name:           "recordset is ERROR",
			orcObject:      &orcv1alpha1.DNSRecordset{},
			osResource:     &recordsets.RecordSet{Status: "ERROR"},
			expectedStatus: metav1.ConditionFalse,
			expectRequeue:  0,
			expectTerminal: true,
		},
		{
			name:           "recordset has unknown status",
			orcObject:      &orcv1alpha1.DNSRecordset{},
			osResource:     &recordsets.RecordSet{Status: "UNKNOWN_STATUS"},
			expectedStatus: metav1.ConditionFalse,
			expectRequeue:  15 * time.Second,
			expectTerminal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, rs := writer.ResourceAvailableStatus(tt.orcObject, tt.osResource)
			if status != tt.expectedStatus {
				t.Errorf("expected status %v, got %v", tt.expectedStatus, status)
			}

			if rs == nil {
				if tt.expectRequeue != 0 || tt.expectTerminal {
					t.Errorf("expected non-nil ReconcileStatus")
				}
				return
			}

			if rs.GetRequeue() != tt.expectRequeue {
				t.Errorf("expected requeue %v, got %v", tt.expectRequeue, rs.GetRequeue())
			}

			err := rs.GetError()
			var terminalError *orcerrors.TerminalError
			hasTerminal := errors.As(err, &terminalError)
			if hasTerminal != tt.expectTerminal {
				t.Errorf("expected terminal error %v, got %v (err: %v)", tt.expectTerminal, hasTerminal, err)
			}
		})
	}
}

func TestApplyResourceStatus(t *testing.T) {
	writer := dnsRecordsetStatusWriter{}

	osResource := &recordsets.RecordSet{
		Name:        testRecordsetName,
		Type:        "A",
		Records:     []string{"192.0.2.1", "192.0.2.2"},
		TTL:         3600,
		Description: "A test DNS recordset",
		Status:      "ACTIVE",
	}

	statusApply := orcapplyconfigv1alpha1.DNSRecordsetStatus()
	writer.ApplyResourceStatus(logr.Discard(), osResource, statusApply)

	if statusApply.Resource == nil {
		t.Fatal("expected Resource in apply configuration to be non-nil")
	}

	res := statusApply.Resource
	if res.Name == nil || *res.Name != testRecordsetName {
		t.Errorf("expected name %q, got %v", testRecordsetName, res.Name)
	}
	if res.Type == nil || *res.Type != "A" {
		t.Errorf("expected type 'A', got %v", res.Type)
	}
	if len(res.Records) != 2 || res.Records[0] != "192.0.2.1" || res.Records[1] != "192.0.2.2" {
		t.Errorf("expected records ['192.0.2.1', '192.0.2.2'], got %v", res.Records)
	}
	if res.TTL == nil || *res.TTL != 3600 {
		t.Errorf("expected TTL 3600, got %v", res.TTL)
	}
	if res.Description == nil || *res.Description != "A test DNS recordset" {
		t.Errorf("expected description 'A test DNS recordset', got %v", res.Description)
	}
	if res.Status == nil || *res.Status != "ACTIVE" {
		t.Errorf("expected status 'ACTIVE', got %v", res.Status)
	}
}

func TestApplyResourceStatus_EmptyFields(t *testing.T) {
	writer := dnsRecordsetStatusWriter{}

	osResource := &recordsets.RecordSet{
		Name: testRecordsetName,
	}

	statusApply := orcapplyconfigv1alpha1.DNSRecordsetStatus()
	writer.ApplyResourceStatus(logr.Discard(), osResource, statusApply)

	if statusApply.Resource == nil {
		t.Fatal("expected Resource in apply configuration to be non-nil")
	}

	res := statusApply.Resource
	if res.Name == nil || *res.Name != testRecordsetName {
		t.Errorf("expected name %q, got %v", testRecordsetName, res.Name)
	}
	if res.Type != nil {
		t.Errorf("expected Type to be nil, got %v", res.Type)
	}
	if len(res.Records) != 0 {
		t.Errorf("expected Records to be empty, got %v", res.Records)
	}
	if res.TTL != nil {
		t.Errorf("expected TTL to be nil, got %v", res.TTL)
	}
	if res.Description != nil {
		t.Errorf("expected Description to be nil, got %v", res.Description)
	}
	if res.Status != nil {
		t.Errorf("expected Status to be nil, got %v", res.Status)
	}
}
