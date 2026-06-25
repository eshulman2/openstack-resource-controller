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
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	orcv1alpha1 "github.com/k-orc/openstack-resource-controller/v2/api/v1alpha1"
	"github.com/k-orc/openstack-resource-controller/v2/internal/controllers/generic/interfaces"
	"github.com/k-orc/openstack-resource-controller/v2/internal/controllers/generic/progress"
	orcerrors "github.com/k-orc/openstack-resource-controller/v2/internal/util/errors"
	orcapplyconfigv1alpha1 "github.com/k-orc/openstack-resource-controller/v2/pkg/clients/applyconfiguration/api/v1alpha1"
)

const (
	RecordsetStatusActive  = "ACTIVE"
	RecordsetStatusPending = "PENDING"
	RecordsetStatusError   = "ERROR"

	// The time to wait before reconciling again when we are expecting OpenStack to finish some task and update status.
	externalUpdatePollingPeriod = 15 * time.Second
)

type dnsRecordsetStatusWriter struct{}

var _ interfaces.ResourceStatusWriter[orcObjectPT, *osResourceT, *objectApplyT, *statusApplyT] = dnsRecordsetStatusWriter{}

func (dnsRecordsetStatusWriter) GetApplyConfig(name, namespace string) *objectApplyT {
	return orcapplyconfigv1alpha1.DNSRecordset(name, namespace)
}

func (dnsRecordsetStatusWriter) ResourceAvailableStatus(orcObject orcObjectPT, osResource *osResourceT) (metav1.ConditionStatus, progress.ReconcileStatus) {
	if osResource == nil {
		if orcObject.Status.ID == nil {
			return metav1.ConditionFalse, nil
		} else {
			return metav1.ConditionUnknown, nil
		}
	}

	switch osResource.Status {
	case RecordsetStatusActive:
		return metav1.ConditionTrue, nil
	case RecordsetStatusPending:
		return metav1.ConditionFalse, progress.WaitingOnOpenStack(progress.WaitingOnReady, externalUpdatePollingPeriod)
	case RecordsetStatusError:
		return metav1.ConditionFalse, progress.WrapError(
			orcerrors.Terminal(orcv1alpha1.ConditionReasonUnrecoverableError, "OpenStack recordset is in ERROR status"))
	default:
		// Fallback for any other/unexpected status
		return metav1.ConditionFalse, progress.WaitingOnOpenStack(progress.WaitingOnReady, externalUpdatePollingPeriod)
	}
}

func (dnsRecordsetStatusWriter) ApplyResourceStatus(log logr.Logger, osResource *osResourceT, statusApply *statusApplyT) {
	resourceStatus := orcapplyconfigv1alpha1.DNSRecordsetResourceStatus().
		WithName(osResource.Name)

	if osResource.Type != "" {
		resourceStatus.WithType(osResource.Type)
	}

	if len(osResource.Records) > 0 {
		resourceStatus.WithRecords(osResource.Records...)
	}

	if osResource.TTL > 0 {
		resourceStatus.WithTTL(int32(osResource.TTL))
	}

	if osResource.Description != "" {
		resourceStatus.WithDescription(osResource.Description)
	}

	if osResource.Status != "" {
		resourceStatus.WithStatus(osResource.Status)
	}

	statusApply.WithResource(resourceStatus)
}
