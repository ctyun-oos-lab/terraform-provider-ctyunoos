// Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
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
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TC-OOS-DSOFT-01 Basic Read - reads the data source after creating the soft delete resource, checking that days matches.
func TestAccS3BucketSoftDeleteDataSource_BasicRead_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_soft_delete_configuration.test"
	resourceName := "ctyun_oos_bucket_soft_delete_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketSoftDeleteDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketSoftDeleteDataSourceConfig_basic(rName, 7),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttrPair(dataSourceName, "days", resourceName, "days"),
					resource.TestCheckResourceAttr(dataSourceName, "days", "7"),
				),
			},
		},
	})
}

// TC-OOS-DSOFT-04 Bucket Not Found - reading the soft delete configuration of a non-existent bucket returns a clear error.
func TestAccS3BucketSoftDeleteDataSource_BucketNotFound_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketSoftDeleteDataSourceConfig_bucket(rName),
				ExpectError: regexache.MustCompile(`(?is)reading OOS Bucket.*Soft Delete.*api error NoSuchBucket`),
			},
		},
	})
}

// TC-OOS-DSOFT-07 Bucket Validation - covers various illegal bucket inputs and verifies clear errors are returned.
func TestAccS3BucketSoftDeleteDataSource_BucketValidation_invalidName(t *testing.T) {
	testCases := []struct {
		name        string
		bucket      string
		errorRegexp string
	}{
		{name: "consecutive-dots", bucket: "foo..bar", errorRegexp: `(?i)api error InvalidBucketName`},
		{name: "uppercase", bucket: "Foo.Bar", errorRegexp: `(?is)must contain only lowercase letters, numbers, periods, and hyphens`},
		{name: "public-ipv4", bucket: "192.168.0.1", errorRegexp: `(?i)api error InvalidBucketName`},
		{name: "leading-dot", bucket: ".foo", errorRegexp: `(?i)api error InvalidBucketName`},
		{name: "underscore", bucket: "foo_bar", errorRegexp: `(?is)must contain only lowercase letters, numbers, periods, and hyphens`},
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
						Config:      testAccBucketSoftDeleteDataSourceConfig_bucket(testCase.bucket),
						ExpectError: regexache.MustCompile(testCase.errorRegexp),
					},
				},
			})
		})
	}
}

// TC-OOS-DSOFT-02 Default Configuration Readback - reads the server-side default days without declaring a soft delete resource, and verifies repeated reads are stable.
func TestAccS3BucketSoftDeleteDataSource_DefaultConfigurationReadback_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_soft_delete_configuration.test"
	config := testAccBucketSoftDeleteDataSourceConfig_defaultConfiguration(rName)
	var defaultDay string

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources[dataSourceName]
						if !ok {
							return fmt.Errorf("not found: %s", dataSourceName)
						}

						defaultDay = rs.Primary.Attributes["days"]
						if defaultDay == "" {
							return fmt.Errorf("%s days is empty", dataSourceName)
						}

						return nil
					},
				),
			},
			{
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources[dataSourceName]
						if !ok {
							return fmt.Errorf("not found: %s", dataSourceName)
						}

						if got := rs.Primary.Attributes["days"]; got != defaultDay {
							return fmt.Errorf("%s days = %q, want stable value %q", dataSourceName, got, defaultDay)
						}

						return nil
					},
				),
			},
		},
	})
}

// TC-OOS-DSOFT-03 Explicit To Default Transition - reads the current remote value via the data source after removing the explicit resource.
func TestAccS3BucketSoftDeleteDataSource_ExplicitToDefaultTransition_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_soft_delete_configuration.test"
	resourceName := "ctyun_oos_bucket_soft_delete_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketSoftDeleteDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketSoftDeleteDataSourceConfig_basic(rName, 13),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttrPair(dataSourceName, "days", resourceName, "days"),
					resource.TestCheckResourceAttr(dataSourceName, "days", "13"),
				),
			},
			{
				Config: testAccBucketSoftDeleteDataSourceConfig_bucketOnly(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketSoftDeleteNotInState(resourceName),
					resource.TestCheckResourceAttr("ctyun_oos_bucket.test", names.AttrBucket, rName),
				),
			},
			{
				Config: testAccBucketSoftDeleteDataSourceConfig_defaultConfiguration(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					testAccCheckBucketSoftDeleteDataSourceDaysNonEmpty(dataSourceName),
				),
			},
		},
	})
}

