// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	tfs3 "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/s3"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TC-S3-LOCK-01 Basic configuration supplement: verifies the Disabled state, the resource ID, and import read-back.
func TestAccS3BucketObjectLockConfiguration_BasicConfig_disabled(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketObjectLockConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationConfig_basic(rName, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "id", rName),
					resource.TestCheckResourceAttr(resourceName, "object_lock_enabled", "Disabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.days", "1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TC-S3-LOCK-01 Basic configuration: verifies that the default retention policy reads back correctly after enabling object lock. (The bucket is deleted to avoid the test framework erroring when deleting an enabled object lock.)
func TestAccS3BucketObjectLockConfiguration_BasicConfig_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketObjectLockConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationConfig_enabled(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "object_lock_enabled", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.days", "1"),
				),
			},
			{
				PreConfig: testAccPreConfigDeleteBucket(ctx, t, rName),
				Config:    testAccBucketObjectLockConfigurationConfig_empty(),
			}},
	})
}

// TC-S3-LOCK-02 Enable/disable round trip: verifies the state can be updated from Disabled to Enabled. (The bucket is deleted to avoid the test framework erroring when deleting an enabled object lock.)
func TestAccS3BucketObjectLockConfiguration_EnableDisableRoundTrip_disabledToEnabled(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketObjectLockConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationConfig_basic(rName, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "object_lock_enabled", "Disabled"),
				),
			},
			{
				Config: testAccBucketObjectLockConfigurationConfig_enabled(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "object_lock_enabled", "Enabled"),
				),
			},
			{
				PreConfig: testAccPreConfigDeleteBucket(ctx, t, rName),
				Config:    testAccBucketObjectLockConfigurationConfig_empty(),
			}},
	})
}

// TC-S3-LOCK-02 Enable/disable round trip: verifies the state cannot be updated from Enabled to Disabled.
func TestAccS3BucketObjectLockConfiguration_EnableDisableRoundTrip_enabledToDisabled(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketObjectLockConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationConfig_enabled(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "object_lock_enabled", "Enabled"),
				),
			},
			{
				Config:      testAccBucketObjectLockConfigurationConfig_basic(rName, 1),
				ExpectError: regexache.MustCompile(`(?i)(InvalidArgument.*enabled.*can not be disabled|enabled.*can not be disabled)`),
			},
			{
				PreConfig: testAccPreConfigDeleteBucket(ctx, t, rName),
				Config:    testAccBucketObjectLockConfigurationConfig_empty(),
			}},
	})
}

// TC-S3-LOCK-02 Enable/disable round trip: verifies that unsupported enum values are rejected.
func TestAccS3BucketObjectLockConfiguration_EnableDisableRoundTrip_invalidValue(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketObjectLockConfigurationConfig_objectLockEnabled(rName, "Invalid"),
				ExpectError: regexache.MustCompile(`(?s)expected object_lock_enabled to be one of.*Enabled.*Disabled`),
			},
		},
	})
}

// TC-S3-LOCK-03 Retention policy update: verifies that the state reads back correctly after the retention period is extended or shortened while Disabled.
func TestAccS3BucketObjectLockConfiguration_RetentionUpdate_disabledextend(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketObjectLockConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationConfig_basic(rName, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.days", "1"),
				),
			},
			{
				Config: testAccBucketObjectLockConfigurationConfig_basic(rName, 5),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.days", "5"),
				),
			},
			{
				Config: testAccBucketObjectLockConfigurationConfig_basic(rName, 3),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.days", "3"),
				),
			},
			{
				Config: testAccBucketObjectLockConfigurationConfig_years(rName, 20),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.years", "20"),
				),
			},
			{
				Config: testAccBucketObjectLockConfigurationConfig_years(rName, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.years", "1"),
				),
			},
			{
				Config: testAccBucketObjectLockConfigurationConfig_basic(rName, 40),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.days", "40"),
				),
			},
		},
	})
}

