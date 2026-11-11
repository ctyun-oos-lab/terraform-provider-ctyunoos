# CTYUN OOS Provider Examples

This directory contains the active example set for the CTYUN OOS provider fork.
The retained examples include S3, IAM, and CloudTrail scenarios that match the
current provider focus.

Current active example directories:

- `s3/s3_bucket` (real Terraform CLI S3 bucket control-plane test with inline subresources)
- `s3/s3_bucket_acl` (real Terraform CLI S3 bucket ACL test)
- `s3/s3_bucket_lifecycle_configuration` (real Terraform CLI lifecycle resource and data-source test)
- `s3/s3_bucket_logging` (real Terraform CLI S3 bucket logging test)
- `s3/s3_bucket_object_lock_configuration` (real Terraform CLI Object Lock resource and data-source test)
- `s3/s3_bucket_policy` (real Terraform CLI S3 bucket policy test)
- `s3/s3_bucket_server_side_encryption_configuration` (real Terraform CLI SSE resource and data-source test)
- `s3/s3_bucket_soft_delete_configuration` (real Terraform CLI soft-delete resource and data-source test)
- `s3/s3_object` (manual Terraform CLI S3 object test; requires a shared bucket)
- `s3/s3_object_copy` (manual Terraform CLI S3 object-copy test; requires a shared bucket)
- `s3/s3_object_recover` (manual Terraform CLI S3 object-recovery test; requires a shared bucket)
- `s3/s3_objects` (manual Terraform CLI S3 object-listing test; requires a shared bucket)
- `s3/s3_objects_soft_delete` (manual Terraform CLI S3 soft-deleted-object listing test; requires a shared bucket)
- `iam/iam-user` (user + user/users data sources, with import examples)
- `iam/iam-group` (group + group/groups data sources)
- `iam/iam-role` (role + role/roles data sources)
- `iam/iam-access-key` (access key + access_keys data source)
- `iam/iam-mfa` (virtual MFA device)
- `iam/iam-user-group-attachment` (user–group membership)
- `iam/iam-saml-provider` (SAML provider)
- `iam/iam-user-policy-attachment` (user policy attachment)
- `iam/iam-group-policy-attachment` (group policy attachment)
- `iam/iam-role-policy-attachment` (role policy attachment)
- `iam/iam-account-login-security-policy` (account login security policy singleton)
- `iam/iam-account-password-policy` (account password policy singleton)
- `iam/iam-account-summary` (account summary data source)
- `iam/iam-assume-role` (provider `assume_role`: single, full fields, chain)
- `cloudtrail/cloudtrail_trail` (real Terraform CLI CloudTrail resource and data source test)
- `compound/iam-full-chain-users-ds` (real Terraform CLI IAM composite test)
- `compound/iam-user-access-key-users-ds` (real Terraform CLI IAM composite test)
- `compound/iam-user-role-shared-policy-users-ds` (real Terraform CLI IAM composite test)
- `compound/s3-full-inline-bucket-object-ds` (real Terraform CLI S3 bucket-control-plane test)
- `compound/s3-shared-bucket` (persistent S3 bucket for shared-bucket object tests)
- `compound/s3-shared-bucket-object-ds` (manual Terraform CLI S3 object and data-source test; requires a shared bucket)
- `compound/s3-independent-subresources-object-copy-ds` (real Terraform CLI S3 independent-subresource control-plane test)
- `compound/s3-shared-bucket-object-copy-ds` (manual Terraform CLI S3 object-copy and data-source test; requires a shared bucket)
- `compound/s3-shared-bucket-multiple-objects-ds` (manual Terraform CLI S3 multiple-object and data-source test; requires a shared bucket)
- `compound/iam-user-s3-bucket-policy` (real Terraform CLI IAM-to-S3 bucket-policy test)
- `compound/iam-role-s3-bucket-policy-object` (real Terraform CLI IAM role-to-S3 bucket-policy control-plane test)
- `compound/s3-shared-bucket-iam-role-policy-object-ds` (manual Terraform CLI IAM role-to-S3 object and data-source test; requires a shared bucket)
- `compound/s3-bucket-cloudtrail-trail` (real Terraform CLI S3-to-CloudTrail composite test)
- `compound/iam-role-s3-bucket-cloudtrail` (real Terraform CLI IAM role-to-S3-to-CloudTrail composite test)

The default test scenarios under each `tests/` directory can be run from the provider root with `make testiac` (set `TF_TEST_CLI=tofu` for OpenTofu); use `TF_TEST_DIR` to select one regular scenario, or `TF_TEST_ROOT=examples/<service>` to run every regular suite under a service sequentially.
Shared-bucket scenarios under `manual-tests/` are excluded from those commands because they require a pre-existing bucket. Use `make shared-bucket-test` to discover and run all of them, or set `TF_TEST_DIR=examples/<directory>` to run one scenario.

Make-managed Terraform tests share provider downloads through the repository-level
`.terraform-plugin-cache` directory while keeping each scenario's `.terraform`
working directory and dependency lock file independent.

## Prerequisites

- Terraform CLI available as `terraform` on `PATH` (what `make testiac` invokes; set `TF_TEST_CLI=tofu` for OpenTofu). Applying a scenario's `main.tf` directly needs **>= 1.5.7**; running the `.tftest.hcl` suites through `make testiac` needs **>= 1.6.0**, when the `terraform test` framework and `.tftest.hcl` format became GA.
- A local provider binary built with `make go-build`.

### Local provider binary override

`make go-build` installs the provider into `terraform-plugin-dir`. Point the CLI at it with a `provider_installation` / `dev_overrides` block in a Terraform CLI config file (`~/.terraformrc`, or a file exported via `TF_CLI_CONFIG_FILE`) that maps `ctyun-oos-lab/ctyunoos` to that directory for your `<version>` and `<os_arch>`. Keep `direct {}` so `hashicorp/random` is still downloaded normally.

### Credentials and endpoints

All scenarios read credentials from the environment. Set the endpoint(s) the target service needs:

```shell
export CTYUNOOS_ACCESS_KEY="..."
export CTYUNOOS_SECRET_KEY="..."
export CTYUNOOS_ENDPOINT="https://<s3-host>:<port>"          # S3, and any S3-backed scenario
export CTYUNOOS_ENDPOINT_IAM="https://<iam-host>:<port>"     # IAM scenarios
export CTYUNOOS_ENDPOINT_TRAIL="https://<trail-host>:<port>" # CloudTrail scenarios
```

IAM-only scenarios need just `CTYUNOOS_ENDPOINT_IAM`; CloudTrail needs both S3 and trail endpoints.

### Notes

- Do not run IAM account-level suites in parallel. `iam-account-login-security-policy`, `iam-account-password-policy`, and `iam-account-summary` all mutate or observe the same IAM account; use a dedicated account when possible.
- `iam-assume-role` uses the configured (typically root) credentials only for setup; AssumeRole itself requires a non-root IAM user, which the suite creates automatically.
- Each scenario creates only uniquely named temporary resources and destroys them during `terraform test` teardown.
