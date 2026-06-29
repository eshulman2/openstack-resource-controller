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

package apivalidations

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	orcv1alpha1 "github.com/k-orc/openstack-resource-controller/v2/api/v1alpha1"
	applyconfigv1alpha1 "github.com/k-orc/openstack-resource-controller/v2/pkg/clients/applyconfiguration/api/v1alpha1"
)

const (
	dnsrecordsetName = "dnsrecordset"
)

func dnsrecordsetStub(namespace *corev1.Namespace) *orcv1alpha1.DNSRecordset {
	obj := &orcv1alpha1.DNSRecordset{}
	obj.Name = dnsrecordsetName
	obj.Namespace = namespace.Name
	return obj
}

func testDNSRecordsetResource() *applyconfigv1alpha1.DNSRecordsetResourceSpecApplyConfiguration {
	return applyconfigv1alpha1.DNSRecordsetResourceSpec().
		WithType("A").
		WithRecords("192.0.2.1").
		WithDNSZoneRef("my-zone")
}

func baseDNSRecordsetPatch(obj client.Object) *applyconfigv1alpha1.DNSRecordsetApplyConfiguration {
	return applyconfigv1alpha1.DNSRecordset(obj.GetName(), obj.GetNamespace()).
		WithSpec(applyconfigv1alpha1.DNSRecordsetSpec().
			WithCloudCredentialsRef(testCredentials()))
}

func testDNSRecordsetImport() *applyconfigv1alpha1.DNSRecordsetImportApplyConfiguration {
	return applyconfigv1alpha1.DNSRecordsetImport().WithFilter(applyconfigv1alpha1.DNSRecordsetFilter().WithName("foo.").WithDNSZoneRef("my-zone"))
}