// TC-S3-LOCK-03 Retention policy update: verifies that the state reads back correctly after the retention period is extended while Enabled.
func TestAccS3BucketObjectLockConfiguration_RetentionUpdate_enabledExtend(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketObjectLockConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationConfig_enabledRetention(rName, "days", 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "object_lock_enabled", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.days", "1"),
				),
			},
			{
				Config: testAccBucketObjectLockConfigurationConfig_enabledRetention(rName, "days", 5),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "object_lock_enabled", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.days", "5"),
				),
			},
			{
				Config: testAccBucketObjectLockConfigurationConfig_enabledRetention(rName, "years", 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "object_lock_enabled", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.years", "1"),
				),
			},
			{
				Config: testAccBucketObjectLockConfigurationConfig_enabledRetention(rName, "days", 500),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "object_lock_enabled", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.days", "500"),
				),
			},
			{
				PreConfig: testAccPreConfigDeleteBucket(ctx, t, rName),
				Config:    testAccBucketObjectLockConfigurationConfig_empty(),
			},
		},
	})
}

// TC-S3-LOCK-03 Retention policy update: verifies that shortening the retention period while Enabled is rejected.
func TestAccS3BucketObjectLockConfiguration_RetentionUpdate_shorten(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketObjectLockConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationConfig_enabledRetention(rName, "days", 5),
			},
			{
				Config:      testAccBucketObjectLockConfigurationConfig_enabledRetention(rName, "days", 1),
				ExpectError: regexache.MustCompile(`(?i)(InvalidArgument.*Default retention period cannot be less than previously value|Default retention period cannot be less than previously value)`),
			},
			{
				PreConfig: testAccPreConfigDeleteBucket(ctx, t, rName),
				Config:    testAccBucketObjectLockConfigurationConfig_empty(),
			},
		},
	})
}

// TC-S3-LOCK-03 Retention policy update: verifies that changing to a non-COMPLIANCE mode is rejected by the schema.
func TestAccS3BucketObjectLockConfiguration_RetentionUpdate_invalidMode(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketObjectLockConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationConfig_basic(rName, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "object_lock_enabled", "Disabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.days", "1"),
				),
			},
			{
				Config:      testAccBucketObjectLockConfigurationConfig_retentionMode(rName, "GOVERNANCE"),
				ExpectError: regexache.MustCompile(`(?is)expected rule\.0\.default_retention\.0\.mode to be one of.*COMPLIANCE`),
			},
			{
				Config: testAccBucketObjectLockConfigurationConfig_basic(rName, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "object_lock_enabled", "Disabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.days", "1"),
				),
			},
		},
	})
}

// TC-S3-LOCK-03 Retention policy update: verifies that the state reads back correctly after updating from having a rule to having no rule.
func TestAccS3BucketObjectLockConfiguration_RetentionUpdate_removeRule(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketObjectLockConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationConfig_basic(rName, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "object_lock_enabled", "Disabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.days", "1"),
				),
			},
			{
				Config: testAccBucketObjectLockConfigurationConfig_withoutRule(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "object_lock_enabled", "Disabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "0"),
				),
			},
			{
				Config:             testAccBucketObjectLockConfigurationConfig_withoutRule(rName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TC-S3-LOCK-03 Retention policy update: verifies that the state reads back correctly after updating from having no rule to having a rule.
func TestAccS3BucketObjectLockConfiguration_RetentionUpdate_addRule(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketObjectLockConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationConfig_withoutRule(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "object_lock_enabled", "Disabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "0"),
				),
			},
			{
				Config: testAccBucketObjectLockConfigurationConfig_basic(rName, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "object_lock_enabled", "Disabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.days", "1"),
				),
			},
			{
				Config:             testAccBucketObjectLockConfigurationConfig_basic(rName, 1),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TC-S3-LOCK-03 Retention policy update: verifies successful creation and stable read-back when no rule is configured.
func TestAccS3BucketObjectLockConfiguration_RetentionUpdate_withoutRule(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketObjectLockConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationConfig_withoutRule(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "object_lock_enabled", "Disabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "0"),
				),
			},
			{
				Config:             testAccBucketObjectLockConfigurationConfig_withoutRule(rName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TC-S3-LOCK-03 Retention policy update: verifies successful creation and stable read-back when no rule is configured while Enabled.
func TestAccS3BucketObjectLockConfiguration_RetentionUpdate_withoutRuleEnabled(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketObjectLockConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationConfig_enabledWithoutRule(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "object_lock_enabled", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "0"),
				),
			},
			{
				Config:             testAccBucketObjectLockConfigurationConfig_enabledWithoutRule(rName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				PreConfig: testAccPreConfigDeleteBucket(ctx, t, rName),
				Config:    testAccBucketObjectLockConfigurationConfig_empty(),
			},
		},
	})
}

