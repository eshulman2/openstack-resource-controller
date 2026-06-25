# Check dependency handling for imported DNSRecordset

## Step 00

Import a DNSRecordset that references other imported resources. The referenced imported resources have no matching resources yet.
Verify the DNSRecordset is waiting for the dependency to be ready.

## Step 01

Create a DNSRecordset matching the import filter, except for referenced resources, and verify that it's not being imported.

## Step 02

Create the referenced resources and a DNSRecordset matching the import filters.

Verify that the observed status on the imported DNSRecordset corresponds to the spec of the created DNSRecordset.

## Step 03

Delete the referenced resources and check that ORC does not prevent deletion. The OpenStack resources still exist because they
were imported resources and we only deleted the ORC representation of it.

## Step 04

Delete the DNSRecordset and validate that all resources are gone.

## Reference

https://k-orc.cloud/development/writing-tests/#import-dependency
