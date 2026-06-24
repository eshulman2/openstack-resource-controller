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

package dnszone

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/dns/v2/zones"
	orcv1alpha1 "github.com/k-orc/openstack-resource-controller/v2/api/v1alpha1"
	orcerrors "github.com/k-orc/openstack-resource-controller/v2/internal/util/errors"
	"k8s.io/utils/ptr"
)

var (
	errNotImplemented = errors.New("not implemented")
	errTest           = errors.New("test error")
)

type mockDNSZoneClient struct {
	zones    []zones.Zone
	getFn    func(ctx context.Context, id string) (*zones.Zone, error)
	createFn func(ctx context.Context, opts zones.CreateOptsBuilder) (*zones.Zone, error)
	deleteFn func(ctx context.Context, id string) error
	updateFn func(ctx context.Context, id string, opts zones.UpdateOptsBuilder) (*zones.Zone, error)
}

func (m mockDNSZoneClient) ListZones(_ context.Context, _ zones.ListOptsBuilder) iter.Seq2[*zones.Zone, error] {
	return func(yield func(*zones.Zone, error) bool) {
		for i := range m.zones {
			if !yield(&m.zones[i], nil) {
				return
			}
		}
	}
}

func (m mockDNSZoneClient) CreateZone(ctx context.Context, opts zones.CreateOptsBuilder) (*zones.Zone, error) {
	if m.createFn != nil {
		return m.createFn(ctx, opts)
	}
	return nil, errNotImplemented
}

func (m mockDNSZoneClient) DeleteZone(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return errNotImplemented
}

func (m mockDNSZoneClient) GetZone(ctx context.Context, id string) (*zones.Zone, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return nil, errNotImplemented
}

func (m mockDNSZoneClient) UpdateZone(ctx context.Context, id string, opts zones.UpdateOptsBuilder) (*zones.Zone, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, opts)
	}
	return nil, errNotImplemented
}

type zoneResult struct {
	zone *zones.Zone
	err  error
}

func TestGetResourceID(t *testing.T) {
	actuator := dnsZoneActuator{}
	zone := &zones.Zone{ID: "test-zone-id"}
	if got := actuator.GetResourceID(zone); got != "test-zone-id" {
		t.Errorf("Expected test-zone-id, got %s", got)
	}
}

func TestGetOSResourceByID(t *testing.T) {
	ctx := context.Background()
	client := mockDNSZoneClient{
		getFn: func(ctx context.Context, id string) (*zones.Zone, error) {
			if id == "found" {
				return &zones.Zone{ID: "found", Name: "example.com."}, nil
			}
			return nil, errTest
		},
	}
	actuator := dnsZoneActuator{osClient: client}

	// Case 1: success
	res, status := actuator.GetOSResourceByID(ctx, "found")
	if status != nil {
		t.Errorf("Expected nil status, got %v", status)
	}
	if res == nil || res.ID != "found" {
		t.Errorf("Expected zone with ID 'found', got %v", res)
	}

	// Case 2: error
	res, status = actuator.GetOSResourceByID(ctx, "notfound")
	if status == nil {
		t.Errorf("Expected error status, got nil")
	}
	if res != nil {
		t.Errorf("Expected nil zone, got %v", res)
	}
}

func TestListOSResourcesForAdoption(t *testing.T) {
	for _, tc := range [...]struct {
		name         string
		resourceSpec orcv1alpha1.DNSZoneResourceSpec
		zones        []zones.Zone
		expectCount  int
		expectIDs    []string
	}{
		{
			name: "exact match",
			resourceSpec: orcv1alpha1.DNSZoneResourceSpec{
				Name:        ptr.To[orcv1alpha1.OpenStackName]("example.com."),
				Email:       "admin@example.com",
				Description: ptr.To("desc"),
				TTL:         ptr.To[int32](3600),
				Type:        orcv1alpha1.DNSZoneTypePrimary,
			},
			zones: []zones.Zone{
				{ID: "1", Name: "example.com.", Email: "admin@example.com", Description: "desc", TTL: 3600, Type: "PRIMARY"},
				{ID: "2", Name: "example.com.", Email: "other@example.com", Description: "desc", TTL: 3600, Type: "PRIMARY"},
			},
			expectCount: 1,
			expectIDs:   []string{"1"},
		},
		{
			name: "no spec description, matches empty description",
			resourceSpec: orcv1alpha1.DNSZoneResourceSpec{
				Name:  ptr.To[orcv1alpha1.OpenStackName]("example.com."),
				Email: "admin@example.com",
				Type:  orcv1alpha1.DNSZoneTypePrimary,
			},
			zones: []zones.Zone{
				{ID: "1", Name: "example.com.", Email: "admin@example.com", Description: "", Type: "PRIMARY"},
				{ID: "2", Name: "example.com.", Email: "admin@example.com", Description: "some-desc", Type: "PRIMARY"},
			},
			expectCount: 1,
			expectIDs:   []string{"1"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			client := mockDNSZoneClient{zones: tc.zones}
			actuator := dnsZoneActuator{osClient: client}

			obj := &orcv1alpha1.DNSZone{
				Spec: orcv1alpha1.DNSZoneSpec{
					Resource: &tc.resourceSpec,
				},
			}

			iter, ok := actuator.ListOSResourcesForAdoption(ctx, obj)
			if !ok {
				t.Fatalf("Expected ok to be true")
			}

			var results []zoneResult
			for zone, err := range iter {
				results = append(results, zoneResult{zone, err})
			}

			if len(results) != tc.expectCount {
				t.Errorf("Expected %d results, got %d", tc.expectCount, len(results))
			}

			for i, id := range tc.expectIDs {
				if i < len(results) && results[i].zone.ID != id {
					t.Errorf("Expected ID %s, got %s", id, results[i].zone.ID)
				}
			}
		})
	}
}