// TC-S3-LOCK-04 Days/years constraints: verifies that the minimum days value is valid.
func TestAccS3BucketObjectLockConfiguration_DaysYearsConstraints_daysMinimum(t *testing.T) {
	testAccBucketObjectLockConfigurationRetentionPeriod(t, "days", 1)
}

// TC-S3-LOCK-04 Days/years constraints: verifies that the maximum days value is valid.
func TestAccS3BucketObjectLockConfiguration_DaysYearsConstraints_daysMaximum(t *testing.T) {
	testAccBucketObjectLockConfigurationRetentionPeriod(t, "days", 36500)
}

// TC-S3-LOCK-04 Days/years constraints: verifies that days exceeding the maximum is rejected.
func TestAccS3BucketObjectLockConfiguration_DaysYearsConstraints_daysOutOfRange(t *testing.T) {
	testAccBucketObjectLockConfigurationInvalidRetentionPeriod(t, "days", 36501, `(?i)expected rule.0.default_retention.0.days to be in the range \(1 - 36500\)`)
}

// TC-S3-LOCK-04 Days/years constraints: verifies that the minimum years value is valid.
func TestAccS3BucketObjectLockConfiguration_DaysYearsConstraints_yearsMinimum(t *testing.T) {
	testAccBucketObjectLockConfigurationRetentionPeriod(t, "years", 1)
}

// TC-S3-LOCK-04 Days/years constraints: verifies that the maximum years value is valid.
func TestAccS3BucketObjectLockConfiguration_DaysYearsConstraints_yearsMaximum(t *testing.T) {
	testAccBucketObjectLockConfigurationRetentionPeriod(t, "years", 100)
}

// TC-S3-LOCK-04 Days/years constraints: verifies that years exceeding the maximum is rejected.
func TestAccS3BucketObjectLockConfiguration_DaysYearsConstraints_yearsOutOfRange(t *testing.T) {
	testAccBucketObjectLockConfigurationInvalidRetentionPeriod(t, "years", 101, `(?i)expected rule.0.default_retention.0.years to be in the range \(1 - 100\)`)
}

// TC-S3-LOCK-05: after removing the resource from Terraform, verifies the remote configuration is retained and can be taken over again.
func TestAccS3BucketObjectLockConfiguration_RemoveConfig(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketObjectLockConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationConfig_enabled(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "object_lock_enabled", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.days", "1"),
				),
			},
			{
				Config:      testAccBucketObjectLockConfigurationConfig_removed(rName),
				ExpectError: regexache.MustCompile(`(?i)CanNotDeleteObjectLockConfiguration.*enabled.*can not be deleted`),
			},
			{
				Config: testAccBucketObjectLockConfigurationConfig_enabled(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "object_lock_enabled", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.days", "1"),
				),
			},
			{
				Config:             testAccBucketObjectLockConfigurationConfig_enabled(rName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				PreConfig: testAccPreConfigDeleteBucket(ctx, t, rName),
				Config:    testAccBucketObjectLockConfigurationConfig_empty(),
			},
		},
	})
}

// TC-S3-LOCK-05: after removing the Disabled resource from the Terraform configuration, verifies the remote configuration is deleted with no residue.
func TestAccS3BucketObjectLockConfiguration_RemoveConfig_disabledDeletesRemote(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketObjectLockConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationConfig_basic(rName, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "object_lock_enabled", "Disabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.days", "1"),
				),
			},
			{
				Config: testAccBucketObjectLockConfigurationConfig_removed(rName),
				Check:  testAccCheckBucketObjectLockConfigurationNotExistsByBucket(ctx, t, rName),
			},
			{
				Config: testAccBucketObjectLockConfigurationConfig_empty(),
			},
		},
	})
}

