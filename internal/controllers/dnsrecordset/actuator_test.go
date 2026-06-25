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
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/dns/v2/recordsets"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	orcv1alpha1 "github.com/k-orc/openstack-resource-controller/v2/api/v1alpha1"
	"github.com/k-orc/openstack-resource-controller/v2/internal/osclients/mock"
	orcerrors "github.com/k-orc/openstack-resource-controller/v2/internal/util/errors"
)

var (
	errTest = errors.New("test error")
)

const (
	testRecordsetName = "www.example.com."
	testZoneID        = "test-zone-id"
)

func mockListRecordsets(recordsetsList []recordsets.RecordSet) iter.Seq2[*recordsets.RecordSet, error] {
	return func(yield func(*recordsets.RecordSet, error) bool) {
		for i := range recordsetsList {
			if !yield(&recordsetsList[i], nil) {
				return
			}
		}
	}
}

func TestGetResourceID(t *testing.T) {
	actuator := dnsRecordsetActuator{}
	rs := &recordsets.RecordSet{ID: "test-rs-id"}
	if got := actuator.GetResourceID(rs); got != "test-rs-id" {
		t.Errorf("Expected test-rs-id, got %s", got)
	}
}

func TestGetOSResourceByID(t *testing.T) {
	ctx := context.Background()
	mockctrl := gomock.NewController(t)
	defer mockctrl.Finish()
	mockClient := mock.NewMockDNSRecordsetClient(mockctrl)

	orcObj := &orcv1alpha1.DNSRecordset{
		Spec: orcv1alpha1.DNSRecordsetSpec{
			Resource: &orcv1alpha1.DNSRecordsetResourceSpec{
				DNSZoneRef: "test-zone",
			},
		},
	}

	// Case 1: empty zoneID -> wait-on-parent
	actuatorEmptyZone := dnsRecordsetActuator{zoneID: "", orcObject: orcObj}
	_, status := actuatorEmptyZone.GetOSResourceByID(ctx, "any-id")
	if status == nil {
		t.Errorf("Expected wait status on empty zoneID, got nil")
	}

	// Case 2: success
	mockClient.EXPECT().GetRecordset(ctx, testZoneID, "found").Return(&recordsets.RecordSet{ID: "found", Name: testRecordsetName}, nil)
	actuator := dnsRecordsetActuator{osClient: mockClient, zoneID: testZoneID, orcObject: orcObj}
	res, status := actuator.GetOSResourceByID(ctx, "found")
	if status != nil {
		t.Errorf("Expected nil status, got %v", status)
	}
	if res == nil || res.ID != "found" {
		t.Errorf("Expected recordset with ID 'found', got %v", res)
	}

	// Case 3: error
	mockClient.EXPECT().GetRecordset(ctx, testZoneID, "notfound").Return(nil, errTest)
	res, status = actuator.GetOSResourceByID(ctx, "notfound")
	if status == nil {
		t.Errorf("Expected error status, got nil")
	}
	if res != nil {
		t.Errorf("Expected nil recordset, got %v", res)
	}
}