func TestListOSResourcesForImport(t *testing.T) {
	for _, tc := range [...]struct {
		name        string
		filter      orcv1alpha1.DNSZoneFilter
		zones       []zones.Zone
		expectCount int
		expectIDs   []string
	}{
		{
			name: "match name and email",
			filter: orcv1alpha1.DNSZoneFilter{
				Name:  ptr.To[orcv1alpha1.OpenStackName]("example.com."),
				Email: ptr.To("admin@example.com"),
			},
			zones: []zones.Zone{
				{ID: "1", Name: "example.com.", Email: "admin@example.com"},
				{ID: "2", Name: "example.com.", Email: "other@example.com"},
			},
			expectCount: 1,
			expectIDs:   []string{"1"},
		},
		{
			name: "match TTL and Type",
			filter: orcv1alpha1.DNSZoneFilter{
				TTL:  ptr.To[int32](1800),
				Type: ptr.To(orcv1alpha1.DNSZoneTypePrimary),
			},
			zones: []zones.Zone{
				{ID: "1", Name: "example.com.", TTL: 1800, Type: "PRIMARY"},
				{ID: "2", Name: "example.com.", TTL: 3600, Type: "PRIMARY"},
				{ID: "3", Name: "example.com.", TTL: 1800, Type: "SECONDARY"},
			},
			expectCount: 1,
			expectIDs:   []string{"1"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			client := mockDNSZoneClient{zones: tc.zones}
			actuator := dnsZoneActuator{osClient: client}

			iter, status := actuator.ListOSResourcesForImport(ctx, &orcv1alpha1.DNSZone{}, tc.filter)
			if status != nil {
				t.Fatalf("Expected nil status, got %v", status)
			}

			var results []zoneResult
			for zone, err := range iter {
				results = append(results, zoneResult{zone, err})
			}

			if len(results) != tc.expectCount {
				t.Errorf("Expected %d results, got %d", tc.expectCount, len(results))
			}

			for i, id := range tc.expectIDs {
				if i < len(results) && results[i].zone.ID != id {
					t.Errorf("Expected ID %s, got %s", id, results[i].zone.ID)
				}
			}
		})
	}
}

func TestCreateResource(t *testing.T) {
	ctx := context.Background()

	// Case 1: Success
	client := mockDNSZoneClient{
		createFn: func(ctx context.Context, opts zones.CreateOptsBuilder) (*zones.Zone, error) {
			createOpts := opts.(zones.CreateOpts)
			return &zones.Zone{
				ID:          "created-id",
				Name:        createOpts.Name,
				Email:       createOpts.Email,
				Description: createOpts.Description,
				TTL:         createOpts.TTL,
				Type:        createOpts.Type,
			}, nil
		},
	}
	actuator := dnsZoneActuator{osClient: client}
	obj := &orcv1alpha1.DNSZone{
		Spec: orcv1alpha1.DNSZoneSpec{
			Resource: &orcv1alpha1.DNSZoneResourceSpec{
				Name:        ptr.To[orcv1alpha1.OpenStackName]("example.com."),
				Email:       "admin@example.com",
				Description: ptr.To("desc"),
				TTL:         ptr.To[int32](3600),
				Type:        orcv1alpha1.DNSZoneTypePrimary,
			},
		},
	}

	res, status := actuator.CreateResource(ctx, obj)
	if status != nil {
		t.Fatalf("Expected nil status, got %v", status)
	}
	if res.ID != "created-id" || res.Name != "example.com." || res.Email != "admin@example.com" || res.Description != "desc" || res.TTL != 3600 || res.Type != "PRIMARY" {
		t.Errorf("Created resource does not match: %v", res)
	}

	// Case 2: Conflict (already exists)
	conflictErr := gophercloud.ErrUnexpectedResponseCode{
		URL:      "http://designate/zones",
		Method:   "POST",
		Expected: []int{201},
		Actual:   409,
		Body:     []byte(`{"message": "Zone already exists"}`),
	}
	clientConflict := mockDNSZoneClient{
		createFn: func(ctx context.Context, opts zones.CreateOptsBuilder) (*zones.Zone, error) {
			return nil, conflictErr
		},
	}
	actuatorConflict := dnsZoneActuator{osClient: clientConflict}

	_, status = actuatorConflict.CreateResource(ctx, obj)
	if status == nil {
		t.Fatalf("Expected non-nil status on conflict")
	}
	needsReschedule, err := status.NeedsReschedule()
	if !needsReschedule {
		t.Errorf("Expected needsReschedule on error")
	}
	if err == nil {
		t.Errorf("Expected error from status, got nil")
	}
	if !orcerrors.IsConflict(err) {
		t.Errorf("Expected conflict error, got %v", err)
	}
	if orcerrors.IsRetryable(err) {
		t.Errorf("Expected conflict error to be terminal (not retryable)")
	}
}

