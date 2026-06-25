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
	"fmt"
	"iter"
	"slices"
	"strings"

	"github.com/gophercloud/gophercloud/v2/openstack/dns/v2/recordsets"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	orcv1alpha1 "github.com/k-orc/openstack-resource-controller/v2/api/v1alpha1"
	"github.com/k-orc/openstack-resource-controller/v2/internal/controllers/generic/interfaces"
	"github.com/k-orc/openstack-resource-controller/v2/internal/controllers/generic/progress"
	"github.com/k-orc/openstack-resource-controller/v2/internal/osclients"
	orcerrors "github.com/k-orc/openstack-resource-controller/v2/internal/util/errors"
)

type (
	createResourceActuator = interfaces.CreateResourceActuator[orcObjectPT, orcObjectT, filterT, osResourceT]
	deleteResourceActuator = interfaces.DeleteResourceActuator[orcObjectPT, orcObjectT, osResourceT]
	helperFactory          = interfaces.ResourceHelperFactory[orcObjectPT, orcObjectT, resourceSpecT, filterT, osResourceT]
)

type dnsRecordsetActuator struct {
	osClient  osclients.DNSRecordsetClient
	k8sClient client.Client
	zoneID    string
	orcObject orcObjectPT
}

var _ createResourceActuator = dnsRecordsetActuator{}
var _ deleteResourceActuator = dnsRecordsetActuator{}

func (dnsRecordsetActuator) GetResourceID(osResource *osResourceT) string {
	return osResource.ID
}

func (actuator dnsRecordsetActuator) GetOSResourceByID(ctx context.Context, id string) (*osResourceT, progress.ReconcileStatus) {
	if actuator.zoneID == "" {
		return nil, progress.WaitingOnObject("DNSZone", string(actuator.orcObject.Spec.Resource.DNSZoneRef), progress.WaitingOnReady)
	}
	resource, err := actuator.osClient.GetRecordset(ctx, actuator.zoneID, id)
	if err != nil {
		return nil, progress.WrapError(err)
	}
	return resource, nil
}

func (actuator dnsRecordsetActuator) ListOSResourcesForAdoption(ctx context.Context, orcObject orcObjectPT) (iter.Seq2[*osResourceT, error], bool) {
	resourceSpec := orcObject.Spec.Resource
	if resourceSpec == nil {
		return nil, false
	}

	if actuator.zoneID == "" {
		return nil, false
	}

	listOpts := recordsets.ListOpts{
		Name: getDNSRecordsetName(orcObject),
		Type: resourceSpec.Type,
	}

	recordsetsSeq := actuator.osClient.ListRecordsets(ctx, actuator.zoneID, listOpts)

	adoptionSeq := func(yield func(*osResourceT, error) bool) {
		for f, err := range recordsetsSeq {
			if err != nil {
				yield(nil, err)
				return
			}

			if namesMatch(f.Name, getDNSRecordsetName(orcObject)) && strings.EqualFold(f.Type, resourceSpec.Type) {
				matches := true
				var mismatchMsg string

				if !recordsMatch(f.Records, resourceSpec.Records) {
					matches = false
					mismatchMsg = fmt.Sprintf("records mismatch: OpenStack has %v, spec has %v", f.Records, resourceSpec.Records)
				} else if resourceSpec.TTL != nil && f.TTL != int(*resourceSpec.TTL) {
					matches = false
					mismatchMsg = fmt.Sprintf("TTL mismatch: OpenStack has %d, spec has %d", f.TTL, *resourceSpec.TTL)
				} else if resourceSpec.Description != nil && f.Description != *resourceSpec.Description {
					matches = false
					mismatchMsg = fmt.Sprintf("description mismatch: OpenStack has %q, spec has %q", f.Description, *resourceSpec.Description)
				}

				if !matches {
					err := orcerrors.Terminal(
						orcv1alpha1.ConditionReasonUnrecoverableError,
						fmt.Sprintf("duplicate recordset found but properties mismatch: %s", mismatchMsg),
					)
					yield(nil, err)
					return
				}

				if !yield(f, nil) {
					return
				}
			}
		}
	}

	return adoptionSeq, true
}

func (actuator dnsRecordsetActuator) ListOSResourcesForImport(ctx context.Context, orcObject orcObjectPT, filter filterT) (iter.Seq2[*osResourceT, error], progress.ReconcileStatus) {
	if actuator.zoneID == "" {
		return nil, progress.WaitingOnObject("DNSZone", string(orcObject.Spec.Resource.DNSZoneRef), progress.WaitingOnReady)
	}

	var filters []osclients.ResourceFilter[osResourceT]

	if filter.Name != nil {
		filters = append(filters, func(f *osResourceT) bool { return namesMatch(f.Name, string(*filter.Name)) })
	}
	if filter.Type != nil {
		filters = append(filters, func(f *osResourceT) bool { return strings.EqualFold(f.Type, *filter.Type) })
	}
	if filter.TTL != nil {
		filters = append(filters, func(f *osResourceT) bool { return f.TTL == int(*filter.TTL) })
	}
	if filter.Description != nil {
		filters = append(filters, func(f *osResourceT) bool { return f.Description == *filter.Description })
	}

	listOpts := recordsets.ListOpts{}
	if filter.Name != nil {
		listOpts.Name = string(*filter.Name)
	}
	if filter.Type != nil {
		listOpts.Type = *filter.Type
	}

	recordsetsSeq := actuator.osClient.ListRecordsets(ctx, actuator.zoneID, listOpts)
	return osclients.Filter(recordsetsSeq, filters...), nil
}

