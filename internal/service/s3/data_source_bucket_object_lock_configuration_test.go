// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TC-OOS-DLOCK-01 Basic Read: reads the data source after creating the object lock configuration resource,
// verifying that TestAccS3BucketObjectLockConfigurationDataSource_BasicRead_basic reads a configuration consistent with the resource.
func TestAccS3BucketObjectLockConfigurationDataSource_BasicRead_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_object_lock_configuration.test"
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationDataSourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttrPair(dataSourceName, "object_lock_enabled", resourceName, "object_lock_enabled"),
					resource.TestCheckResourceAttr(dataSourceName, "object_lock_enabled", "Disabled"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.default_retention.#", "1"),
					resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.default_retention.0.days", resourceName, "rule.0.default_retention.0.days"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.default_retention.0.days", "3"),
					resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.default_retention.0.mode", resourceName, "rule.0.default_retention.0.mode"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
				),
			},
		},
	})
}

// TC-OOS-DLOCK-02 Days Retention Readback: reads the data source after configuring days retention,
// verifying that TestAccS3BucketObjectLockConfigurationDataSource_DaysRetentionReadback_basic correctly back-fills days with years empty.
func TestAccS3BucketObjectLockConfigurationDataSource_DaysRetentionReadback_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_object_lock_configuration.test"
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationDataSourceConfig_daysRetention(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(dataSourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.default_retention.#", "1"),
					resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.default_retention.0.mode", resourceName, "rule.0.default_retention.0.mode"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.default_retention.0.days", resourceName, "rule.0.default_retention.0.days"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.default_retention.0.days", "7"),
					resource.TestCheckNoResourceAttr(dataSourceName, "rule.0.default_retention.0.years"),
				),
			},
		},
	})
}

// TC-OOS-DLOCK-03 Years Retention Readback: reads the data source after configuring years retention,
// verifying that TestAccS3BucketObjectLockConfigurationDataSource_YearsRetentionReadback_basic correctly back-fills years with days empty.
func TestAccS3BucketObjectLockConfigurationDataSource_YearsRetentionReadback_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_object_lock_configuration.test"
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationDataSourceConfig_yearsRetention(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(dataSourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.default_retention.#", "1"),
					resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.default_retention.0.mode", resourceName, "rule.0.default_retention.0.mode"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.default_retention.0.years", resourceName, "rule.0.default_retention.0.years"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.default_retention.0.years", "2"),
					resource.TestCheckNoResourceAttr(dataSourceName, "rule.0.default_retention.0.days"),
				),
			},
		},
	})
}

// TC-OOS-DLOCK-04 Not Configured: reads the data source when the bucket exists but the object lock configuration has no rule,
// verifying that TestAccS3BucketObjectLockConfigurationDataSource_NotConfigured_basic returns a clear read error for a bucket without the object lock attribute.
func TestAccS3BucketObjectLockConfigurationDataSource_NotConfigured_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketObjectLockConfigurationDataSourceConfig_withoutRule(rName),
				ExpectError: regexache.MustCompile(`Bucket Object Lock Configuration Data Source.*couldn't find resource`),
			},
		},
	})
}

// TC-OOS-DLOCK-04 Not Configured: reads the data source when the object lock configuration exists but has no rule,
// verifying that TestAccS3BucketObjectLockConfigurationDataSource_NotConfigured_withoutRule back-fills the Disabled state and represents rule as an empty list.
func TestAccS3BucketObjectLockConfigurationDataSource_NotConfigured_withoutRule(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_object_lock_configuration.test"
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationDataSourceConfig_objectLockConfigurationWithoutRule(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttrPair(dataSourceName, "object_lock_enabled", resourceName, "object_lock_enabled"),
					resource.TestCheckResourceAttr(dataSourceName, "object_lock_enabled", "Disabled"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.#", "0"),
				),
			},
		},
	})
}

// TC-OOS-DLOCK-05 Bucket Not Found: reads a non-existent bucket,
// verifying that TestAccS3BucketObjectLockConfigurationDataSource_BucketNotFound_basic returns a clear read error.
func TestAccS3BucketObjectLockConfigurationDataSource_BucketNotFound_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketObjectLockConfigurationDataSourceConfig_bucketNotFound(rName),
				ExpectError: regexache.MustCompile(`Bucket Object Lock Configuration Data Source.*couldn't find resource`),
			},
		},
	})
}

// TC-OOS-DLOCK-07 Bucket Validation: covers various illegal bucket inputs, verifying the data source returns clear errors under both local validation and API validation.
func TestAccS3BucketObjectLockConfigurationDataSource_BucketValidation_invalidName(t *testing.T) {
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
				Steps: []resource.TestStep{{
					Config:      testAccBucketObjectLockConfigurationDataSourceConfig_bucketNotFound(testCase.bucket),
					ExpectError: regexache.MustCompile(testCase.errorRegexp),
				}},
			})
		})
	}
}