// TC-OOS-DSOFT-05 Resource Consistency - reads via the data source immediately after updating the resource's days and checks consistency.
func TestAccS3BucketSoftDeleteDataSource_ResourceConsistency_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_soft_delete_configuration.test"
	resourceName := "ctyun_oos_bucket_soft_delete_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketSoftDeleteDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketSoftDeleteDataSourceConfig_basic(rName, 4),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttrPair(dataSourceName, "days", resourceName, "days"),
					resource.TestCheckResourceAttr(dataSourceName, "days", "4"),
				),
			},
			{
				Config: testAccBucketSoftDeleteDataSourceConfig_basic(rName, 12),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttrPair(dataSourceName, "days", resourceName, "days"),
					resource.TestCheckResourceAttr(dataSourceName, "days", "12"),
				),
			},
			{
				Config: testAccBucketSoftDeleteDataSourceConfig_basic(rName, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttrPair(dataSourceName, "days", resourceName, "days"),
					resource.TestCheckResourceAttr(dataSourceName, "days", "1"),
				),
			},
			{
				Config: testAccBucketSoftDeleteDataSourceConfig_basic(rName, 15),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, rName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
					resource.TestCheckResourceAttrPair(dataSourceName, "days", resourceName, "days"),
					resource.TestCheckResourceAttr(dataSourceName, "days", "15"),
				),
			},
		},
	})
}

// TC-OOS-DSOFT-06 Read Stability - refreshes a bucket without explicit configuration multiple times, verifying the default days are stable with no perpetual diff.
func TestAccS3BucketSoftDeleteDataSource_ReadStability_defaultConfiguration(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_soft_delete_configuration.test"
	config := testAccBucketSoftDeleteDataSourceConfig_defaultConfiguration(rName)
	var defaultDay string

	check := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, rName),
		resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
	)
	captureDefaultDay := resource.ComposeAggregateTestCheckFunc(
		check,
		testAccCaptureBucketSoftDeleteDataSourceDays(dataSourceName, &defaultDay),
	)
	checkStableDefaultDay := resource.ComposeAggregateTestCheckFunc(
		check,
		testAccCheckBucketSoftDeleteDataSourceDaysEquals(dataSourceName, &defaultDay),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  check,
			},
			{
				RefreshState: true,
				Check:        captureDefaultDay,
			},
			{
				RefreshState: true,
				Check:        checkStableDefaultDay,
			},
			{
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TC-OOS-DSOFT-06 Read Stability - refreshes multiple times after explicit configuration, verifying the data source and resource days are stable, consistent, and produce no perpetual diff.
func TestAccS3BucketSoftDeleteDataSource_ReadStability_explicitConfiguration(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_oos_bucket_soft_delete_configuration.test"
	resourceName := "ctyun_oos_bucket_soft_delete_configuration.test"
	config := testAccBucketSoftDeleteDataSourceConfig_basic(rName, 9)

	check := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrPair(dataSourceName, names.AttrBucket, resourceName, names.AttrBucket),
		resource.TestCheckResourceAttr(dataSourceName, names.AttrBucket, rName),
		resource.TestCheckResourceAttr(dataSourceName, names.AttrID, rName),
		resource.TestCheckResourceAttrPair(dataSourceName, "days", resourceName, "days"),
		resource.TestCheckResourceAttr(dataSourceName, "days", "9"),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketSoftDeleteDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  check,
			},
			{
				RefreshState: true,
				Check:        check,
			},
			{
				RefreshState: true,
				Check:        check,
			},
			{
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

func testAccCheckBucketSoftDeleteDataSourceDaysNonEmpty(dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("not found: %s", dataSourceName)
		}

		if days := rs.Primary.Attributes["days"]; days == "" {
			return fmt.Errorf("%s days is empty", dataSourceName)
		}

		return nil
	}
}

func testAccCaptureBucketSoftDeleteDataSourceDays(dataSourceName string, target *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("not found: %s", dataSourceName)
		}

		days := rs.Primary.Attributes["days"]
		if days == "" {
			return fmt.Errorf("%s days is empty", dataSourceName)
		}
		*target = days

		return nil
	}
}

func testAccCheckBucketSoftDeleteDataSourceDaysEquals(dataSourceName string, expected *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("not found: %s", dataSourceName)
		}
		if *expected == "" {
			return fmt.Errorf("%s expected days was not captured", dataSourceName)
		}

		if days := rs.Primary.Attributes["days"]; days != *expected {
			return fmt.Errorf("%s days = %q, want stable value %q", dataSourceName, days, *expected)
		}

		return nil
	}
}

func testAccBucketSoftDeleteDataSourceConfig_basic(rName string, days int) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_soft_delete_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket
  days   = %[2]d
}

data "ctyun_oos_bucket_soft_delete_configuration" "test" {
  bucket = ctyun_oos_bucket_soft_delete_configuration.test.bucket
}
`, rName, days)
}

func testAccBucketSoftDeleteDataSourceConfig_bucket(bucket string) string {
	return fmt.Sprintf(`
data "ctyun_oos_bucket_soft_delete_configuration" "test" {
  bucket = %[1]q
}
`, bucket)
}

func testAccBucketSoftDeleteDataSourceConfig_bucketOnly(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}
`, rName)
}

func testAccBucketSoftDeleteDataSourceConfig_defaultConfiguration(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

data "ctyun_oos_bucket_soft_delete_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket
}
`, rName)
}
