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
	"errors"
	"fmt"
	"net"
	"strings"
)

// ValidateDNSRecordset implements format validation rules for DNSRecordset specifications
// to catch format issues before making OpenStack API calls.
func ValidateDNSRecordset(obj orcObjectPT, zoneSuffix string) error {
	if obj == nil || obj.Spec.Resource == nil {
		return nil
	}
	resource := obj.Spec.Resource

	// 1. Name suffix validation
	recordsetName := getDNSRecordsetName(obj)
	normRecordsetName := strings.ToLower(recordsetName)
	normZoneSuffix := strings.ToLower(zoneSuffix)
	if normZoneSuffix != "" && !strings.HasSuffix(normZoneSuffix, ".") {
		normZoneSuffix += "."
	}

	if normZoneSuffix != "" {
		if normRecordsetName != normZoneSuffix && !strings.HasSuffix(normRecordsetName, "."+normZoneSuffix) {
			return fmt.Errorf("recordset name %q does not end with the parent zone suffix %q", recordsetName, zoneSuffix)
		}
	}

	// 2. Records format validation per recordset type
	recordType := strings.ToUpper(resource.Type)
	if len(resource.Records) == 0 {
		return errors.New("records are required")
	}

	for _, r := range resource.Records {
		switch recordType {
		case "A":
			ip := net.ParseIP(r)
			if ip == nil || ip.To4() == nil {
				return fmt.Errorf("invalid IPv4 address %q for A record", r)
			}
		case "AAAA":
			ip := net.ParseIP(r)
			if ip == nil || ip.To4() != nil {
				return fmt.Errorf("invalid IPv6 address %q for AAAA record", r)
			}
		case "CNAME":
			if !strings.HasSuffix(r, ".") {
				return fmt.Errorf("invalid CNAME record %q: must end with a trailing dot", r)
			}
		case "TXT":
			// Check for unbalanced quotes (syntax error)
			hasPrefix := strings.HasPrefix(r, `"`)
			hasSuffix := strings.HasSuffix(r, `"`)
			if hasPrefix != hasSuffix {
				return fmt.Errorf("invalid TXT record %q: mismatched/unbalanced quotes", r)
			}
		}
	}

	return nil
}
