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

package osclients

import (
	"context"
	"fmt"
	"iter"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/dns/v2/recordsets"
	"github.com/gophercloud/utils/v2/openstack/clientconfig"
)

type DNSRecordsetClient interface {
	ListRecordsets(ctx context.Context, zoneID string, opts recordsets.ListOptsBuilder) iter.Seq2[*recordsets.RecordSet, error]
	CreateRecordset(ctx context.Context, zoneID string, opts recordsets.CreateOptsBuilder) (*recordsets.RecordSet, error)
	GetRecordset(ctx context.Context, zoneID string, recordsetID string) (*recordsets.RecordSet, error)
	UpdateRecordset(ctx context.Context, zoneID string, recordsetID string, opts recordsets.UpdateOptsBuilder) (*recordsets.RecordSet, error)
	DeleteRecordset(ctx context.Context, zoneID string, recordsetID string) error
}

type dnsRecordsetClient struct{ client *gophercloud.ServiceClient }

// NewDNSRecordsetClient returns a new OpenStack client.
func NewDNSRecordsetClient(providerClient *gophercloud.ProviderClient, providerClientOpts *clientconfig.ClientOpts) (DNSRecordsetClient, error) {
	client, err := openstack.NewDNSV2(providerClient, gophercloud.EndpointOpts{
		Region:       providerClientOpts.RegionName,
		Availability: clientconfig.GetEndpointType(providerClientOpts.EndpointType),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create dnsrecordset service client: %v", err)
	}

	return &dnsRecordsetClient{client}, nil
}

func (c dnsRecordsetClient) ListRecordsets(ctx context.Context, zoneID string, opts recordsets.ListOptsBuilder) iter.Seq2[*recordsets.RecordSet, error] {
	pager := recordsets.ListByZone(c.client, zoneID, opts)
	return func(yield func(*recordsets.RecordSet, error) bool) {
		_ = pager.EachPage(ctx, yieldPage(recordsets.ExtractRecordSets, yield))
	}
}

func (c dnsRecordsetClient) CreateRecordset(ctx context.Context, zoneID string, opts recordsets.CreateOptsBuilder) (*recordsets.RecordSet, error) {
	return recordsets.Create(ctx, c.client, zoneID, opts).Extract()
}

func (c dnsRecordsetClient) DeleteRecordset(ctx context.Context, zoneID string, recordsetID string) error {
	return recordsets.Delete(ctx, c.client, zoneID, recordsetID).ExtractErr()
}

func (c dnsRecordsetClient) GetRecordset(ctx context.Context, zoneID string, recordsetID string) (*recordsets.RecordSet, error) {
	return recordsets.Get(ctx, c.client, zoneID, recordsetID).Extract()
}

func (c dnsRecordsetClient) UpdateRecordset(ctx context.Context, zoneID string, recordsetID string, opts recordsets.UpdateOptsBuilder) (*recordsets.RecordSet, error) {
	return recordsets.Update(ctx, c.client, zoneID, recordsetID, opts).Extract()
}

type dnsRecordsetErrorClient struct{ error }

// NewDNSRecordsetErrorClient returns a DNSRecordsetClient in which every method returns the given error.
func NewDNSRecordsetErrorClient(e error) DNSRecordsetClient {
	return dnsRecordsetErrorClient{e}
}

func (e dnsRecordsetErrorClient) ListRecordsets(_ context.Context, _ string, _ recordsets.ListOptsBuilder) iter.Seq2[*recordsets.RecordSet, error] {
	return func(yield func(*recordsets.RecordSet, error) bool) {
		yield(nil, e.error)
	}
}

func (e dnsRecordsetErrorClient) CreateRecordset(_ context.Context, _ string, _ recordsets.CreateOptsBuilder) (*recordsets.RecordSet, error) {
	return nil, e.error
}

func (e dnsRecordsetErrorClient) DeleteRecordset(_ context.Context, _ string, _ string) error {
	return e.error
}

func (e dnsRecordsetErrorClient) GetRecordset(_ context.Context, _ string, _ string) (*recordsets.RecordSet, error) {
	return nil, e.error
}

func (e dnsRecordsetErrorClient) UpdateRecordset(_ context.Context, _ string, _ string, _ recordsets.UpdateOptsBuilder) (*recordsets.RecordSet, error) {
	return nil, e.error
}
