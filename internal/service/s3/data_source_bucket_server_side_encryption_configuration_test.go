// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package s3_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/YakDriver/regexache"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TC-OOS-DSSE-01 Basic Read: reads the data source after creating the bucket SSE resource, verifying the SSE algorithm matches the resource.
func TestAccS3BucketServerSideEncryptionConfigurationDataSource_BasicRead_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_server_side_encryption_configuration.test"
	resourceName := "ctyun_oos_bucket_server_side_encryption_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketServerSideEncryptionConfigurationDataSourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(dataSourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.apply_server_side_encryption_by_default.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.apply_server_side_encryption_by_default.0.sse_algorithm", "AES256"),
				),
			},
		},
	})
}

// TC-OOS-DSSE-03 Bucket Not Found: reads a non-existent bucket, verifying a clear error is returned.
func TestAccS3BucketServerSideEncryptionConfigurationDataSource_BucketNotFound_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketServerSideEncryptionConfigurationDataSourceConfig_bucketNotFound(rName),
				ExpectError: regexache.MustCompile(`Bucket Server Side Encryption Configuration Data Source.*couldn't find resource`),
			},
		},
	})
}

// TC-OOS-DSSE-02 Not Configured: reads when the bucket exists but SSE is not configured, verifying a clear error is returned
func TestAccS3BucketServerSideEncryptionConfigurationDataSource_NotConfigured_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketServerSideEncryptionConfigurationDataSourceConfig_notConfigured(rName),
				ExpectError: regexache.MustCompile(`Bucket Server Side Encryption Configuration Data Source.*couldn't find resource`),
			},
		},
	})
}

// TC-OOS-DSSE-04 Nested Rule Stability: refreshes the same SSE configuration multiple times, verifying the nested block order is stable with no perpetual diff.
func TestAccS3BucketServerSideEncryptionConfigurationDataSource_NestedRuleStability_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_server_side_encryption_configuration.test"
	resourceName := "ctyun_oos_bucket_server_side_encryption_configuration.test"
	config := testAccBucketServerSideEncryptionConfigurationDataSourceConfig_basic(rName)

	check := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
		resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, rName),
		resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
		resource.TestCheckResourceAttr(dataSourceName, "rule.#", "1"),
		resource.TestCheckResourceAttr(dataSourceName, "rule.0.apply_server_side_encryption_by_default.#", "1"),
		resource.TestCheckResourceAttr(dataSourceName, "rule.0.apply_server_side_encryption_by_default.0.sse_algorithm", "AES256"),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config, Check: check},
			{RefreshState: true, Check: check},
			{RefreshState: true, Check: check},
			{Config: config, PlanOnly: true, ExpectNonEmptyPlan: false},
		},
	})
}

// TC-OOS-DSSE-05 Resource Consistency: removes and re-declares the same SSE resource, verifying the data source read result stays consistent.
func TestAccS3BucketServerSideEncryptionConfigurationDataSource_ResourceConsistency_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_server_side_encryption_configuration.test"
	resourceName := "ctyun_oos_bucket_server_side_encryption_configuration.test"
	bucketName := "ctyun_oos_bucket.test"
	resourceConfig := testAccBucketServerSideEncryptionConfigurationDataSourceConfig_basic(rName)
	detachedConfig := testAccBucketServerSideEncryptionConfigurationDataSourceConfig_resourceConsistencyDetached(rName)

	check := func(bucketResourceName string) resource.TestCheckFunc {
		return resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, bucketResourceName, names.AttrBucket),
			resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, rName),
			resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
			resource.TestCheckResourceAttr(dataSourceName, "rule.#", "1"),
			resource.TestCheckResourceAttr(dataSourceName, "rule.0.apply_server_side_encryption_by_default.#", "1"),
			resource.TestCheckResourceAttr(dataSourceName, "rule.0.apply_server_side_encryption_by_default.0.sse_algorithm", "AES256"),
		)
	}

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{Config: resourceConfig, Check: check(resourceName)},
			{Config: detachedConfig, Check: check(bucketName)},
			{Config: resourceConfig, Check: check(resourceName)},
			{Config: resourceConfig, PlanOnly: true, ExpectNonEmptyPlan: false},
		},
	})
}

// TC-OOS-DSSE-06 Bucket Validation: covers various illegal bucket inputs, verifying the schema and API return clear errors.
func TestAccS3BucketServerSideEncryptionConfigurationDataSource_BucketValidation_invalidName(t *testing.T) {
	testCases := []struct {
		name        string
		bucket      string
		errorRegexp string
	}{
		{name: "consecutive-dots", bucket: "foo..bar", errorRegexp: `(?i)api error InvalidBucketName`},
		{name: "uppercase", bucket: "Foo.Bar", errorRegexp: `(?is)must contain only lowercase letters, numbers, periods, and\s+hyphens`},
		{name: "public-ipv4", bucket: "192.168.0.1", errorRegexp: `(?i)api error InvalidBucketName`},
		{name: "leading-dot", bucket: ".foo", errorRegexp: `(?i)api error InvalidBucketName`},
		{name: "underscore", bucket: "foo_bar", errorRegexp: `(?is)must contain only lowercase letters, numbers, periods, and\s+hyphens`},
		{name: "too-long", bucket: strings.Repeat("a", 64), errorRegexp: `(?i)Attribute bucket string length must be between 3 and 63, got`},
		{name: "too-short", bucket: strings.Repeat("a", 2), errorRegexp: `(?i)Attribute bucket string length must be between 3 and 63, got`},
		{name: "empty", bucket: "", errorRegexp: `(?i)Attribute bucket string length must be between 3 and 63, got`},
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
						Config:      testAccBucketServerSideEncryptionConfigurationDataSourceConfig_bucketNotFound(testCase.bucket),
						ExpectError: regexache.MustCompile(testCase.errorRegexp),
					},
				},
			})
		})
	}
}

func testAccBucketServerSideEncryptionConfigurationDataSourceConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

}

resource "ctyun_oos_bucket_server_side_encryption_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

data "ctyun_oos_bucket_server_side_encryption_configuration" "test" {
  bucket = ctyun_oos_bucket_server_side_encryption_configuration.test.bucket
}
`, rName)
}

func testAccBucketServerSideEncryptionConfigurationDataSourceConfig_resourceConsistencyDetached(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

}

data "ctyun_oos_bucket_server_side_encryption_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket
}
`, rName)
}

func testAccBucketServerSideEncryptionConfigurationDataSourceConfig_bucketNotFound(rName string) string {
	return fmt.Sprintf(`
data "ctyun_oos_bucket_server_side_encryption_configuration" "test" {
  bucket = %[1]q
}
`, rName)
}

func testAccBucketServerSideEncryptionConfigurationDataSourceConfig_notConfigured(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

}

data "ctyun_oos_bucket_server_side_encryption_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket
}
`, rName)
}
