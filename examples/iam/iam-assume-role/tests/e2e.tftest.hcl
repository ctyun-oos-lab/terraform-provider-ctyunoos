# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-ASSUME-ROLE-CLI-01 Provider assume_role.
#
# OOS IAM rejects AssumeRole from root accounts. Setup uses the env root AK to
# create a source IAM user + access key + roles; assumed modules use that
# user's AK/SK with assume_role.
#
# Proof that temporary role credentials are used (not root / not source user):
#   - Source user may only iam:AssumeRole (no GetAccountSummary).
#   - Target / role B may only iam:GetAccountSummary.
#   - Reading account summary under assume_role must succeed.
#   - Role A has only AssumeRole->B (no GetAccountSummary), so A->B success
#     means the final session is B's credentials.
#
# Note: terraform test expect_failures only covers checkable objects (variable
# validation, preconditions, check blocks). Provider AccessDenied on apply is
# NOT absorbable via expect_failures — those deny runs are intentionally omitted.

run "skips_empty_assume_role_block" {
  command = apply

  module {
    source = "./empty"
  }

  assert {
    condition     = output.summary_id == "iam_account_summary"
    error_message = "Empty assume_role should keep permanent credentials and read account summary."
  }

  assert {
    condition     = output.users >= 0
    error_message = "Account summary users counter should be non-negative with an empty assume_role block."
  }
}

run "skips_blank_role_arn" {
  command = apply

  module {
    source = "./empty-arn"
  }

  assert {
    condition     = output.summary_id == "iam_account_summary"
    error_message = "Blank role_arn should keep permanent credentials and read account summary."
  }
}

run "setup_roles" {
  command = apply

  module {
    source = "./setup"
  }

  assert {
    condition     = can(regex("^arn:ctyun:iam::[^:]+:role/", output.role_arn))
    error_message = "Setup did not return a CTYUN IAM role ARN for the single-hop target."
  }

  assert {
    condition     = can(regex("^arn:ctyun:iam::[^:]+:role/", output.role_a_arn))
    error_message = "Setup did not return a CTYUN IAM role ARN for chain role A."
  }

  assert {
    condition     = can(regex("^arn:ctyun:iam::[^:]+:role/", output.role_b_arn))
    error_message = "Setup did not return a CTYUN IAM role ARN for chain role B."
  }

  assert {
    condition     = output.source_access_key != "" && output.source_user_arn != ""
    error_message = "Setup did not create a source IAM user access key for AssumeRole."
  }

  assert {
    condition     = output.attachment_target_id != "" && output.attachment_chain_a_id != "" && output.attachment_chain_b_id != "" && output.source_attachment_id != ""
    error_message = "Setup did not attach IAM policies to the source user and AssumeRole target roles."
  }
}

run "assumes_single_role_allowed_summary" {
  command = apply

  module {
    source = "./assumed"
  }

  variables {
    access_key   = run.setup_roles.source_access_key
    secret_key   = run.setup_roles.source_secret_key
    role_arn     = run.setup_roles.role_arn
    session_name = "tf-cli-assume-role"
    duration     = "3600s"
  }

  assert {
    condition     = output.summary_id == "iam_account_summary"
    error_message = "Assumed credentials should be able to call GetAccountSummary."
  }

  assert {
    condition     = output.users >= 0
    error_message = "Account summary users counter should be non-negative with assumed credentials."
  }

  assert {
    condition     = output.assumed_role_arn == var.role_arn
    error_message = "The assumed module did not use the setup target role ARN."
  }
}

run "assumes_role_with_all_optional_fields_allowed_summary" {
  command = apply

  module {
    source = "./assumed-full"
  }

  variables {
    access_key = run.setup_roles.source_access_key
    secret_key = run.setup_roles.source_secret_key
    role_arn   = run.setup_roles.role_arn
  }

  assert {
    condition     = output.summary_id == "iam_account_summary"
    error_message = "Full assume_role fields should still allow GetAccountSummary."
  }
}

# Role A: only iam:AssumeRole -> B (no GetAccountSummary).
# Role B: only iam:GetAccountSummary.
# Summary success after A->B therefore requires B's temporary credentials.
run "assumes_role_chain_allowed_summary" {
  command = apply

  module {
    source = "./assumed-chain"
  }

  variables {
    access_key = run.setup_roles.source_access_key
    secret_key = run.setup_roles.source_secret_key
    role_a_arn = run.setup_roles.role_a_arn
    role_b_arn = run.setup_roles.role_b_arn
  }

  assert {
    condition     = output.summary_id == "iam_account_summary"
    error_message = "Chained AssumeRole should allow GetAccountSummary with role B temporary credentials."
  }

  assert {
    condition     = output.role_a_arn == var.role_a_arn && output.role_b_arn == var.role_b_arn
    error_message = "Chain module did not receive both setup role ARNs."
  }
}
