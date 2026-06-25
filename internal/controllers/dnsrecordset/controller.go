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

	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud/v2/openstack/dns/v2/recordsets"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	orcv1alpha1 "github.com/k-orc/openstack-resource-controller/v2/api/v1alpha1"
	"github.com/k-orc/openstack-resource-controller/v2/internal/controllers/generic/interfaces"
	"github.com/k-orc/openstack-resource-controller/v2/internal/controllers/generic/progress"
	"github.com/k-orc/openstack-resource-controller/v2/internal/controllers/generic/reconciler"
	"github.com/k-orc/openstack-resource-controller/v2/internal/scope"
	"github.com/k-orc/openstack-resource-controller/v2/internal/util/credentials"
	"github.com/k-orc/openstack-resource-controller/v2/internal/util/dependency"
	orcapplyconfigv1alpha1 "github.com/k-orc/openstack-resource-controller/v2/pkg/clients/applyconfiguration/api/v1alpha1"
	"github.com/k-orc/openstack-resource-controller/v2/pkg/predicates"
)

const controllerName = "dnsrecordset"

// +kubebuilder:rbac:groups=openstack.k-orc.cloud,resources=dnsrecordsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=openstack.k-orc.cloud,resources=dnsrecordsets/status,verbs=get;update;patch

type dnsrecordsetReconcilerConstructor struct {
	scopeFactory scope.Factory
}

func New(scopeFactory scope.Factory) interfaces.Controller {
	return dnsrecordsetReconcilerConstructor{scopeFactory: scopeFactory}
}

func (dnsrecordsetReconcilerConstructor) GetName() string {
	return controllerName
}

var dnsZoneDependency = dependency.NewDeletionGuardDependency[*orcObjectListT, *orcv1alpha1.DNSZone](
	"spec.resource.dnsZoneRef",
	func(obj orcObjectPT) []string {
		resource := obj.Spec.Resource
		if resource == nil {
			return nil
		}
		return []string{string(resource.DNSZoneRef)}
	},
	finalizer, externalObjectFieldOwner,
)

// SetupWithManager sets up the controller with the Manager.
func (c dnsrecordsetReconcilerConstructor) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := ctrl.LoggerFrom(ctx)
	k8sClient := mgr.GetClient()

	dnsZoneWatchEventHandler, err := dnsZoneDependency.WatchEventHandler(log, k8sClient)
	if err != nil {
		return err
	}

	builder := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&orcv1alpha1.DNSRecordset{}).
		Watches(&orcv1alpha1.DNSZone{}, dnsZoneWatchEventHandler,
			builder.WithPredicates(predicates.NewBecameAvailable(log, &orcv1alpha1.DNSZone{})),
		)

	if err := errors.Join(
		dnsZoneDependency.AddToManager(ctx, mgr),
		credentialsDependency.AddToManager(ctx, mgr),
		credentials.AddCredentialsWatch(log, k8sClient, builder, credentialsDependency),
	); err != nil {
		return err
	}

	r := reconciler.NewController(controllerName, k8sClient, c.scopeFactory, dnsRecordsetHelperFactory{}, dnsRecordsetStatusWriter{})
	return builder.Complete(&r)
}

type objectApplyT = orcapplyconfigv1alpha1.DNSRecordsetApplyConfiguration
type statusApplyT = orcapplyconfigv1alpha1.DNSRecordsetStatusApplyConfiguration
type osResourceT = recordsets.RecordSet

type dnsRecordsetStatusWriter struct{}

var _ interfaces.ResourceStatusWriter[orcObjectPT, *osResourceT, *objectApplyT, *statusApplyT] = dnsRecordsetStatusWriter{}

func (dnsRecordsetStatusWriter) GetApplyConfig(name, namespace string) *objectApplyT {
	return orcapplyconfigv1alpha1.DNSRecordset(name, namespace)
}

func (dnsRecordsetStatusWriter) ResourceAvailableStatus(orcObject orcObjectPT, osResource *osResourceT) (metav1.ConditionStatus, progress.ReconcileStatus) {
	return metav1.ConditionFalse, nil
}

func (dnsRecordsetStatusWriter) ApplyResourceStatus(log logr.Logger, osResource *osResourceT, statusApply *statusApplyT) {
}

type dnsRecordsetHelperFactory struct{}

var _ interfaces.ResourceHelperFactory[orcObjectPT, orcObjectT, resourceSpecT, filterT, osResourceT] = dnsRecordsetHelperFactory{}

func (dnsRecordsetHelperFactory) NewAPIObjectAdapter(obj orcObjectPT) interfaces.APIObjectAdapter[orcObjectPT, resourceSpecT, filterT] {
	return dnsrecordsetAdapter{obj}
}

func (dnsRecordsetHelperFactory) NewCreateActuator(ctx context.Context, orcObject orcObjectPT, controller interfaces.ResourceController) (interfaces.CreateResourceActuator[orcObjectPT, orcObjectT, filterT, osResourceT], progress.ReconcileStatus) {
	return dnsRecordsetActuator{}, nil
}

func (dnsRecordsetHelperFactory) NewDeleteActuator(ctx context.Context, orcObject orcObjectPT, controller interfaces.ResourceController) (interfaces.DeleteResourceActuator[orcObjectPT, orcObjectT, osResourceT], progress.ReconcileStatus) {
	return dnsRecordsetActuator{}, nil
}

type dnsRecordsetActuator struct{}

var _ interfaces.CreateResourceActuator[orcObjectPT, orcObjectT, filterT, osResourceT] = dnsRecordsetActuator{}
var _ interfaces.DeleteResourceActuator[orcObjectPT, orcObjectT, osResourceT] = dnsRecordsetActuator{}

func (dnsRecordsetActuator) GetResourceID(osResource *osResourceT) string {
	return osResource.ID
}

func (dnsRecordsetActuator) GetOSResourceByID(ctx context.Context, id string) (*osResourceT, progress.ReconcileStatus) {
	return nil, nil
}

func (dnsRecordsetActuator) ListOSResourcesForAdoption(ctx context.Context, orcObject orcObjectPT) (iter.Seq2[*osResourceT, error], bool) {
	return nil, false
}

func (dnsRecordsetActuator) ListOSResourcesForImport(ctx context.Context, orcObject orcObjectPT, filter filterT) (iter.Seq2[*osResourceT, error], progress.ReconcileStatus) {
	return nil, nil
}

func (dnsRecordsetActuator) CreateResource(ctx context.Context, orcObject orcObjectPT) (*osResourceT, progress.ReconcileStatus) {
	return nil, nil
}

func (dnsRecordsetActuator) DeleteResource(ctx context.Context, orcObject orcObjectPT, osResource *osResourceT) progress.ReconcileStatus {
	return nil
}
