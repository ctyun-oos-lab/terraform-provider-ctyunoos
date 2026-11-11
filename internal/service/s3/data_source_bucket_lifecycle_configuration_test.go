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

// TC-OOS-DLC-01 Basic Read: reads the data source after creating the lifecycle resource, verifying the read-back state matches the resource.
func TestAccS3BucketLifecycleConfigurationDataSource_BasicRead_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_lifecycle_configuration.test"
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationDataSourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(dataSourceName, "rule.#", "1"),
					resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.id", resourceName, "rule.0.id"),
					resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.status", resourceName, "rule.0.status"),
					resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.prefix", resourceName, "rule.0.prefix"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.expiration.#", "1"),
					resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.expiration.0.days", resourceName, "rule.0.expiration.0.days"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.transition.#", "0"),
				),
			},
		},
	})
}

// TC-OOS-DLC-04 Not Configured: reads a bucket that exists but has no lifecycle configured, verifying the current implementation returns a clear error.
func TestAccS3BucketLifecycleConfigurationDataSource_NotConfigured_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationDataSourceConfig_notConfigured(rName),
				ExpectError: regexache.MustCompile(`Bucket Lifecycle Configuration Data Source.*couldn't find resource`),
			},
		},
	})
}

// TC-OOS-DLC-07 Bucket Validation: covers read errors for illegal bucket names and an empty bucket name.
func TestAccS3BucketLifecycleConfigurationDataSource_BucketValidation_invalidName(t *testing.T) {
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
						Config:      testAccBucketLifecycleConfigurationDataSourceConfig_bucket(testCase.bucket),
						ExpectError: regexache.MustCompile(testCase.errorRegexp),
					},
				},
			})
		})
	}
}

// TC-OOS-DLC-07 Bucket Validation: verifies that a well-formed bucket that does not exist remotely returns a clear error.
func TestAccS3BucketLifecycleConfigurationDataSource_BucketValidation_nonexistent(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-missing-lifecycle")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationDataSourceConfig_bucket(bucketName),
				ExpectError: regexache.MustCompile(`Bucket Lifecycle Configuration Data Source.*couldn't find resource`),
			},
		},
	})
}

// TC-OOS-DLC-02 Transition Rule Readback: reads a lifecycle containing a transition, verifying that transition.days and transition.storage_class are correctly back-filled.
func TestAccS3BucketLifecycleConfigurationDataSource_TransitionRuleReadback_basic(t *testing.T) {
	t.Skip("temporarily skipped")

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_lifecycle_configuration.test"
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationDataSourceConfig_transition(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(dataSourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.id", rName),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.status", "Enabled"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.prefix", "logs/"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.expiration.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.transition.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.transition.0.days", "30"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.transition.0.storage_class", "STANDARD_IA"),
				),
			},
		},
	})
}

// TC-OOS-DLC-08 Transition Date Readback: reads a lifecycle containing transition.date, verifying the date is normalized into a stable RFC3339 form.
func TestAccS3BucketLifecycleConfigurationDataSource_TransitionDateReadback_basic(t *testing.T) {
	t.Skip("temporarily skipped")

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_lifecycle_configuration.test"
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"
	configuredDate := "2030-01-02T00:00:00.000Z"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationDataSourceConfig_transitionDate(rName, configuredDate),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(dataSourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.id", rName),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.status", "Enabled"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.prefix", "transition-date/"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.expiration.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.transition.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.transition.0.date", configuredDate),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.transition.0.storage_class", "STANDARD_IA"),
				),
			},
		},
	})
}

// TC-OOS-DLC-08 Transition Date Readback: verifies that an invalid RFC3339 transition.date is rejected at the lifecycle resource schema validation stage.
func TestAccS3BucketLifecycleConfigurationDataSource_TransitionDateReadback_invalidFormat(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationDataSourceConfig_transitionDate(rName, "2030-01-02"),
				ExpectError: regexache.MustCompile(`(?i)not valid RFC3339 string format`),
			},
		},
	})
}

// TC-OOS-DLC-08 Transition Date Readback: verifies that a transition.date not in UTC midnight form is rejected at the lifecycle resource schema validation stage.
func TestAccS3BucketLifecycleConfigurationDataSource_TransitionDateReadback_invalidTimezone(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationDataSourceConfig_transitionDate(rName, "2030-01-02T00:00:00.000+08:00"),
				ExpectError: regexache.MustCompile(`(?is)must be a UTC midnight timestamp in the\s+exact form`),
			},
		},
	})
}