// TC-S3-LOCK-04 Days/years constraints: verifies that configuring both days and years is rejected by the schema.
func TestAccS3BucketObjectLockConfiguration_DaysYearsConstraints_daysAndYearsConflict(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketObjectLockConfigurationConfig_daysAndYears(rName),
				ExpectError: regexache.MustCompile(`(?s)Invalid combination of arguments.*days.*years.*Invalid combination of arguments.*years.*days`),
			},
		},
	})
}

// TC-S3-LOCK-04 Days/years constraints: verifies that days of 0 is rejected by the schema.
func TestAccS3BucketObjectLockConfiguration_DaysYearsConstraints_daysZero(t *testing.T) {
	testAccBucketObjectLockConfigurationInvalidRetentionPeriod(t, "days", 0, `(?i)expected rule\.0\.default_retention\.0\.days to be in the range \(1 - 36500\)`)
}

// TC-S3-LOCK-04 Days/years constraints: verifies that years of 0 is rejected by the schema.
func TestAccS3BucketObjectLockConfiguration_DaysYearsConstraints_yearsZero(t *testing.T) {
	testAccBucketObjectLockConfigurationInvalidRetentionPeriod(t, "years", 0, `(?i)expected rule\.0\.default_retention\.0\.years to be in the range \(1 - 100\)`)
}

// TC-S3-LOCK-03: verifies that a rule present but missing default_retention is rejected by the schema.
func TestAccS3BucketObjectLockConfiguration_RetentionUpdate_missingDefaultRetention(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketObjectLockConfigurationConfig_missingDefaultRetention(rName),
				ExpectError: regexache.MustCompile(`(?is)(At least 1 "default_retention" blocks are required|default_retention.*required)`),
			},
		},
	})
}

func testAccBucketObjectLockConfigurationRetentionPeriod(t *testing.T, unit string, value int) {
	t.Helper()

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"
	attributeName := fmt.Sprintf("rule.0.default_retention.0.%s", unit)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketObjectLockConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationConfig_retentionPeriod(rName, unit, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttr(resourceName, attributeName, fmt.Sprintf("%d", value)),
				),
			},
		},
	})
}

func testAccBucketObjectLockConfigurationInvalidRetentionPeriod(t *testing.T, unit string, value int, errorRegexp string) {
	t.Helper()

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketObjectLockConfigurationConfig_retentionPeriod(rName, unit, value),
				ExpectError: regexache.MustCompile(errorRegexp),
			},
		},
	})
}

