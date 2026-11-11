# Copyright IBM Corp. 2014, 2026
# SPDX-License-Identifier: MPL-2.0
service "iam" {
  sdk {
    id            = "IAM"
    arn_namespace = "iam"
  }

  names {
    provider_name_upper  = "IAM"
    human_friendly       = "IAM (Identity & Access Management)"
    human_friendly_short = "IAM"
  }

  env_var {
    custom_env_var = "CTYUNOOS_ENDPOINT_IAM"
  }

  endpoint_info {
    endpoint_api_call = "ListRoles"
    endpoint_template = "https://oos-{region}-iam.ctyunapi.cn"
  }

  resource_prefix {
    correct = "ctyun_iam_"
  }

  provider_package_correct = "iam"
  doc_prefix               = ["iam_"]
  is_global = true
}

service "s3" {
  sdk {
    id            = "S3"
    arn_namespace = "s3"
  }

  names {
    provider_name_upper  = "S3"
    human_friendly       = "S3 (Simple Storage)"
    human_friendly_short = "S3"
  }

  env_var {
    custom_env_var = "CTYUNOOS_ENDPOINT"
  }

  endpoint_info {
    endpoint_api_call = "ListBuckets"
    endpoint_template = "https://oos-{region}.ctyunapi.cn"
  }

  resource_prefix {
    correct = "ctyun_oos_"
  }

  provider_package_correct = "s3"
  doc_prefix               = ["ctyun_oos_bucket", "ctyun_oos_object"]
}

service "cloudtrail" {
  sdk {
    id            = "CloudTrail"
    arn_namespace = "cloudtrail"
  }

  names {
    provider_name_upper = "CloudTrail"
    human_friendly      = "CloudTrail"
  }

  env_var {
    custom_env_var = "CTYUNOOS_ENDPOINT_TRAIL"
  }
  
  endpoint_info {
    endpoint_api_call = "ListChannels"
    endpoint_template = "https://oos-{region}-cloudtrail.ctyunapi.cn"
  }

  resource_prefix {
    correct = "ctyun_cloudtrail"
  }

  provider_package_correct = "cloudtrail"
  doc_prefix               = ["cloudtrail"]
  is_global                = true
}
