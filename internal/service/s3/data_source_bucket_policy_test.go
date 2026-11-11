// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/YakDriver/regexache"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	awspolicy "github.com/hashicorp/awspolicyequivalence"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TC-OOS-DPOL-01 Basic Read: reads the data source after creating the bucket policy resource, verifying the policy is semantically equivalent to the resource side.
func TestAccS3BucketPolicyDataSource_BasicRead_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_policy.test"
	resourceName := "ctyun_oos_bucket_policy.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketPolicyDataSourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					testAccCheckBucketPolicyMatch(dataSourceName, names.AttrPolicy, resourceName, names.AttrPolicy),
				),
			},
		},
	})
}

// TC-OOS-DPOL-02 JSON Normalization: creates the resource with a policy differing in whitespace, indentation, and field order, then reads the data source, verifying the output is stable normalized JSON.
func TestAccS3BucketPolicyDataSource_JSONNormalization_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_policy.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketPolicyDataSourceConfig_jsonNormalization(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrPolicy, testAccBucketPolicyDataSourceNormalizedPolicy(rName, "oos:GetObject")),
				),
			},
		},
	})
}

// TC-OOS-DPOL-05 Resource Consistency: reads the data source immediately after updating the policy resource, verifying the normalized policy text stays consistent for both single- and multi-statement policies.
func TestAccS3BucketPolicyDataSource_ResourceConsistency_update(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_policy.test"
	resourceName := "ctyun_oos_bucket_policy.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketPolicyDataSourceConfig_withPolicy(rName, "oos:GetObject"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrPolicy, testAccBucketPolicyDataSourceNormalizedPolicy(rName, "oos:GetObject")),
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrPolicy, resourceName, names.AttrPolicy),
				),
			},
			{
				Config: testAccBucketPolicyDataSourceConfig_multipleStatements(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrPolicy, testAccBucketPolicyDataSourceNormalizedMultipleStatementsPolicy(rName)),
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrPolicy, resourceName, names.AttrPolicy),
				),
			},
			{
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrPolicy, testAccBucketPolicyDataSourceNormalizedMultipleStatementsPolicy(rName)),
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrPolicy, resourceName, names.AttrPolicy),
				),
			},
			{
				Config: testAccBucketPolicyDataSourceConfig_withPolicy(rName, "oos:GetObject"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrPolicy, testAccBucketPolicyDataSourceNormalizedPolicy(rName, "oos:GetObject")),
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrPolicy, resourceName, names.AttrPolicy),
				),
			},
		},
	})
}

// TC-OOS-DPOL-06 Bucket Validation: covers illegal bucket names and an empty bucket name, verifying the schema or read path returns a clear error.
func TestAccS3BucketPolicyDataSource_BucketValidation_invalidName(t *testing.T) {
	testCases := []struct {
		name        string
		bucket      string
		errorRegexp string
	}{
		{name: "consecutive-dots", bucket: "foo..bar", errorRegexp: `(?i)api error InvalidBucketName`},
		{name: "uppercase", bucket: "Foo.Bar", errorRegexp: `(?is)must contain only lowercase letters, numbers, periods, and hyphens`},
		{name: "public-ipv4", bucket: "192.168.0.1", errorRegexp: `(?i)api error InvalidBucketName`},
		{name: "leading-dot", bucket: ".foo", errorRegexp: `(?i)api error InvalidBucketName`},
		{name: "underscore", bucket: "foo_bar", errorRegexp: `(?is)must contain only lowercase letters, numbers, periods, and\s+hyphens`},
		{name: "too-long", bucket: strings.Repeat("a", 64), errorRegexp: `(?i)expected length of bucket to be in the range \(3 - 63\), got `},
		{name: "too-short", bucket: strings.Repeat("a", 2), errorRegexp: `(?i)expected length of bucket to be in the range \(3 - 63\), got `},
		{name: "empty", bucket: "", errorRegexp: `(?i)expected length of bucket to be in the range \(3 - 63\), got `},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := acctest.Context(t)

			acctest.ParallelTest(ctx, t, resource.TestCase{
				PreCheck:                 func() { acctest.PreCheck(ctx, t) },
				ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
				ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      testAccBucketPolicyDataSourceConfig_bucket(testCase.bucket),
						ExpectError: regexache.MustCompile(testCase.errorRegexp),
					},
				},
			})
		})
	}
}

// TC-OOS-DPOL-04 Bucket Not Found: reads a non-existent bucket, verifying the current implementation returns a clear read error.
func TestAccS3BucketPolicyDataSource_BucketNotFound_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketPolicyDataSourceConfig_bucket(rName),
				ExpectError: regexache.MustCompile(`reading OOS Bucket.* Policy: couldn't find resource`),
			},
		},
	})
}

// TC-OOS-DPOL-03 Not Configured: reads a bucket that exists but has no policy configured, verifying the current implementation returns a clear read error.
func TestAccS3BucketPolicyDataSource_NotConfigured_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketPolicyDataSourceConfig_notConfigured(rName),
				ExpectError: regexache.MustCompile(`reading OOS Bucket.* Policy: couldn't find resource`),
			},
		},
	})
}