// TC-S3-LOCK-07: Effect On Object Operations: overwriting the same-key object fails, deleting the object fails, batch deletion fails, and some multipart upload/complete/copy part operations fail, while writes and reads of different keys still succeed
func TestAccS3BucketObjectLockConfiguration_EffectOnObjectOperations(t *testing.T) {
	acctest.SkipIfNotRunningAcceptanceTests(t)

	bucket := acctest.SkipIfEnvVarNotSet(t, "CTYUNOOS_SAME_ACCOUNT_BUCKET_NAME")
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)

	conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)
	if _, err := tfs3.FindObjectLockConfiguration(ctx, conn, bucket); err != nil {
		t.Fatalf("reading object lock configuration for dedicated bucket %q: %s", bucket, err)
	}

	// Test setup: create an object protected by the default retention policy, and reserve a different key for normal read/write verification.
	prefix := fmt.Sprintf("tf-acc-object-lock-effects/%s", acctest.RandomWithPrefix(t, "object-lock"))
	protectedKey := prefix + "/protected"
	writableKey := prefix + "/different-key"

	_, err := conn.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(protectedKey),
		Body:   strings.NewReader("protected"),
	}, testAccOOSPutObjectOptions)
	if err != nil {
		t.Fatalf("creating protected object %q: %s", protectedKey, err)
	}

	// Locked-object basic operations: same-key overwrite, single-object deletion, and batch deletion should all be rejected.
	t.Run("overwrite", func(t *testing.T) {
		_, err := conn.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(protectedKey),
			Body:   strings.NewReader("overwrite"),
		}, testAccOOSPutObjectOptions)
		testAccCheckObjectLockOperationDenied(t, "overwriting protected object", err)
	})

	t.Run("delete", func(t *testing.T) {
		_, err := conn.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(protectedKey),
		})
		testAccCheckObjectLockOperationDenied(t, "deleting protected object", err)
	})

	t.Run("bulk delete", func(t *testing.T) {
		output, err := conn.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(bucket),
			Delete: &types.Delete{
				Objects: []types.ObjectIdentifier{{Key: aws.String(protectedKey)}},
				Quiet:   aws.Bool(true),
			},
		}, tfs3.WithContentMD5)
		if err != nil {
			testAccCheckObjectLockOperationDenied(t, "bulk deleting protected object", err)
			return
		}
		if len(output.Errors) != 1 {
			t.Fatalf("expected one protected-object bulk delete error, got %#v", output.Errors)
		}
		if code := aws.ToString(output.Errors[0].Code); !testAccObjectLockOperationDeniedCode(code) {
			t.Fatalf("unexpected protected-object bulk delete error code %q", code)
		}
	})

	// Non-locked object operations: a different key can still be written and read normally.
	t.Run("different key put and get", func(t *testing.T) {
		_, err := conn.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(writableKey),
			Body:   strings.NewReader("readable"),
		}, testAccOOSPutObjectOptions)
		if err != nil {
			t.Fatalf("creating object at a different key: %s", err)
		}

		output, err := conn.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(writableKey),
		})
		if err != nil {
			t.Fatalf("reading object at a different key: %s", err)
		}
		defer output.Body.Close()

		body, err := io.ReadAll(output.Body)
		if err != nil {
			t.Fatalf("reading different-key object body: %s", err)
		}
		if got, want := string(body), "readable"; got != want {
			t.Fatalf("different-key object body = %q, want %q", got, want)
		}
	})

	// Multipart upload operations: uploading parts to or copying parts into the locked object should be rejected.
	t.Run("multipart complete", func(t *testing.T) {
		testAccCheckObjectLockMultipartCompleteDenied(ctx, t, conn, bucket, protectedKey)
	})

	t.Run("copy part", func(t *testing.T) {
		testAccCheckObjectLockCopyPartDenied(ctx, t, conn, bucket, protectedKey, writableKey)
	})
}

func testAccCheckObjectLockOperationDenied(t *testing.T, operation string, err error) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected %s to be rejected by object lock", operation)
	}
	if !testAccObjectLockOperationDeniedCode(err.Error()) {
		t.Fatalf("expected object-lock rejection for %s, got: %s", operation, err)
	}
}

func testAccObjectLockOperationDeniedCode(value string) bool {
	value = strings.ToLower(value)

	return strings.Contains(value, "accessdenied") ||
		strings.Contains(value, "fileimmutable") ||
		strings.Contains(value, "objectlock") ||
		strings.Contains(value, "retention") ||
		strings.Contains(value, "not allowed")
}

func testAccCheckObjectLockMultipartCompleteDenied(ctx context.Context, t *testing.T, conn *s3.Client, bucket, key string) {
	t.Helper()

	created, err := conn.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		testAccCheckObjectLockOperationDenied(t, "creating multipart upload for protected object", err)
		return
	}
	defer func() {
		// Best-effort cleanup; ignore abort errors.
		_, _ = conn.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(bucket),
			Key:      aws.String(key),
			UploadId: created.UploadId,
		})
	}()

	part, err := conn.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		PartNumber: aws.Int32(1),
		UploadId:   created.UploadId,
		Body:       strings.NewReader("multipart overwrite"),
	})
	if err != nil {
		testAccCheckObjectLockOperationDenied(t, "uploading multipart part for protected object", err)
		return
	}

	_, err = conn.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: created.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: []types.CompletedPart{{ETag: part.ETag, PartNumber: aws.Int32(1)}},
		},
	})
	testAccCheckObjectLockOperationDenied(t, "completing multipart upload for protected object", err)
}