// TC-OOS-DLC-03 Date Expiration Readback: reads a lifecycle containing expiration.date, verifying the date is normalized into a stable RFC3339 form.
func TestAccS3BucketLifecycleConfigurationDataSource_DateExpirationReadback_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_lifecycle_configuration.test"
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"
	dateValue := "2030-01-02T00:00:00.000Z"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationDataSourceConfig_date(rName, dateValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(dataSourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.id", rName),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.status", "Enabled"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.prefix", "date/"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.expiration.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.expiration.0.date", dateValue),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.transition.#", "0"),
				),
			},
		},
	})
}

// TC-OOS-DLC-09 Generated Rule ID Readback: reads the data source with rule.id omitted, verifying the auto-generated lifecycle rule ID is correctly back-filled.
func TestAccS3BucketLifecycleConfigurationDataSource_GeneratedRuleIDReadback_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_lifecycle_configuration.test"
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationDataSourceConfig_generatedRuleID(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(dataSourceName, "rule.#", "1"),
					resource.TestMatchResourceAttr(dataSourceName, "rule.0.id", regexache.MustCompile(`^tf-s3-lifecycle-`)),
					resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.id", resourceName, "rule.0.id"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.status", "Enabled"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.prefix", "generated/"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.expiration.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.expiration.0.days", "30"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.transition.#", "0"),
				),
			},
		},
	})
}

// TC-OOS-DLC-10 Empty Prefix Readback: reads a lifecycle rule whose prefix is an empty string, verifying the data source preserves the empty prefix.
func TestAccS3BucketLifecycleConfigurationDataSource_EmptyPrefixReadback_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_lifecycle_configuration.test"
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationDataSourceConfig_emptyPrefix(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(dataSourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.id", rName),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.status", "Enabled"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.prefix", ""),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.expiration.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.expiration.0.days", "30"),
					resource.TestCheckResourceAttr(dataSourceName, "rule.0.transition.#", "0"),
				),
			},
		},
	})
}

// TC-OOS-DLC-05 Multiple Rules Stability: refreshes repeatedly after reading multiple lifecycle rules, verifying rule order and field read-back remain stable.
func TestAccS3BucketLifecycleConfigurationDataSource_MultipleRulesStability_basic(t *testing.T) {
	t.Skip("temporarily skipped")

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_lifecycle_configuration.test"
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"
	config := testAccBucketLifecycleConfigurationDataSourceConfig_multipleRules(rName)
	check := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
		resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
		resource.TestCheckResourceAttr(dataSourceName, "rule.#", "2"),
		resource.TestCheckResourceAttr(dataSourceName, "rule.0.id", "days-stable"),
		resource.TestCheckResourceAttr(dataSourceName, "rule.0.status", "Enabled"),
		resource.TestCheckResourceAttr(dataSourceName, "rule.0.prefix", "days/"),
		resource.TestCheckResourceAttr(dataSourceName, "rule.0.expiration.#", "1"),
		resource.TestCheckResourceAttr(dataSourceName, "rule.0.expiration.0.days", "365"),
		resource.TestCheckResourceAttr(dataSourceName, "rule.0.transition.#", "0"),
		resource.TestCheckResourceAttr(dataSourceName, "rule.1.id", "transition-stable"),
		resource.TestCheckResourceAttr(dataSourceName, "rule.1.status", "Disabled"),
		resource.TestCheckResourceAttr(dataSourceName, "rule.1.prefix", "transition/"),
		resource.TestCheckResourceAttr(dataSourceName, "rule.1.expiration.#", "0"),
		resource.TestCheckResourceAttr(dataSourceName, "rule.1.transition.#", "1"),
		resource.TestCheckResourceAttr(dataSourceName, "rule.1.transition.0.days", "30"),
		resource.TestCheckResourceAttr(dataSourceName, "rule.1.transition.0.storage_class", "STANDARD_IA"),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{Config: config, Check: check},
			{RefreshState: true, Check: check},
			{RefreshState: true, Check: check},
			{Config: config, PlanOnly: true, ExpectNonEmptyPlan: false},
		},
	})
}