func (actuator dnsRecordsetActuator) CreateResource(ctx context.Context, obj orcObjectPT) (*osResourceT, progress.ReconcileStatus) {
	resource := obj.Spec.Resource

	if resource == nil {
		return nil, progress.WrapError(
			orcerrors.Terminal(orcv1alpha1.ConditionReasonInvalidConfiguration, "Creation requested, but spec.resource is not set"))
	}

	if actuator.zoneID == "" {
		return nil, progress.WaitingOnObject("DNSZone", string(resource.DNSZoneRef), progress.WaitingOnReady)
	}

	createOpts := recordsets.CreateOpts{
		Name:        getDNSRecordsetName(obj),
		Type:        resource.Type,
		Records:     resource.Records,
		Description: ptr.Deref(resource.Description, ""),
	}
	if resource.TTL != nil {
		createOpts.TTL = int(*resource.TTL)
	}

	osResource, err := actuator.osClient.CreateRecordset(ctx, actuator.zoneID, createOpts)
	if err != nil {
		if !orcerrors.IsRetryable(err) {
			reason := orcv1alpha1.ConditionReasonInvalidConfiguration
			if orcerrors.IsConflict(err) {
				reason = orcv1alpha1.ConditionReasonUnrecoverableError
			}
			err = orcerrors.Terminal(reason, "invalid configuration creating resource: "+err.Error(), err)
		}
		return nil, progress.WrapError(err)
	}

	return osResource, nil
}

func (actuator dnsRecordsetActuator) DeleteResource(ctx context.Context, _ orcObjectPT, resource *osResourceT) progress.ReconcileStatus {
	if actuator.zoneID == "" {
		return progress.WaitingOnObject("DNSZone", string(actuator.orcObject.Spec.Resource.DNSZoneRef), progress.WaitingOnReady)
	}
	return progress.WrapError(actuator.osClient.DeleteRecordset(ctx, actuator.zoneID, resource.ID))
}

type dnsRecordsetHelperFactory struct{}

var _ helperFactory = dnsRecordsetHelperFactory{}

func newActuator(ctx context.Context, orcObject orcObjectPT, controller interfaces.ResourceController) (dnsRecordsetActuator, progress.ReconcileStatus) {
	log := ctrl.LoggerFrom(ctx)

	_, reconcileStatus := credentialsDependency.GetDependencies(ctx, controller.GetK8sClient(), orcObject, func(*corev1.Secret) bool { return true })
	if needsReschedule, _ := reconcileStatus.NeedsReschedule(); needsReschedule {
		return dnsRecordsetActuator{}, reconcileStatus
	}

	clientScope, err := controller.GetScopeFactory().NewClientScopeFromObject(ctx, controller.GetK8sClient(), log, orcObject)
	if err != nil {
		return dnsRecordsetActuator{}, progress.WrapError(err)
	}
	osClient, err := clientScope.NewDNSRecordsetClient()
	if err != nil {
		return dnsRecordsetActuator{}, progress.WrapError(err)
	}

	dnsZone, reconcileStatus := dnsZoneDependency.GetDependency(
		ctx, controller.GetK8sClient(), orcObject,
		func(dep *orcv1alpha1.DNSZone) bool {
			return orcv1alpha1.IsAvailable(dep) && dep.Status.ID != nil && *dep.Status.ID != ""
		},
	)
	if needsReschedule, _ := reconcileStatus.NeedsReschedule(); needsReschedule {
		return dnsRecordsetActuator{}, reconcileStatus
	}

	var zoneID string
	if dnsZone != nil && dnsZone.Status.ID != nil {
		zoneID = *dnsZone.Status.ID
	}

	return dnsRecordsetActuator{
		osClient:  osClient,
		k8sClient: controller.GetK8sClient(),
		zoneID:    zoneID,
		orcObject: orcObject,
	}, nil
}

func (dnsRecordsetHelperFactory) NewAPIObjectAdapter(obj orcObjectPT) interfaces.APIObjectAdapter[orcObjectPT, resourceSpecT, filterT] {
	return dnsrecordsetAdapter{obj}
}

func (dnsRecordsetHelperFactory) NewCreateActuator(ctx context.Context, orcObject orcObjectPT, controller interfaces.ResourceController) (interfaces.CreateResourceActuator[orcObjectPT, orcObjectT, filterT, osResourceT], progress.ReconcileStatus) {
	return newActuator(ctx, orcObject, controller)
}

func (dnsRecordsetHelperFactory) NewDeleteActuator(ctx context.Context, orcObject orcObjectPT, controller interfaces.ResourceController) (interfaces.DeleteResourceActuator[orcObjectPT, orcObjectT, osResourceT], progress.ReconcileStatus) {
	return newActuator(ctx, orcObject, controller)
}

func getDNSRecordsetName(orcObject orcObjectPT) string {
	name := getResourceName(orcObject)
	if name != "" && name[len(name)-1] != '.' {
		return name + "."
	}
	return name
}

func namesMatch(a, b string) bool {
	return strings.EqualFold(a, b)
}

func recordsMatch(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := make([]string, len(a))
	copy(ac, a)
	bc := make([]string, len(b))
	copy(bc, b)
	slices.Sort(ac)
	slices.Sort(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}