func testAccCheckObjectLockCopyPartDenied(ctx context.Context, t *testing.T, conn *s3.Client, bucket, protectedKey, sourceKey string) {
	t.Helper()

	created, err := conn.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(protectedKey),
	})
	if err != nil {
		testAccCheckObjectLockOperationDenied(t, "creating copy multipart upload for protected object", err)
		return
	}
	defer func() {
		// Best-effort cleanup; ignore abort errors.
		_, _ = conn.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(bucket),
			Key:      aws.String(protectedKey),
			UploadId: created.UploadId,
		})
	}()

	_, err = conn.UploadPartCopy(ctx, &s3.UploadPartCopyInput{
		Bucket:     aws.String(bucket),
		CopySource: aws.String(bucket + "/" + sourceKey),
		Key:        aws.String(protectedKey),
		PartNumber: aws.Int32(1),
		UploadId:   created.UploadId,
	})
	testAccCheckObjectLockOperationDenied(t, "copying multipart part to protected object", err)
}

// TC-S3-LOCK-08 Read/delete validation and disappearance handling: verifies that all eight categories of illegal bucket names return clear errors.
func TestAccS3BucketObjectLockConfiguration_ReadDeleteValidationAndDisappears_invalidBucketName(t *testing.T) {
	invalidNames := []struct {
		name        string
		bucket      string
		errorRegexp string
	}{
		{name: "consecutive-dots", bucket: "foo..bar", errorRegexp: `api error InvalidBucketName`},
		{name: "uppercase", bucket: "Foo.Bar", errorRegexp: `(?is)must contain only lowercase letters, numbers, periods, and\s+hyphens`},
		{name: "private-ipv4", bucket: "192.168.0.1", errorRegexp: `(?i)api error InvalidBucketName`},
		{name: "loopback-ipv4", bucket: "127.0.0.1", errorRegexp: `(?i)api error InvalidBucketName`},
		{name: "leading-dot", bucket: ".foo", errorRegexp: `(?i)api error InvalidBucketName`},
		{name: "trailing-dot", bucket: "bar.", errorRegexp: `(?i)api error InvalidBucketName`},
		{name: "underscore", bucket: "foo_bar", errorRegexp: `(?is)must contain only lowercase letters, numbers, periods, and\s+hyphens`},
		{name: "too-long", bucket: strings.Repeat("a", 64), errorRegexp: `(?i)expected length of bucket to be in the range \(3 - 63\)`},
	}

	for _, testCase := range invalidNames {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := acctest.Context(t)

			acctest.ParallelTest(ctx, t, resource.TestCase{
				PreCheck:                 func() { acctest.PreCheck(ctx, t) },
				ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
				ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      testAccBucketObjectLockConfigurationConfig_unmanagedBucket(testCase.bucket),
						ExpectError: regexache.MustCompile(testCase.errorRegexp),
					},
				},
			})
		})
	}
}

// TC-S3-LOCK-08 Read/delete validation and disappearance handling: verifies that an empty bucket name is rejected.
func TestAccS3BucketObjectLockConfiguration_ReadDeleteValidationAndDisappears_emptyBucket(t *testing.T) {
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketObjectLockConfigurationConfig_unmanagedBucket(""),
				ExpectError: regexache.MustCompile(`(?i)(expected length of bucket to be in the range \(3 - 63\)|invalid value for bucket)`),
			},
		},
	})
}

// TC-S3-LOCK-08 Read/delete validation and disappearance handling: verifies that a non-existent bucket is rejected.
func TestAccS3BucketObjectLockConfiguration_ReadDeleteValidationAndDisappears_bucketNotFound(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketObjectLockConfigurationConfig_unmanagedBucket(rName),
				ExpectError: regexache.MustCompile(`(?i)(NoSuchBucket|not.?found|status.?404)`),
			},
		},
	})
}