func TestDeleteResource(t *testing.T) {
	ctx := context.Background()
	var deletedID string
	client := mockDNSZoneClient{
		deleteFn: func(ctx context.Context, id string) error {
			deletedID = id
			return nil
		},
	}
	actuator := dnsZoneActuator{osClient: client}
	zone := &zones.Zone{ID: "delete-me"}

	status := actuator.DeleteResource(ctx, &orcv1alpha1.DNSZone{}, zone)
	if status != nil {
		t.Errorf("Expected nil status, got %v", status)
	}
	if deletedID != "delete-me" {
		t.Errorf("Expected delete-me to be deleted, got %s", deletedID)
	}
}

func TestUpdateResource(t *testing.T) {
	ctx := context.Background()

	var updatedOpts zones.UpdateOpts
	client := mockDNSZoneClient{
		updateFn: func(ctx context.Context, id string, opts zones.UpdateOptsBuilder) (*zones.Zone, error) {
			updatedOpts = opts.(zones.UpdateOpts)
			return &zones.Zone{ID: id}, nil
		},
	}
	actuator := dnsZoneActuator{osClient: client}

	obj := &orcv1alpha1.DNSZone{
		Spec: orcv1alpha1.DNSZoneSpec{
			Resource: &orcv1alpha1.DNSZoneResourceSpec{
				Name:        ptr.To[orcv1alpha1.OpenStackName]("example.com."),
				Email:       "new-admin@example.com",
				Description: ptr.To("new-desc"),
				TTL:         ptr.To[int32](7200),
				Type:        orcv1alpha1.DNSZoneTypePrimary,
			},
		},
	}
	osResource := &zones.Zone{
		ID:          "zone-id",
		Name:        "example.com.",
		Email:       "admin@example.com",
		Description: "desc",
		TTL:         3600,
		Type:        "PRIMARY",
	}

	status := actuator.updateResource(ctx, obj, osResource)
	if status == nil {
		t.Fatalf("Expected progress status, got nil")
	}

	if updatedOpts.Email != "new-admin@example.com" {
		t.Errorf("Expected email new-admin@example.com, got %s", updatedOpts.Email)
	}
	if ptr.Deref(updatedOpts.Description, "") != "new-desc" {
		t.Errorf("Expected description new-desc, got %s", ptr.Deref(updatedOpts.Description, ""))
	}
	if updatedOpts.TTL != 7200 {
		t.Errorf("Expected TTL 7200, got %d", updatedOpts.TTL)
	}
}

func TestNeedsUpdate(t *testing.T) {
	testCases := []struct {
		name         string
		updateOpts   zones.UpdateOpts
		expectChange bool
	}{
		{
			name:         "Empty base opts",
			updateOpts:   zones.UpdateOpts{},
			expectChange: false,
		},
		{
			name:         "Updated opts",
			updateOpts:   zones.UpdateOpts{Description: ptr.To("updated")},
			expectChange: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := needsUpdate(tt.updateOpts)
			if got != tt.expectChange {
				t.Errorf("Expected change: %v, got: %v", tt.expectChange, got)
			}
		})
	}
}

func TestHandleDescriptionUpdate(t *testing.T) {
	ptrToDescription := ptr.To[string]
	testCases := []struct {
		name          string
		newValue      *string
		existingValue string
		expectChange  bool
	}{
		{name: "Identical", newValue: ptrToDescription("desc"), existingValue: "desc", expectChange: false},
		{name: "Different", newValue: ptrToDescription("new-desc"), existingValue: "desc", expectChange: true},
		{name: "No value provided, existing is set", newValue: nil, existingValue: "desc", expectChange: true},
		{name: "No value provided, existing is empty", newValue: nil, existingValue: "", expectChange: false},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			resource := &orcv1alpha1.DNSZoneResourceSpec{Description: tt.newValue}
			osResource := &osResourceT{Description: tt.existingValue}

			updateOpts := zones.UpdateOpts{}
			handleDescriptionUpdate(&updateOpts, resource, osResource)

			got, _ := needsUpdate(updateOpts)
			if got != tt.expectChange {
				t.Errorf("Expected change: %v, got: %v", tt.expectChange, got)
			}
		})

	}
}
