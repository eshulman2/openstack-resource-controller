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

package osclients_test

import (
	"context"
	"errors"
	"testing"

	"github.com/k-orc/openstack-resource-controller/v2/internal/osclients"
)

// TestDNSRecordsetErrorClient verifies that the error client returns the
// configured error for every method.
func TestDNSRecordsetErrorClient(t *testing.T) {
	testErr := errors.New("test configured error")
	client := osclients.NewDNSRecordsetErrorClient(testErr)
	ctx := context.Background()

	t.Run("ListRecordsets", func(t *testing.T) {
		var gotErr error
		for _, err := range client.ListRecordsets(ctx, "zone-id", nil) {
			gotErr = err
			break
		}
		if !errors.Is(gotErr, testErr) {
			t.Errorf("ListRecordsets: expected %v, got %v", testErr, gotErr)
		}
	})

	t.Run("CreateRecordset", func(t *testing.T) {
		_, err := client.CreateRecordset(ctx, "zone-id", nil)
		if !errors.Is(err, testErr) {
			t.Errorf("CreateRecordset: expected %v, got %v", testErr, err)
		}
	})

	t.Run("DeleteRecordset", func(t *testing.T) {
		err := client.DeleteRecordset(ctx, "zone-id", "recordset-id")
		if !errors.Is(err, testErr) {
			t.Errorf("DeleteRecordset: expected %v, got %v", testErr, err)
		}
	})

	t.Run("GetRecordset", func(t *testing.T) {
		_, err := client.GetRecordset(ctx, "zone-id", "recordset-id")
		if !errors.Is(err, testErr) {
			t.Errorf("GetRecordset: expected %v, got %v", testErr, err)
		}
	})

	t.Run("UpdateRecordset", func(t *testing.T) {
		_, err := client.UpdateRecordset(ctx, "zone-id", "recordset-id", nil)
		if !errors.Is(err, testErr) {
			t.Errorf("UpdateRecordset: expected %v, got %v", testErr, err)
		}
	})
}
