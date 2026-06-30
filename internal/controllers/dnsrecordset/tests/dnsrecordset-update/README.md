# Update DNSRecordset

## Step 00

Create a DNSRecordset using only mandatory fields.

## Step 01

Update all mutable fields.

## Step 02

Revert the resource to its original value and verify that records are reverted, while the TTL is preserved since removing TTL from the spec is not managed/reconciled by the actuator.

## Reference

https://k-orc.cloud/development/writing-tests/#update