// TC-S3-LOCK-08 Read/delete validation and disappearance handling: sends a delete request for a bucket with no remote lock configuration, expecting a clear error.
func TestAccS3BucketObjectLockConfiguration_ReadDeleteValidationAndDisappears_deleteMissingConfiguration(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketObjectLockConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationConfig_bucketOnly(rName),
				Check:  testAccCheckBucketObjectLockConfigurationNotExistsByBucket(ctx, t, rName),
			},
			{
				PreConfig: func() {
					err := tfs3.OOSDeleteBucketObjectLock(ctx, acctest.ProviderMeta(ctx, t).S3Client(ctx), rName)
					if err == nil {
						t.Fatal("expected deleting a nonexistent object lock configuration to return an error")
					}
					if !strings.Contains(err.Error(), "ObjectLockConfigurationNotFound") {
						t.Fatalf("expected ObjectLockConfigurationNotFound when deleting a nonexistent object lock configuration, got: %s", err)
					}
				},
				Config: testAccBucketObjectLockConfigurationConfig_bucketOnly(rName),
				Check:  testAccCheckBucketObjectLockConfigurationNotExistsByBucket(ctx, t, rName),
			},
		},
	})
}

// TC-S3-LOCK-08 Read/delete validation and disappearance handling: verifies that the resource plans re-creation after the remote bucket is deleted.
func TestAccS3BucketObjectLockConfiguration_ReadDeleteValidationAndDisappears_bucketDisappears(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketObjectLockConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationConfig_basic(rName, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					acctest.CheckSDKResourceDisappears(ctx, t, tfs3.ResourceBucket(), "ctyun_oos_bucket.test"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TC-S3-LOCK-09 Import: verifies importing the object lock configuration by bucket name.
func TestAccS3BucketObjectLockConfiguration_Import_bucket(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_object_lock_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketObjectLockConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketObjectLockConfigurationConfig_enabled(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketObjectLockConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "object_lock_enabled", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.mode", "COMPLIANCE"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.default_retention.0.days", "1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateId:     rName,
				ImportStateVerify: true,
			},
			{
				PreConfig: testAccPreConfigDeleteBucket(ctx, t, rName),
				Config:    testAccBucketObjectLockConfigurationConfig_empty(),
			}},
	})
}

// TC-S3-LOCK-04 Days and years constraints supplement: verifies that the configuration is rejected when both days and years are missing.
func TestAccS3BucketObjectLockConfiguration_DaysYearsConstraints_missingRetentionPeriod(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketObjectLockConfigurationConfig_missingRetentionPeriod(rName),
				ExpectError: regexache.MustCompile(`(?s)Invalid combination of arguments.*one of.*days.*years.*must be\s+specified`),
			},
		},
	})
}

// TC-S3-LOCK-10 Required field validation: verifies that the resource configuration is rejected when object_lock_enabled is missing.
func TestAccS3BucketObjectLockConfiguration_RequiredFields_missingObjectLockEnabled(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketObjectLockConfigurationConfig_missingObjectLockEnabled(rName),
				ExpectError: regexache.MustCompile(`(?is)(Missing required argument.*object_lock_enabled|object_lock_enabled.*(required|must be specified)|required field is not set)`),
			},
		},
	})
}

func testAccCheckBucketObjectLockConfigurationDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_oos_bucket_object_lock_configuration" {
				continue
			}

			_, err := tfs3.FindObjectLockConfiguration(ctx, conn, rs.Primary.Attributes[names.AttrBucket])

			if retry.NotFound(err) {
				continue
			}

			if err != nil {
				return err
			}

			return fmt.Errorf("OOS Bucket Object Lock Configuration (%s) still exists", rs.Primary.ID)
		}

		return nil
	}
}

func testAccPreConfigDeleteBucket(ctx context.Context, t *testing.T, bucket string) func() {
	return func() {
		t.Helper()

		_, err := acctest.ProviderMeta(ctx, t).S3Client(ctx).DeleteBucket(ctx, &s3.DeleteBucketInput{
			Bucket: aws.String(bucket),
		})
		if err != nil {
			t.Fatalf("deleting bucket %q before Terraform cleanup: %s", bucket, err)
		}
	}
}
func testAccCheckBucketObjectLockConfigurationExists(ctx context.Context, t *testing.T, n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		_, err := tfs3.FindObjectLockConfiguration(ctx, conn, rs.Primary.Attributes[names.AttrBucket])
		if retry.NotFound(err) {
			return fmt.Errorf("OOS Bucket Object Lock Configuration (%s) not found", rs.Primary.Attributes[names.AttrBucket])
		}

		return err
	}
}

