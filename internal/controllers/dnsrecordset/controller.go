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

	"github.com/gophercloud/gophercloud/v2/openstack/dns/v2/recordsets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	orcv1alpha1 "github.com/k-orc/openstack-resource-controller/v2/api/v1alpha1"
	"github.com/k-orc/openstack-resource-controller/v2/internal/controllers/generic/interfaces"
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

var dnsZoneImportDependency = dependency.NewDependency[*orcObjectListT, *orcv1alpha1.DNSZone](
	"spec.import.filter.dnsZoneRef",
	func(obj orcObjectPT) []string {
		resource := obj.Spec.Import
		if resource == nil || resource.Filter == nil {
			return nil
		}
		return []string{string(resource.Filter.DNSZoneRef)}
	},
)

// SetupWithManager sets up the controller with the Manager.
func (c dnsrecordsetReconcilerConstructor) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := ctrl.LoggerFrom(ctx)
	k8sClient := mgr.GetClient()

	dnsZoneWatchEventHandler, err := dnsZoneDependency.WatchEventHandler(log, k8sClient)
	if err != nil {
		return err
	}

	dnsZoneImportWatchEventHandler, err := dnsZoneImportDependency.WatchEventHandler(log, k8sClient)
	if err != nil {
		return err
	}

	builder := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&orcv1alpha1.DNSRecordset{}).
		Watches(&orcv1alpha1.DNSZone{}, dnsZoneWatchEventHandler,
			builder.WithPredicates(predicates.NewBecameAvailable(log, &orcv1alpha1.DNSZone{})),
		).
		Watches(&orcv1alpha1.DNSZone{}, dnsZoneImportWatchEventHandler,
			builder.WithPredicates(predicates.NewBecameAvailable(log, &orcv1alpha1.DNSZone{})),
		)

	if err := errors.Join(
		dnsZoneDependency.AddToManager(ctx, mgr),
		dnsZoneImportDependency.AddToManager(ctx, mgr),
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