var _ = Describe("ORC DNSRecordset API validations", func() {
	var namespace *corev1.Namespace
	BeforeEach(func() {
		namespace = createNamespace()
	})

	runManagementPolicyTests(func() *corev1.Namespace { return namespace }, managementPolicyTestArgs[*applyconfigv1alpha1.DNSRecordsetApplyConfiguration]{
		createObject: func(ns *corev1.Namespace) client.Object { return dnsrecordsetStub(ns) },
		basePatch: func(obj client.Object) *applyconfigv1alpha1.DNSRecordsetApplyConfiguration {
			return baseDNSRecordsetPatch(obj)
		},
		applyResource: func(p *applyconfigv1alpha1.DNSRecordsetApplyConfiguration) {
			p.Spec.WithResource(testDNSRecordsetResource())
		},
		applyImport: func(p *applyconfigv1alpha1.DNSRecordsetApplyConfiguration) {
			p.Spec.WithImport(testDNSRecordsetImport())
		},
		applyEmptyImport: func(p *applyconfigv1alpha1.DNSRecordsetApplyConfiguration) {
			p.Spec.WithImport(applyconfigv1alpha1.DNSRecordsetImport())
		},
		applyEmptyFilter: func(p *applyconfigv1alpha1.DNSRecordsetApplyConfiguration) {
			p.Spec.WithImport(applyconfigv1alpha1.DNSRecordsetImport().WithFilter(applyconfigv1alpha1.DNSRecordsetFilter()))
		},
		applyValidFilter: func(p *applyconfigv1alpha1.DNSRecordsetApplyConfiguration) {
			p.Spec.WithImport(applyconfigv1alpha1.DNSRecordsetImport().WithFilter(applyconfigv1alpha1.DNSRecordsetFilter().WithName("foo.").WithDNSZoneRef("my-zone")))
		},
		applyManaged: func(p *applyconfigv1alpha1.DNSRecordsetApplyConfiguration) {
			p.Spec.WithManagementPolicy(orcv1alpha1.ManagementPolicyManaged)
		},
		applyUnmanaged: func(p *applyconfigv1alpha1.DNSRecordsetApplyConfiguration) {
			p.Spec.WithManagementPolicy(orcv1alpha1.ManagementPolicyUnmanaged)
		},
		applyManagedOptions: func(p *applyconfigv1alpha1.DNSRecordsetApplyConfiguration) {
			p.Spec.WithManagedOptions(applyconfigv1alpha1.ManagedOptions().WithOnDelete(orcv1alpha1.OnDeleteDetach))
		},
		getManagementPolicy: func(obj client.Object) orcv1alpha1.ManagementPolicy {
			return obj.(*orcv1alpha1.DNSRecordset).Spec.ManagementPolicy
		},
		getOnDelete: func(obj client.Object) orcv1alpha1.OnDelete {
			return obj.(*orcv1alpha1.DNSRecordset).Spec.ManagedOptions.OnDelete
		},
	})

	It("should reject name not ending with a period", func(ctx context.Context) {
		dnsrecordset := dnsrecordsetStub(namespace)
		patch := baseDNSRecordsetPatch(dnsrecordset)
		patch.Spec.WithResource(testDNSRecordsetResource().WithName("invalid-name"))
		Expect(applyObj(ctx, dnsrecordset, patch)).To(MatchError(ContainSubstring("name must end with a period")))
	})

	It("should accept name ending with a period", func(ctx context.Context) {
		dnsrecordset := dnsrecordsetStub(namespace)
		patch := baseDNSRecordsetPatch(dnsrecordset)
		patch.Spec.WithResource(testDNSRecordsetResource().WithName("valid-name."))
		Expect(applyObj(ctx, dnsrecordset, patch)).To(Succeed())
	})

	It("should reject TTL outside valid range", func(ctx context.Context) {
		dnsrecordset := dnsrecordsetStub(namespace)

		patch := baseDNSRecordsetPatch(dnsrecordset)
		patch.Spec.WithResource(testDNSRecordsetResource().WithTTL(0))
		Expect(applyObj(ctx, dnsrecordset, patch)).To(MatchError(ContainSubstring("spec.resource.ttl in body should be greater than or equal to 1")))
	})

	It("should enforce name immutability", func(ctx context.Context) {
		dnsrecordset := dnsrecordsetStub(namespace)
		patch := baseDNSRecordsetPatch(dnsrecordset)
		patch.Spec.WithResource(testDNSRecordsetResource().WithName("original-name."))
		Expect(applyObj(ctx, dnsrecordset, patch)).To(Succeed())

		patch = baseDNSRecordsetPatch(dnsrecordset)
		patch.Spec.WithResource(testDNSRecordsetResource().WithName("updated-name."))
		Expect(applyObj(ctx, dnsrecordset, patch)).To(MatchError(ContainSubstring("name is immutable")))
	})

	It("should enforce type immutability", func(ctx context.Context) {
		dnsrecordset := dnsrecordsetStub(namespace)
		patch := baseDNSRecordsetPatch(dnsrecordset)
		patch.Spec.WithResource(testDNSRecordsetResource().WithType("A"))
		Expect(applyObj(ctx, dnsrecordset, patch)).To(Succeed())

		patch = baseDNSRecordsetPatch(dnsrecordset)
		patch.Spec.WithResource(testDNSRecordsetResource().WithType("AAAA"))
		Expect(applyObj(ctx, dnsrecordset, patch)).To(MatchError(ContainSubstring("type is immutable")))
	})

	It("should enforce dnsZoneRef immutability", func(ctx context.Context) {
		dnsrecordset := dnsrecordsetStub(namespace)
		patch := baseDNSRecordsetPatch(dnsrecordset)
		patch.Spec.WithResource(testDNSRecordsetResource().WithDNSZoneRef("original-zone"))
		Expect(applyObj(ctx, dnsrecordset, patch)).To(Succeed())

		patch = baseDNSRecordsetPatch(dnsrecordset)
		patch.Spec.WithResource(testDNSRecordsetResource().WithDNSZoneRef("updated-zone"))
		Expect(applyObj(ctx, dnsrecordset, patch)).To(MatchError(ContainSubstring("dnsZoneRef is immutable")))
	})
})
