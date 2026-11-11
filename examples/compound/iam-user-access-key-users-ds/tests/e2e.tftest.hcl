# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-IAM-COMPOSITE-03 User AccessKey With Users DS
run "creates_and_reads_user_access_key" {
  command = apply

  assert {
    condition     = output.returned_user_names == output.expected_user_names
    error_message = "The users data source did not return exactly the user created by this test."
  }

  assert {
    condition     = output.access_key_status == "Active"
    error_message = "The access key resource did not read back the default Active status."
  }

  assert {
    condition     = output.returned_access_key_status == "Active"
    error_message = "The access keys data source did not return the created access key with Active status."
  }
}

run "verifies_convergence" {
  command = plan
}