func TestListOSResourcesForAdoption(t *testing.T) {
	ctx := context.Background()

	orcObj := &orcv1alpha1.DNSRecordset{
		ObjectMeta: metav1.ObjectMeta{
			Name: "www.example.com.",
		},
		Spec: orcv1alpha1.DNSRecordsetSpec{
			Resource: &orcv1alpha1.DNSRecordsetResourceSpec{
				Name:       ptr.To[orcv1alpha1.OpenStackName]("www.example.com."),
				Type:       "A",
				Records:    []string{"1.2.3.4"},
				TTL:        ptr.To[int32](300),
				DNSZoneRef: "test-zone",
			},
		},
	}

	// Case 1: no spec resource
	orcObjNoSpec := &orcv1alpha1.DNSRecordset{}
	actuator := dnsRecordsetActuator{}
	_, canAdopt := actuator.ListOSResourcesForAdoption(ctx, orcObjNoSpec)
	if canAdopt {
		t.Errorf("Expected canAdopt false with no spec resource")
	}

	// Case 2: empty zoneID -> canAdopt is false
	actuatorEmptyZone := dnsRecordsetActuator{zoneID: ""}
	_, canAdopt = actuatorEmptyZone.ListOSResourcesForAdoption(ctx, orcObj)
	if canAdopt {
		t.Errorf("Expected canAdopt false with empty zoneID")
	}

	// Case 3: property match succeeds
	mockctrl := gomock.NewController(t)
	defer mockctrl.Finish()
	mockClient := mock.NewMockDNSRecordsetClient(mockctrl)
	actuator = dnsRecordsetActuator{osClient: mockClient, zoneID: testZoneID, orcObject: orcObj}

	listOpts := recordsets.ListOpts{
		Name: "www.example.com.",
		Type: "A",
	}
	mockClient.EXPECT().ListRecordsets(ctx, testZoneID, listOpts).Return(mockListRecordsets([]recordsets.RecordSet{
		{ID: "1", Name: "www.example.com.", Type: "A", Records: []string{"1.2.3.4"}, TTL: 300},
	}))

	seq, canAdopt := actuator.ListOSResourcesForAdoption(ctx, orcObj)
	if !canAdopt {
		t.Errorf("Expected canAdopt true")
	}
	next, stop := iter.Pull2(seq)
	defer stop()
	f, err, ok := next()
	if !ok || err != nil || f == nil || f.ID != "1" {
		t.Errorf("Expected to fetch recordset with ID '1', got ok=%v, err=%v, f=%v", ok, err, f)
	}

	// Case 4: property mismatch returns Terminal error
	// records mismatch
	mockClient.EXPECT().ListRecordsets(ctx, testZoneID, listOpts).Return(mockListRecordsets([]recordsets.RecordSet{
		{ID: "1", Name: "www.example.com.", Type: "A", Records: []string{"8.8.8.8"}, TTL: 300},
	}))
	seq, _ = actuator.ListOSResourcesForAdoption(ctx, orcObj)
	next, stop = iter.Pull2(seq)
	defer stop()
	_, err, ok = next()
	if !ok || err == nil {
		t.Errorf("Expected mismatch error, got ok=%v, err=%v", ok, err)
	}
	var terminalErr *orcerrors.TerminalError
	if !errors.As(err, &terminalErr) {
		t.Errorf("Expected TerminalError, got %v", err)
	}

	// Case 5: TTL mismatch returns Terminal error
	mockClient.EXPECT().ListRecordsets(ctx, testZoneID, listOpts).Return(mockListRecordsets([]recordsets.RecordSet{
		{ID: "1", Name: "www.example.com.", Type: "A", Records: []string{"1.2.3.4"}, TTL: 600},
	}))
	seq, _ = actuator.ListOSResourcesForAdoption(ctx, orcObj)
	next, stop = iter.Pull2(seq)
	defer stop()
	_, err, ok = next()
	if !ok || err == nil {
		t.Errorf("Expected TTL mismatch error, got ok=%v, err=%v", ok, err)
	}

	// Case 6: description mismatch returns Terminal error
	orcObjWithDesc := orcObj.DeepCopy()
	orcObjWithDesc.Spec.Resource.Description = ptr.To("testing description")
	mockClient.EXPECT().ListRecordsets(ctx, testZoneID, listOpts).Return(mockListRecordsets([]recordsets.RecordSet{
		{ID: "1", Name: "www.example.com.", Type: "A", Records: []string{"1.2.3.4"}, TTL: 300, Description: "different desc"},
	}))
	actuatorWithDesc := dnsRecordsetActuator{osClient: mockClient, zoneID: testZoneID, orcObject: orcObjWithDesc}
	seq, _ = actuatorWithDesc.ListOSResourcesForAdoption(ctx, orcObjWithDesc)
	next, stop = iter.Pull2(seq)
	defer stop()
	_, err, ok = next()
	if !ok || err == nil {
		t.Errorf("Expected description mismatch error, got ok=%v, err=%v", ok, err)
	}
}