func testAccCheckBucketPolicyMatch(nameFirst, keyFirst, nameSecond, keySecond string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[nameFirst]
		if !ok {
			return fmt.Errorf("Not found: %s", nameFirst)
		}

		policy1, ok := rs.Primary.Attributes[keyFirst]
		if !ok {
			return fmt.Errorf("attribute %q not found for %q", keyFirst, nameFirst)
		}

		rs, ok = s.RootModule().Resources[nameSecond]
		if !ok {
			return fmt.Errorf("Not found: %s", nameSecond)
		}

		policy2, ok := rs.Primary.Attributes[keySecond]
		if !ok {
			return fmt.Errorf("attribute %q not found for %q", keySecond, nameSecond)
		}

		areEquivalent, err := awspolicy.PoliciesAreEquivalent(policy1, policy2)
		if err != nil {
			return fmt.Errorf("comparing IAM Policies failed: %w", err)
		}

		if !areEquivalent {
			return fmt.Errorf("S3 bucket policies differ.\npolicy1: %s\npolicy2: %s", policy1, policy2)
		}

		return nil
	}
}

func testAccDataSourceBucketPolicyConfig_base(rName, action string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_policy" "test" {
  bucket = ctyun_oos_bucket.test.bucket
  policy = jsonencode({
    Version = "2012-10-17"
    Id      = "*"
    Statement = [
      {
        Sid       = "*"
        Effect    = "Allow"
        Principal = { CTYUN = "arn:ctyun:iam:::user/test" }
        Action    = %[2]q
        Resource  = "arn:ctyun:oos:::%[1]s/*"
      }
    ]
  })
}
`, rName, action)
}

func testAccBucketPolicyDataSourceConfig_basic(rName string) string {
	return testAccBucketPolicyDataSourceConfig_withPolicy(rName, "oos:GetObject")
}

func testAccBucketPolicyDataSourceConfig_withPolicy(rName, action string) string {
	return acctest.ConfigCompose(testAccDataSourceBucketPolicyConfig_base(rName, action), `
data "ctyun_oos_bucket_policy" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  depends_on = [ctyun_oos_bucket_policy.test]
}
`)
}

func testAccBucketPolicyDataSourceConfig_multipleStatements(rName string) string {
	return acctest.ConfigCompose(fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_policy" "test" {
  bucket = ctyun_oos_bucket.test.bucket
  policy = jsonencode({
    Version = "2012-10-17"
    Id      = "*"
    Statement = [
      {
        Sid       = "ObjectRead"
        Effect    = "Allow"
        Principal = { CTYUN = "arn:ctyun:iam:::user/test" }
        Action    = "oos:GetObject"
        Resource  = "arn:ctyun:oos:::%[1]s/*"
      },
      {
        Sid       = "ObjectWrite"
        Effect    = "Allow"
        Principal = { CTYUN = "arn:ctyun:iam:::user/test" }
        Action    = "oos:PutObject"
        Resource  = "arn:ctyun:oos:::%[1]s/*"
      },
    ]
  })
}
`, rName), `
data "ctyun_oos_bucket_policy" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  depends_on = [ctyun_oos_bucket_policy.test]
}
`)
}

func testAccBucketPolicyDataSourceConfig_jsonNormalization(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_policy" "test" {
  bucket = ctyun_oos_bucket.test.bucket
  policy = <<POLICY
{
  "Version" : "2012-10-17",
  "Statement" : [
    {
      "Resource" : "arn:ctyun:oos:::%[1]s/*",
      "Principal" : { "CTYUN" : "arn:ctyun:iam:::user/test" },
      "Sid" : "*",
      "Effect" : "Allow",
      "Action" : "oos:GetObject"
    }
  ],
  "Id" : "*"
}
POLICY
}

data "ctyun_oos_bucket_policy" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  depends_on = [ctyun_oos_bucket_policy.test]
}
`, rName)
}

func testAccBucketPolicyDataSourceNormalizedPolicy(rName, action string) string {
	return fmt.Sprintf(`{"Id":"*","Statement":[{"Action":%[2]q,"Effect":"Allow","Principal":{"CTYUN":"arn:ctyun:iam:::user/test"},"Resource":"arn:ctyun:oos:::%[1]s/*","Sid":"*"}],"Version":"2012-10-17"}`, rName, action)
}

func testAccBucketPolicyDataSourceNormalizedMultipleStatementsPolicy(rName string) string {
	return fmt.Sprintf(`{"Id":"*","Statement":[{"Action":"oos:GetObject","Effect":"Allow","Principal":{"CTYUN":"arn:ctyun:iam:::user/test"},"Resource":"arn:ctyun:oos:::%[1]s/*","Sid":"ObjectRead"},{"Action":"oos:PutObject","Effect":"Allow","Principal":{"CTYUN":"arn:ctyun:iam:::user/test"},"Resource":"arn:ctyun:oos:::%[1]s/*","Sid":"ObjectWrite"}],"Version":"2012-10-17"}`, rName)
}

func testAccBucketPolicyDataSourceConfig_bucket(bucketName string) string {
	return fmt.Sprintf(`
data "ctyun_oos_bucket_policy" "test" {
  bucket = %[1]q
}
`, bucketName)
}

func testAccBucketPolicyDataSourceConfig_notConfigured(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

data "ctyun_oos_bucket_policy" "test" {
  bucket = ctyun_oos_bucket.test.bucket
}
`, rName)
}
