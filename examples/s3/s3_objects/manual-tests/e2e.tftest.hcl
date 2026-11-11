# Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
# SPDX-License-Identifier: MPL-2.0

# TC-S3-OBJECTS-CLI-01 Create objects, list them through ctyun_oos_objects, then refine the query and confirm convergence.
run "lists_objects_by_prefix" {
  command = apply

  assert {
    condition     = output.data_id == output.bucket_name
    error_message = "The objects data source ID should be the bucket name."
  }

  assert {
    condition     = output.data_prefix == "${output.object_prefix}/objects/" && output.data_delimiter == null
    error_message = "The initial objects query did not preserve its prefix and delimiter."
  }

  assert {
    condition     = length(output.keys) == 3 && length(output.common_prefixes) == 0
    error_message = "The initial objects query should return all three object keys and no common prefixes."
  }

  assert {
    condition     = alltrue([for key in output.expected_keys : contains(output.keys, key)])
    error_message = "The objects data source did not return all created keys."
  }
}

run "lists_common_prefixes" {
  command = apply

  variables {
    query_prefix = "objects/prefix1/"
    delimiter    = "/"
    max_keys     = 1
  }

  assert {
    condition     = output.data_prefix == "${output.object_prefix}/objects/prefix1/" && output.data_delimiter == "/" && output.data_max_keys == 1
    error_message = "The refined objects query did not preserve prefix, delimiter, and max_keys."
  }

  assert {
    condition     = length(output.keys) == 1 && length(output.common_prefixes) == 0 && contains(output.keys, output.expected_keys[0])
    error_message = "The prefix query should return one direct key due to max_keys and no common prefixes."
  }
}

run "verifies_convergence" {
  command = plan

  variables {
    query_prefix = "objects/prefix1/"
    delimiter    = "/"
    max_keys     = 1
  }

  assert {
    condition     = output.data_prefix == "${output.object_prefix}/objects/prefix1/" && output.data_delimiter == "/"
    error_message = "Apply was followed by a plan that still expected an objects query change."
  }
}