func TestCreateResource(t *testing.T) {
	ctx := context.Background()

	orcObj := &orcv1alpha1.DNSRecordset{
		Spec: orcv1alpha1.DNSRecordsetSpec{
			Resource: &orcv1alpha1.DNSRecordsetResourceSpec{
				Name:       ptr.To[orcv1alpha1.OpenStackName]("www.example.com."),
				Type:       "A",
				Records:    []string{"1.2.3.4"},
				TTL:        ptr.To[int32](300),
				DNSZoneRef: "test-zone",
			},
		},
	}

	// Case 1: empty zoneID -> wait-on-parent
	actuatorEmptyZone := dnsRecordsetActuator{zoneID: "", orcObject: orcObj}
	_, status := actuatorEmptyZone.CreateResource(ctx, orcObj)
	if status == nil {
		t.Errorf("Expected wait status on empty zoneID, got nil")
	}

	// Case 2: success
	mockctrl := gomock.NewController(t)
	defer mockctrl.Finish()
	mockClient := mock.NewMockDNSRecordsetClient(mockctrl)
	actuator := dnsRecordsetActuator{osClient: mockClient, zoneID: testZoneID, orcObject: orcObj}

	createOpts := recordsets.CreateOpts{
		Name:    "www.example.com.",
		Type:    "A",
		Records: []string{"1.2.3.4"},
		TTL:     300,
	}
	mockClient.EXPECT().CreateRecordset(ctx, testZoneID, createOpts).Return(&recordsets.RecordSet{ID: "created-id", Name: "www.example.com."}, nil)

	res, status := actuator.CreateResource(ctx, orcObj)
	if status != nil {
		t.Errorf("Expected nil status, got %v", status)
	}
	if res == nil || res.ID != "created-id" {
		t.Errorf("Expected created recordset, got %v", res)
	}

	// Case 3: terminal error on create
	mockClient.EXPECT().CreateRecordset(ctx, testZoneID, createOpts).Return(nil, errTest)
	_, status = actuator.CreateResource(ctx, orcObj)
	if status == nil {
		t.Errorf("Expected error status on create failure, got nil")
	}

	// Case 4: 409 Conflict error on create
	errConflict := gophercloud.ErrUnexpectedResponseCode{Actual: 409}
	mockClient.EXPECT().CreateRecordset(ctx, testZoneID, createOpts).Return(nil, errConflict)
	_, status = actuator.CreateResource(ctx, orcObj)
	if status == nil {
		t.Fatalf("Expected error status on 409 Conflict, got nil")
	}
	err := status.GetError()
	if err == nil {
		t.Fatal("Expected status error to be non-nil")
	}
	var terminalErr *orcerrors.TerminalError
	if !errors.As(err, &terminalErr) {
		t.Errorf("Expected TerminalError for 409 Conflict, got %T", err)
	}
	if terminalErr.Reason != orcv1alpha1.ConditionReasonUnrecoverableError {
		t.Errorf("Expected ConditionReasonUnrecoverableError, got %s", terminalErr.Reason)
	}
}

func TestDeleteResource(t *testing.T) {
	ctx := context.Background()

	orcObj := &orcv1alpha1.DNSRecordset{
		Spec: orcv1alpha1.DNSRecordsetSpec{
			Resource: &orcv1alpha1.DNSRecordsetResourceSpec{
				DNSZoneRef: "test-zone",
			},
		},
	}

	// Case 1: empty zoneID -> wait-on-parent
	actuatorEmptyZone := dnsRecordsetActuator{zoneID: "", orcObject: orcObj}
	status := actuatorEmptyZone.DeleteResource(ctx, orcObj, &recordsets.RecordSet{ID: "any-id"})
	if status == nil {
		t.Errorf("Expected wait status on empty zoneID, got nil")
	}

	// Case 2: success
	mockctrl := gomock.NewController(t)
	defer mockctrl.Finish()
	mockClient := mock.NewMockDNSRecordsetClient(mockctrl)
	actuator := dnsRecordsetActuator{osClient: mockClient, zoneID: testZoneID, orcObject: orcObj}

	mockClient.EXPECT().DeleteRecordset(ctx, testZoneID, "del-id").Return(nil)
	status = actuator.DeleteResource(ctx, orcObj, &recordsets.RecordSet{ID: "del-id"})
	if status != nil {
		t.Errorf("Expected nil status, got %v", status)
	}
}

func TestCreateResourceValidation(t *testing.T) {
	ctx := context.Background()

	// Invalid A record format should trigger validation failure
	orcObj := &orcv1alpha1.DNSRecordset{
		Spec: orcv1alpha1.DNSRecordsetSpec{
			Resource: &orcv1alpha1.DNSRecordsetResourceSpec{
				Name:       ptr.To[orcv1alpha1.OpenStackName]("www.example.com."),
				Type:       "A",
				Records:    []string{"not-an-ip"},
				DNSZoneRef: "test-zone",
			},
		},
	}

	actuator := dnsRecordsetActuator{zoneID: testZoneID, zoneSuffix: "example.com.", orcObject: orcObj}
	_, status := actuator.CreateResource(ctx, orcObj)
	if status == nil {
		t.Fatal("Expected error status on validation failure, got nil")
	}

	err := status.GetError()
	if err == nil {
		t.Fatal("Expected error to be set on status")
	}

	var terminalErr *orcerrors.TerminalError
	if !errors.As(err, &terminalErr) {
		t.Errorf("Expected TerminalError, got %T", err)
	}
}