// TC-OOS-DLC-06 Resource Consistency: reads the data source immediately after updating the lifecycle resource, verifying both sides' state and action configuration are consistent.
func TestAccS3BucketLifecycleConfigurationDataSource_ResourceConsistency_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_lifecycle_configuration.test"
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"
	initialConfig := testAccBucketLifecycleConfigurationDataSourceConfig_consistency(rName, false)
	updatedConfig := testAccBucketLifecycleConfigurationDataSourceConfig_consistency(rName, true)

	checkInitial := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
		resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
		resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.id", resourceName, "rule.0.id"),
		resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.prefix", resourceName, "rule.0.prefix"),
		resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.status", resourceName, "rule.0.status"),
		resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.expiration.0.days", resourceName, "rule.0.expiration.0.days"),
		resource.TestCheckResourceAttr(dataSourceName, "rule.0.expiration.#", "1"),
		resource.TestCheckResourceAttr(dataSourceName, "rule.0.transition.#", "0"),
	)
	checkUpdated := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
		resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
		resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.id", resourceName, "rule.0.id"),
		resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.prefix", resourceName, "rule.0.prefix"),
		resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.status", resourceName, "rule.0.status"),
		resource.TestCheckResourceAttrPair(dataSourceName, "rule.0.expiration.0.days", resourceName, "rule.0.expiration.0.days"),
		resource.TestCheckResourceAttr(dataSourceName, "rule.0.expiration.#", "1"),
		resource.TestCheckResourceAttr(dataSourceName, "rule.0.transition.#", "0"),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{Config: initialConfig, Check: checkInitial},
			{Config: updatedConfig, Check: checkUpdated},
		},
	})
}

func testAccBucketLifecycleConfigurationDataSourceConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    id     = %[1]q
    prefix = "prefix/"
    status = "Enabled"

    expiration {
      days = 365
    }
  }
}

data "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket_lifecycle_configuration.test.bucket
}
`, rName)
}

func testAccBucketLifecycleConfigurationDataSourceConfig_bucket(bucket string) string {
	return fmt.Sprintf(`
data "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = %[1]q
}
`, bucket)
}

func testAccBucketLifecycleConfigurationDataSourceConfig_notConfigured(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

data "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket
}
`, rName)
}

func testAccBucketLifecycleConfigurationDataSourceConfig_transition(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    id     = %[1]q
    prefix = "logs/"
    status = "Enabled"

    transition {
      days          = 30
      storage_class = "STANDARD_IA"
    }
  }
}

data "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket_lifecycle_configuration.test.bucket
}
`, rName)
}

func testAccBucketLifecycleConfigurationDataSourceConfig_transitionDate(rName, dateValue string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    id     = %[1]q
    prefix = "transition-date/"
    status = "Enabled"

    transition {
      date          = %[2]q
      storage_class = "STANDARD_IA"
    }
  }
}

data "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket_lifecycle_configuration.test.bucket
}
`, rName, dateValue)
}

func testAccBucketLifecycleConfigurationDataSourceConfig_date(rName, dateValue string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    id     = %[1]q
    prefix = "date/"
    status = "Enabled"

    expiration {
      date = %[2]q
    }
  }
}

data "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket_lifecycle_configuration.test.bucket
}
`, rName, dateValue)
}

func testAccBucketLifecycleConfigurationDataSourceConfig_generatedRuleID(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    prefix = "generated/"
    status = "Enabled"

    expiration {
      days = 30
    }
  }
}

data "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket_lifecycle_configuration.test.bucket
}
`, rName)
}

func testAccBucketLifecycleConfigurationDataSourceConfig_emptyPrefix(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    id     = %[1]q
    prefix = ""
    status = "Enabled"

    expiration {
      days = 30
    }
  }
}

data "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket_lifecycle_configuration.test.bucket
}
`, rName)
}

func testAccBucketLifecycleConfigurationDataSourceConfig_multipleRules(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    id     = "days-stable"
    prefix = "days/"
    status = "Enabled"

    expiration {
      days = 365
    }
  }

  rule {
    id     = "transition-stable"
    prefix = "transition/"
    status = "Disabled"

    transition {
      days          = 30
      storage_class = "STANDARD_IA"
    }
  }
}

data "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket_lifecycle_configuration.test.bucket
}
`, rName)
}

func testAccBucketLifecycleConfigurationDataSourceConfig_consistency(rName string, updated bool) string {
	if updated {
		return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    id     = %[1]q
    prefix = "updated/"
    status = "Disabled"

    expiration {
      days = 45
    }
  }
}

data "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket_lifecycle_configuration.test.bucket
}
`, rName)
	}

	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    id     = %[1]q
    prefix = "initial/"
    status = "Enabled"

    expiration {
      days = 30
    }
  }
}

data "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket_lifecycle_configuration.test.bucket
}
`, rName)
}