// TC-OOS-DLOCK-06 Resource Consistency: reads via the data source immediately after the resource updates retention,
// keeping the original retention read-back coverage and verifying the resource and data source states are consistent.
func TestAccS3BucketObjectLockConfigurationDataSource_ResourceConsistency_retentionUpdate(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_object_lock_configuration.test"
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationDataSourceConfig_retention(rName, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttrPair(dataSourceName, "object_lock_enabled", resourceName, "object_lock_enabled"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.default_retention.#", "1"),
					resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.default_retention.0.mode", resourceName, "rule.0.default_retention.0.mode"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.default_retention.0.days", resourceName, "rule.0.default_retention.0.days"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.default_retention.0.days", "1"),
					resource.TestCheckNoResourceAttr(dataSourceName, "rule.0.default_retention.0.years"),
				),
			},
			{
				Config: testAccBucketObjectLockConfigurationDataSourceConfig_retention(rName, 5),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttrPair(dataSourceName, "object_lock_enabled", resourceName, "object_lock_enabled"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.default_retention.#", "1"),
					resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.default_retention.0.mode", resourceName, "rule.0.default_retention.0.mode"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.default_retention.0.days", resourceName, "rule.0.default_retention.0.days"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.default_retention.0.days", "5"),
					resource.TestCheckNoResourceAttr(dataSourceName, "rule.0.default_retention.0.years"),
				),
			},
		},
	})
}

// TC-OOS-DLOCK-06 Resource Consistency: reads via the data source immediately after the resource updates Enabled/Disabled,
// verifying that TestAccS3BucketObjectLockConfigurationDataSource_ResourceConsistency_enabledDisabled shows consistent state on both sides.
func TestAccS3BucketObjectLockConfigurationDataSource_ResourceConsistency_enabledDisabled(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_object_lock_configuration.test"
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"
	check := func(objectLockEnabled string) resource.TestCheckFunc {
		return resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
			resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
			resource.TestCheckResourceAttrPair(dataSourceName, "object_lock_enabled", resourceName, "object_lock_enabled"),
			resource.TestCheckResourceAttr(dataSourceName, "object_lock_enabled", objectLockEnabled),
			resource.TestCheckResourceAttr(dataSourceName, "rule.#", "1"),
			resource.TestCheckResourceAttr(dataSourceName, "rule.0.default_retention.#", "1"),
			resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.default_retention.0.mode", resourceName, "rule.0.default_retention.0.mode"),
			resource.TestCheckResourceAttr(dataSourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
			resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.default_retention.0.days", resourceName, "rule.0.default_retention.0.days"),
			resource.TestCheckResourceAttr(dataSourceName, "rule.0.default_retention.0.days", "1"),
			resource.TestCheckNoResourceAttr(dataSourceName, "rule.0.default_retention.0.years"),
		)
	}

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationDataSourceConfig_objectLockEnabled(rName, "Disabled"),
				Check:  check("Disabled"),
			},
			{
				Config: testAccBucketObjectLockConfigurationDataSourceConfig_objectLockEnabled(rName, "Enabled"),
				Check:  check("Enabled"),
			},
			{
				PreConfig: testAccPreConfigDeleteBucket(ctx, t, rName),
				Config:    testAccBucketObjectLockConfigurationConfig_empty(),
			},
		},
	})
}

func testAccBucketObjectLockConfigurationDataSourceConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  object_lock_enabled = "Disabled"

  rule {
    default_retention {
      mode = %[2]q
      days = 3
    }
  }
}

data "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket_object_lock_configuration.test.bucket
}
`, rName, types.ObjectLockRetentionModeCompliance)
}

func testAccBucketObjectLockConfigurationDataSourceConfig_daysRetention(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  object_lock_enabled = "Disabled"

  rule {
    default_retention {
      mode = %[2]q
      days = 7
    }
  }
}

data "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket_object_lock_configuration.test.bucket
}
`, rName, types.ObjectLockRetentionModeCompliance)
}

func testAccBucketObjectLockConfigurationDataSourceConfig_yearsRetention(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  object_lock_enabled = "Disabled"

  rule {
    default_retention {
      mode  = %[2]q
      years = 2
    }
  }
}

data "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket_object_lock_configuration.test.bucket
}
`, rName, types.ObjectLockRetentionModeCompliance)
}

func testAccBucketObjectLockConfigurationDataSourceConfig_withoutRule(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

data "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket
}
`, rName)
}

func testAccBucketObjectLockConfigurationDataSourceConfig_objectLockConfigurationWithoutRule(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  object_lock_enabled = "Disabled"
}

data "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket_object_lock_configuration.test.bucket
}
`, rName)
}

func testAccBucketObjectLockConfigurationDataSourceConfig_objectLockEnabled(rName, objectLockEnabled string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  object_lock_enabled = %[2]q

  rule {
    default_retention {
      mode = %[3]q
      days = 1
    }
  }
}

data "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket_object_lock_configuration.test.bucket
}
`, rName, objectLockEnabled, types.ObjectLockRetentionModeCompliance)
}

func testAccBucketObjectLockConfigurationDataSourceConfig_retention(rName string, days int) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  object_lock_enabled = "Disabled"

  rule {
    default_retention {
      mode = %[2]q
      days = %[3]d
    }
  }
}

data "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket_object_lock_configuration.test.bucket
}
`, rName, types.ObjectLockRetentionModeCompliance, days)
}

func testAccBucketObjectLockConfigurationDataSourceConfig_bucketNotFound(rName string) string {
	return fmt.Sprintf(`
data "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = %[1]q
}
`, rName)
}