func TestUpdateResource(t *testing.T) {
	ctx := context.Background()

	orcObj := &orcv1alpha1.DNSRecordset{
		Spec: orcv1alpha1.DNSRecordsetSpec{
			Resource: &orcv1alpha1.DNSRecordsetResourceSpec{
				Name:        ptr.To[orcv1alpha1.OpenStackName]("www.example.com."),
				Type:        "A",
				Records:     []string{"1.2.3.4"},
				TTL:         ptr.To[int32](300),
				Description: ptr.To("new desc"),
				DNSZoneRef:  "test-zone",
			},
		},
	}

	mockctrl := gomock.NewController(t)
	defer mockctrl.Finish()
	mockClient := mock.NewMockDNSRecordsetClient(mockctrl)
	actuator := dnsRecordsetActuator{osClient: mockClient, zoneID: testZoneID, zoneSuffix: "example.com.", orcObject: orcObj}

	// Case 1: No changes -> return nil
	osResourceNoChanges := &recordsets.RecordSet{
		ID:          "rs-id",
		Name:        "www.example.com.",
		Type:        "A",
		Records:     []string{"1.2.3.4"},
		TTL:         300,
		Description: "new desc",
	}
	status := actuator.updateResource(ctx, orcObj, osResourceNoChanges)
	if status != nil {
		t.Errorf("Expected nil status for no changes, got %v", status)
	}

	// Case 2: TTL and Description changed -> call UpdateRecordset
	osResourceWithChanges := &recordsets.RecordSet{
		ID:          "rs-id",
		Name:        "www.example.com.",
		Type:        "A",
		Records:     []string{"1.2.3.4"},
		TTL:         600,
		Description: "old desc",
	}
	expectedDesc := "new desc"
	expectedTTL := 300
	expectedOpts := recordsets.UpdateOpts{
		Description: &expectedDesc,
		TTL:         &expectedTTL,
	}

	mockClient.EXPECT().UpdateRecordset(ctx, testZoneID, "rs-id", expectedOpts).Return(&recordsets.RecordSet{}, nil)
	status = actuator.updateResource(ctx, orcObj, osResourceWithChanges)
	if status == nil {
		t.Fatal("Expected non-nil status for successful update, got nil")
	}
	messages := status.GetProgressMessages()
	if len(messages) == 0 || messages[0] != "Resource status will be refreshed" {
		t.Errorf("Expected progress status to indicate refresh, got %v", messages)
	}

	// Case 3: validation fails during update
	orcObjInvalid := &orcv1alpha1.DNSRecordset{
		Spec: orcv1alpha1.DNSRecordsetSpec{
			Resource: &orcv1alpha1.DNSRecordsetResourceSpec{
				Name:       ptr.To[orcv1alpha1.OpenStackName]("www.example.com."),
				Type:       "A",
				Records:    []string{"invalid-ip"},
				DNSZoneRef: "test-zone",
			},
		},
	}
	status = actuator.updateResource(ctx, orcObjInvalid, osResourceNoChanges)
	if status == nil {
		t.Fatal("Expected non-nil status on validation failure, got nil")
	}
	err := status.GetError()
	if err == nil {
		t.Fatal("Expected error on validation failure")
	}
	var terminalErr *orcerrors.TerminalError
	if !errors.As(err, &terminalErr) {
		t.Errorf("Expected TerminalError, got %T", err)
	}
}

func TestListOSResourcesForImport(t *testing.T) {
	ctx := context.Background()
	mockctrl := gomock.NewController(t)
	defer mockctrl.Finish()
	mockClient := mock.NewMockDNSRecordsetClient(mockctrl)

	orcObj := &orcv1alpha1.DNSRecordset{
		Spec: orcv1alpha1.DNSRecordsetSpec{
			Import: &orcv1alpha1.DNSRecordsetImport{
				Filter: &orcv1alpha1.DNSRecordsetFilter{
					DNSZoneRef: "test-zone",
					Name:       ptr.To[orcv1alpha1.OpenStackName]("www.example.com."),
					Type:       ptr.To("A"),
				},
			},
		},
	}

	actuator := dnsRecordsetActuator{osClient: mockClient, zoneID: testZoneID, orcObject: orcObj}

	listOpts := recordsets.ListOpts{
		Name: "www.example.com.",
		Type: "A",
	}
	mockClient.EXPECT().ListRecordsets(ctx, testZoneID, listOpts).Return(mockListRecordsets([]recordsets.RecordSet{
		{ID: "imported-id", Name: "www.example.com.", Type: "A"},
	}))

	filter := orcObj.Spec.Import.Filter
	seq, status := actuator.ListOSResourcesForImport(ctx, orcObj, *filter)
	if status != nil {
		t.Fatalf("Expected nil status, got %v", status)
	}

	next, stop := iter.Pull2(seq)
	defer stop()
	f, err, ok := next()
	if !ok || err != nil || f == nil || f.ID != "imported-id" {
		t.Errorf("Expected to fetch recordset with ID 'imported-id', got ok=%v, err=%v, f=%v", ok, err, f)
	}
}
