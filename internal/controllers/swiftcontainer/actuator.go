/*
Copyright 2024 The ORC Authors.

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

package swiftcontainer

import (
	"context"
	"iter"

	"github.com/gophercloud/gophercloud/v2/openstack/objectstorage/v1/containers"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	orcv1alpha1 "github.com/k-orc/openstack-resource-controller/v2/api/v1alpha1"
	generic "github.com/k-orc/openstack-resource-controller/v2/internal/controllers/generic/interfaces"
	"github.com/k-orc/openstack-resource-controller/v2/internal/controllers/generic/progress"
	osclients "github.com/k-orc/openstack-resource-controller/v2/internal/osclients"
	orcerrors "github.com/k-orc/openstack-resource-controller/v2/internal/util/errors"
)

// osContainerT wraps containers.GetHeader with the container name, since
// GetHeader does not include the name of the container.
type osContainerT struct {
	Name string
	containers.GetHeader
}

// OpenStack resource types
type (
	osResourceT = osContainerT

	createResourceActuator = generic.CreateResourceActuator[orcObjectPT, orcObjectT, filterT, osResourceT]
	deleteResourceActuator = generic.DeleteResourceActuator[orcObjectPT, orcObjectT, osResourceT]
	helperFactory          = generic.ResourceHelperFactory[orcObjectPT, orcObjectT, resourceSpecT, filterT, osResourceT]
)

type swiftcontainerActuator struct {
	osClient osclients.SwiftContainerClient
}

var _ createResourceActuator = swiftcontainerActuator{}
var _ deleteResourceActuator = swiftcontainerActuator{}

func (swiftcontainerActuator) GetResourceID(osResource *osContainerT) string {
	// Swift containers are identified by name
	return osResource.Name
}

func (actuator swiftcontainerActuator) GetOSResourceByID(ctx context.Context, id string) (*osContainerT, progress.ReconcileStatus) {
	header, err := actuator.osClient.GetContainer(ctx, id)
	if err != nil {
		return nil, progress.WrapError(err)
	}
	return &osContainerT{Name: id, GetHeader: *header}, nil
}

func (actuator swiftcontainerActuator) ListOSResourcesForAdoption(ctx context.Context, orcObject orcObjectPT) (iter.Seq2[*osContainerT, error], bool) {
	resourceSpec := orcObject.Spec.Resource
	if resourceSpec == nil {
		return nil, false
	}

	name := getResourceName(orcObject)
	return func(yield func(*osContainerT, error) bool) {
		header, err := actuator.osClient.GetContainer(ctx, name)
		if err != nil {
			if !orcerrors.IsNotFound(err) {
				yield(nil, err)
			}
			return
		}
		yield(&osContainerT{Name: name, GetHeader: *header}, nil)
	}, true
}

func (actuator swiftcontainerActuator) ListOSResourcesForImport(ctx context.Context, _ orcObjectPT, filter filterT) (iter.Seq2[*osResourceT, error], progress.ReconcileStatus) {
	return func(yield func(*osContainerT, error) bool) {
		if filter.Name != nil {
			name := string(*filter.Name)
			header, err := actuator.osClient.GetContainer(ctx, name)
			if err != nil {
				if !orcerrors.IsNotFound(err) {
					yield(nil, err)
				}
				return
			}
			yield(&osContainerT{Name: name, GetHeader: *header}, nil)
		} else {
			// List all containers and filter by prefix
			listOpts := containers.ListOpts{}
			for container, err := range actuator.osClient.ListContainers(ctx, listOpts) {
				if err != nil {
					yield(nil, err)
					return
				}

				if filter.Prefix != nil && !hasPrefix(container.Name, *filter.Prefix) {
					continue
				}

				header, err := actuator.osClient.GetContainer(ctx, container.Name)
				if err != nil {
					yield(nil, err)
					return
				}
				if !yield(&osContainerT{Name: container.Name, GetHeader: *header}, nil) {
					return
				}
			}
		}
	}, nil
}

// hasPrefix checks if name starts with prefix.
func hasPrefix(name, prefix string) bool {
	if len(prefix) > len(name) {
		return false
	}
	return name[:len(prefix)] == prefix
}

func (actuator swiftcontainerActuator) CreateResource(ctx context.Context, obj orcObjectPT) (*osContainerT, progress.ReconcileStatus) {
	resource := obj.Spec.Resource

	if resource == nil {
		// Should have been caught by API validation
		return nil, progress.WrapError(
			orcerrors.Terminal(orcv1alpha1.ConditionReasonInvalidConfiguration, "Creation requested, but spec.resource is not set"))
	}

	name := getResourceName(obj)

	createOpts := containers.CreateOpts{}

	if resource.ContainerRead != nil {
		createOpts.ContainerRead = *resource.ContainerRead
	}
	if resource.ContainerWrite != nil {
		createOpts.ContainerWrite = *resource.ContainerWrite
	}
	if resource.StoragePolicy != nil {
		createOpts.StoragePolicy = *resource.StoragePolicy
	}

	if len(resource.Metadata) > 0 {
		metadata := make(map[string]string, len(resource.Metadata))
		for _, m := range resource.Metadata {
			metadata[m.Key] = m.Value
		}
		createOpts.Metadata = metadata
	}

	_, err := actuator.osClient.CreateContainer(ctx, name, createOpts)
	if err != nil {
		if !orcerrors.IsRetryable(err) {
			err = orcerrors.Terminal(orcv1alpha1.ConditionReasonInvalidConfiguration, "invalid configuration creating resource: "+err.Error(), err)
		}
		return nil, progress.WrapError(err)
	}

	// Fetch the created container to return its header
	header, err := actuator.osClient.GetContainer(ctx, name)
	if err != nil {
		return nil, progress.WrapError(err)
	}

	return &osContainerT{Name: name, GetHeader: *header}, nil
}

func (actuator swiftcontainerActuator) DeleteResource(ctx context.Context, orcObject orcObjectPT, _ *osContainerT) progress.ReconcileStatus {
	name := getResourceName(orcObject)
	_, err := actuator.osClient.DeleteContainer(ctx, name)
	return progress.WrapError(err)
}

type swiftcontainerHelperFactory struct{}

var _ helperFactory = swiftcontainerHelperFactory{}

func newActuator(ctx context.Context, orcObject *orcv1alpha1.SwiftContainer, controller generic.ResourceController) (swiftcontainerActuator, progress.ReconcileStatus) {
	log := ctrl.LoggerFrom(ctx)

	// Ensure credential secrets exist and have our finalizer
	_, reconcileStatus := credentialsDependency.GetDependencies(ctx, controller.GetK8sClient(), orcObject, func(*corev1.Secret) bool { return true })
	if needsReschedule, _ := reconcileStatus.NeedsReschedule(); needsReschedule {
		return swiftcontainerActuator{}, reconcileStatus
	}

	clientScope, err := controller.GetScopeFactory().NewClientScopeFromObject(ctx, controller.GetK8sClient(), log, orcObject)
	if err != nil {
		return swiftcontainerActuator{}, progress.WrapError(err)
	}
	osClient, err := clientScope.NewSwiftContainerClient()
	if err != nil {
		return swiftcontainerActuator{}, progress.WrapError(err)
	}

	return swiftcontainerActuator{
		osClient: osClient,
	}, nil
}

func (swiftcontainerHelperFactory) NewAPIObjectAdapter(obj orcObjectPT) adapterI {
	return swiftcontainerAdapter{obj}
}

func (swiftcontainerHelperFactory) NewCreateActuator(ctx context.Context, orcObject orcObjectPT, controller generic.ResourceController) (createResourceActuator, progress.ReconcileStatus) {
	return newActuator(ctx, orcObject, controller)
}

func (swiftcontainerHelperFactory) NewDeleteActuator(ctx context.Context, orcObject orcObjectPT, controller generic.ResourceController) (deleteResourceActuator, progress.ReconcileStatus) {
	return newActuator(ctx, orcObject, controller)
}