func testAccCheckBucketObjectLockConfigurationNotExistsByBucket(ctx context.Context, t *testing.T, bucket string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		_, err := tfs3.FindObjectLockConfiguration(ctx, conn, bucket)
		if retry.NotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}

		return fmt.Errorf("OOS Bucket Object Lock Configuration (%s) still exists", bucket)
	}
}
func testAccBucketObjectLockConfigurationConfig_missingRetentionPeriod(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

}

resource "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  object_lock_enabled = "Disabled"

  rule {
    default_retention {
      mode = "COMPLIANCE"
    }
  }
}
`, rName)
}

func testAccBucketObjectLockConfigurationConfig_missingObjectLockEnabled(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    default_retention {
      mode = "COMPLIANCE"
      days = 1
    }
  }
}
`, rName)
}

func testAccBucketObjectLockConfigurationConfig_retentionMode(rName, mode string) string {
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
      days = 1
    }
  }
}
`, rName, mode)
}
func testAccBucketObjectLockConfigurationConfig_withoutRule(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  object_lock_enabled = "Disabled"
}
`, rName)
}
func testAccBucketObjectLockConfigurationConfig_enabledWithoutRule(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  object_lock_enabled = "Enabled"
}
`, rName)
}
func testAccBucketObjectLockConfigurationConfig_basic(rName string, days int) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  object_lock_enabled = "Disabled"

  rule {
    default_retention {
      mode = "COMPLIANCE"
      days = %[2]d
    }
  }
}
`, rName, days)
}

func testAccBucketObjectLockConfigurationConfig_years(rName string, years int) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

}

resource "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  object_lock_enabled = "Disabled"

  rule {
    default_retention {
      mode  = "COMPLIANCE"
      years = %[2]d
    }
  }
}
`, rName, years)
}

func testAccBucketObjectLockConfigurationConfig_enabled(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  object_lock_enabled = "Enabled"

  rule {
    default_retention {
      mode = "COMPLIANCE"
      days = 1
    }
  }
}
`, rName)
}

func testAccBucketObjectLockConfigurationConfig_objectLockEnabled(rName, objectLockEnabled string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

}

resource "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  object_lock_enabled = %[2]q

  rule {
    default_retention {
      mode = "COMPLIANCE"
      days = 1
    }
  }
}
`, rName, objectLockEnabled)
}

func testAccBucketObjectLockConfigurationConfig_daysAndYears(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  object_lock_enabled = "Disabled"

  rule {
    default_retention {
      mode  = "COMPLIANCE"
      days  = 1
      years = 1
    }
  }
}
`, rName)
}
func testAccBucketObjectLockConfigurationConfig_missingDefaultRetention(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  object_lock_enabled = "Disabled"

  rule {}
}
`, rName)
}
func testAccBucketObjectLockConfigurationConfig_retentionPeriod(rName, unit string, value int) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

}

resource "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  object_lock_enabled = "Disabled"

  rule {
    default_retention {
      mode  = "COMPLIANCE"
      %[2]s = %[3]d
    }
  }
}
`, rName, unit, value)
}

func testAccBucketObjectLockConfigurationConfig_bucketOnly(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}
`, rName)
}

func testAccBucketObjectLockConfigurationConfig_unmanagedBucket(bucket string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = %[1]q

  object_lock_enabled = "Disabled"

  rule {
    default_retention {
      mode = "COMPLIANCE"
      days = 1
    }
  }
}
`, bucket)
}

func testAccBucketObjectLockConfigurationConfig_empty() string {
	return `
provider "ctyun" {}
`
}

func testAccBucketObjectLockConfigurationConfig_enabledRetention(rName, unit string, value int) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

}

resource "ctyun_oos_bucket_object_lock_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  object_lock_enabled = "Enabled"

  rule {
    default_retention {
      mode  = "COMPLIANCE"
      %[2]s = %[3]d
    }
  }
}
`, rName, unit, value)
}

func testAccBucketObjectLockConfigurationConfig_removed(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}
`, rName)
}
