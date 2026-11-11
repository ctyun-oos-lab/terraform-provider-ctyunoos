// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package s3_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/YakDriver/regexache"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	tfs3 "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/s3"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TC-S3-LOGGING-01 Basic Enable - target bucket and prefix
// TestAccS3BucketLogging_BasicEnable_withPrefix verifies that a target bucket and an explicit prefix can be created and read back correctly.
func TestAccS3BucketLogging_BasicEnable_withPrefix(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_logging.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLoggingDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLoggingConfig_basic(rName, "log/"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttrPair(resourceName, "target_bucket", "ctyun_oos_bucket.log_bucket", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "target_prefix", "log/"),
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

// TC-S3-LOGGING-01 Basic Enable - target bucket only
// TestAccS3BucketLogging_BasicEnable_withoutPrefix verifies that logging can be created without target_prefix and stably reads back an empty prefix.
func TestAccS3BucketLogging_BasicEnable_withoutPrefix(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_logging.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLoggingDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLoggingConfig_targetBucketOnly(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttrPair(resourceName, "target_bucket", "ctyun_oos_bucket.log_bucket", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "target_prefix", ""),
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

// TC-S3-LOGGING-06 Input Validation And Disappears - logging configuration disappears
// TestAccS3BucketLogging_InputValidationAndDisappears_loggingDisappears verifies that after the remote logging configuration is deleted, the resource produces a re-creation plan.
func TestAccS3BucketLogging_InputValidationAndDisappears_loggingDisappears(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_logging.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLoggingDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLoggingConfig_basic(rName, "log/"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccBucketLoggingConfig_basic(rName, "log/"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					acctest.CheckSDKResourceDisappears(ctx, t, tfs3.ResourceBucketLogging(), resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TC-S3-LOGGING-06 Input Validation And Disappears - source bucket disappears
// TestAccS3BucketLogging_InputValidationAndDisappears_bucketDisappears verifies that after the source Bucket is deleted remotely, the logging resource produces a re-creation plan.
func TestAccS3BucketLogging_InputValidationAndDisappears_bucketDisappears(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_logging.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLoggingDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLoggingConfig_basic(rName, "log/"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					acctest.CheckSDKResourceDisappears(ctx, t, tfs3.ResourceBucket(), "ctyun_oos_bucket.test"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TC-S3-LOGGING-02 Update Target Bucket - valid target bucket
// TestAccS3BucketLogging_UpdateTargetBucket_success verifies that after switching to another valid target bucket, the remote configuration and state stay in sync.
func TestAccS3BucketLogging_UpdateTargetBucket_success(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_logging.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLoggingDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLoggingConfig_basic(rName, "log/"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, "target_bucket", "ctyun_oos_bucket.log_bucket", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "target_prefix", "log/"),
				),
			},

			{
				Config: testAccBucketLoggingConfig_updateTargetBucket(rName, "log/"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, "target_bucket", "ctyun_oos_bucket.alt_log_bucket", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "target_prefix", "log/"),
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

// TC-S3-LOGGING-02 Update Target Bucket - target bucket and prefix together
// TestAccS3BucketLogging_UpdateTargetBucketAndPrefix_success verifies that after updating target_bucket and target_prefix together, the remote configuration and state stay in sync.
func TestAccS3BucketLogging_UpdateTargetBucketAndPrefix_success(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_logging.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLoggingDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLoggingConfig_basic(rName, "log/"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, "target_bucket", "ctyun_oos_bucket.log_bucket", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "target_prefix", "log/"),
				),
			},
			{
				Config: testAccBucketLoggingConfig_updateTargetBucket(rName, "tf-logs/"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, "target_bucket", "ctyun_oos_bucket.alt_log_bucket", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "target_prefix", "tf-logs/"),
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

// TC-S3-LOGGING-02 Update Target Bucket - bucket ForceNew
// TestAccS3BucketLogging_UpdateBucket_forceNew verifies that modifying the bucket attribute replaces the logging resource instead of updating it in place.
func TestAccS3BucketLogging_UpdateBucket_forceNew(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_logging.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLoggingDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLoggingConfig_forceNew(rName, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "bucket", rName),
					resource.TestCheckResourceAttrPair(resourceName, "bucket", "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttrPair(resourceName, "target_bucket", "ctyun_oos_bucket.log_bucket", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "target_prefix", "log/"),
				),
			},
			{
				Config: testAccBucketLoggingConfig_forceNew(rName, rName+"-new"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "bucket", rName+"-new"),
					resource.TestCheckResourceAttrPair(resourceName, "bucket", "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttrPair(resourceName, "target_bucket", "ctyun_oos_bucket.log_bucket", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "target_prefix", "log/"),
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

// TC-S3-LOGGING-02 Update Target Bucket - invalid target bucket name
// TestAccS3BucketLogging_UpdateTargetBucket_invalidName verifies that updating to an illegal target bucket name returns a clear error.
func TestAccS3BucketLogging_UpdateTargetBucket_invalidName(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLoggingDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLoggingConfig_basic(rName, "log/"),
			},
			{
				Config:      testAccBucketLoggingConfig_targetBucketLiteral(rName, "invalid_bucket"),
				ExpectError: regexache.MustCompile(`(?i)must contain only lowercase letters, numbers, periods, and hyphens`),
			},
			{
				Config: testAccBucketLoggingConfig_basic(rName, "log/"),
				Check:  testAccCheckBucketLoggingExists(ctx, t, "ctyun_oos_bucket_logging.test"),
			},
		},
	})
}

// TC-S3-LOGGING-02 Update Target Bucket - nonexistent target bucket
// TestAccS3BucketLogging_UpdateTargetBucket_nonexistent verifies that updating to a non-existent target bucket returns an error.
func TestAccS3BucketLogging_UpdateTargetBucket_nonexistent(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	targetBucket := fmt.Sprintf("%s-missing-log", rName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLoggingDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLoggingConfig_basic(rName, "log/"),
			},
			{
				Config:      testAccBucketLoggingConfig_targetBucketLiteral(rName, targetBucket),
				ExpectError: regexache.MustCompile(`(?i)InvalidTargetBucketForLogging`),
			},
		},
	})
}

// TC-S3-LOGGING-02 Update Target Bucket - inaccessible target bucket
// TestAccS3BucketLogging_UpdateTargetBucket_accessDenied verifies that OOS returns InvalidTargetBucketForLogging when the target bucket is not writable.
func TestAccS3BucketLogging_UpdateTargetBucket_accessDenied(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	targetBucket := acctest.SkipIfEnvVarNotSet(t, "CTYUNOOS_OTHER_ACCOUNT_BUCKET_NAME")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLoggingDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLoggingConfig_basic(rName, "log/"),
			},
			{
				Config:      testAccBucketLoggingConfig_targetBucketLiteral(rName, targetBucket),
				ExpectError: regexache.MustCompile(`(?i)InvalidTargetBucketForLogging`),
			},
		},
	})
}

// TC-S3-LOGGING-03 Update Target Prefix - explicit prefix
// TestAccS3BucketLogging_UpdateTargetPrefix_explicit verifies that after updating to an explicit prefix, the remote configuration and state stay in sync.
func TestAccS3BucketLogging_UpdateTargetPrefix_explicit(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_logging.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLoggingDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLoggingConfig_basic(rName, "log/"),
			},
			{
				Config: testAccBucketLoggingConfig_basic(rName, "tf-logs/"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, "target_bucket", "ctyun_oos_bucket.log_bucket", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "target_prefix", "tf-logs/"),
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

// TC-S3-LOGGING-03 Update Target Prefix - special characters
// TestAccS3BucketLogging_UpdateTargetPrefix_specialCharacters verifies that a prefix containing spaces, Unicode, and path segments reads back stably after update.
func TestAccS3BucketLogging_UpdateTargetPrefix_specialCharacters(t *testing.T) {
	testCases := []struct {
		name   string
		prefix string
	}{
		{name: "spaces", prefix: "logs with spaces/"},
		{name: "unicode", prefix: "日志/"},
		{name: "path-segments", prefix: "../logs/"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := acctest.Context(t)
			rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
			resourceName := "ctyun_oos_bucket_logging.test"

			acctest.ParallelTest(ctx, t, resource.TestCase{
				PreCheck:                 func() { acctest.PreCheck(ctx, t) },
				ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
				ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
				CheckDestroy:             testAccCheckBucketLoggingDestroy(ctx, t),
				Steps: []resource.TestStep{
					{
						Config: testAccBucketLoggingConfig_basic(rName, "log/"),
					},
					{
						Config: testAccBucketLoggingConfig_basic(rName, testCase.prefix),
						Check: resource.ComposeAggregateTestCheckFunc(
							testAccCheckBucketLoggingExists(ctx, t, resourceName),
							resource.TestCheckResourceAttrPair(resourceName, "target_bucket", "ctyun_oos_bucket.log_bucket", names.AttrBucket),
							resource.TestCheckResourceAttr(resourceName, "target_prefix", testCase.prefix),
						),
					},
					{
						ResourceName:      resourceName,
						ImportState:       true,
						ImportStateVerify: true,
					},
				},
			})
		})
	}
}

// TC-S3-LOGGING-03 Update Target Prefix - omitted to explicit prefix
// TestAccS3BucketLogging_UpdateTargetPrefix_omittedToExplicit verifies that explicitly setting a prefix after target_prefix was unset reads back stably.
func TestAccS3BucketLogging_UpdateTargetPrefix_omittedToExplicit(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_logging.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLoggingDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLoggingConfig_targetBucketOnly(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "target_prefix", ""),
				),
			},
			{
				Config: testAccBucketLoggingConfig_basic(rName, "tf-logs/"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, "target_bucket", "ctyun_oos_bucket.log_bucket", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "target_prefix", "tf-logs/"),
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

// TC-S3-LOGGING-03 Update Target Prefix - empty prefix
// TestAccS3BucketLogging_UpdateTargetPrefix_empty verifies that updating to an explicit empty-string prefix reads back the empty value stably.
func TestAccS3BucketLogging_UpdateTargetPrefix_empty(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_logging.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLoggingDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLoggingConfig_basic(rName, "log/"),
			},
			{
				Config: testAccBucketLoggingConfig_basic(rName, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, "target_bucket", "ctyun_oos_bucket.log_bucket", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "target_prefix", ""),
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

// TC-S3-LOGGING-03 Update Target Prefix - omitted prefix
// TestAccS3BucketLogging_UpdateTargetPrefix_omitted verifies that removing target_prefix reads back the empty value stably.
func TestAccS3BucketLogging_UpdateTargetPrefix_omitted(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_logging.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLoggingDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLoggingConfig_basic(rName, "log/"),
			},
			{
				Config: testAccBucketLoggingConfig_targetBucketOnly(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, "target_bucket", "ctyun_oos_bucket.log_bucket", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "target_prefix", ""),
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

// TC-S3-LOGGING-03 Update Target Prefix - overlong prefix
// TestAccS3BucketLogging_UpdateTargetPrefix_invalid verifies that updating to a prefix exceeding the supported length is rejected by the server.
func TestAccS3BucketLogging_UpdateTargetPrefix_invalid(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_logging.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLoggingDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLoggingConfig_basic(rName, "log/"),
			},
			{
				Config:      testAccBucketLoggingConfig_basic(rName, strings.Repeat("a", 1025)),
				ExpectError: regexache.MustCompile(`(?i)(InvalidArgument|Invalid.*Prefix|prefix.*(invalid|length|too long))`),
			},
			{
				Config: testAccBucketLoggingConfig_basic(rName, "log/"),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TC-S3-LOGGING-04 Remove Logging
// TestAccS3BucketLogging_RemoveLogging_fromConfig verifies that after the logging resource is removed, the remote configuration is fully disabled without leaving a target bucket behind.
func TestAccS3BucketLogging_RemoveLogging_fromConfig(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_logging.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLoggingDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLoggingConfig_basic(rName, "log/"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "target_prefix", "log/"),
				),
			},
			{
				Config: testAccBucketLoggingConfig_bucketOnly(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingDisabled(ctx, t, rName),
				),
			},
			{
				Config: testAccBucketLoggingConfig_basic(rName, "log/"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, "target_bucket", "ctyun_oos_bucket.log_bucket", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "target_prefix", "log/"),
				),
			},
		},
	})
}

// TC-S3-LOGGING-06 Input Validation And Disappears - source bucket invalid inputs
// TestAccS3BucketLogging_InputValidationAndDisappears_bucketInvalidInputs verifies that invalid source bucket inputs all return clear errors.
func TestAccS3BucketLogging_InputValidationAndDisappears_bucketInvalidInputs(t *testing.T) {
	testCases := []struct {
		name        string
		config      func(string) string
		errorRegexp string
	}{
		{
			name:        "empty-source-bucket",
			config:      func(rName string) string { return testAccBucketLoggingConfig_sourceBucketLiteral(rName, "") },
			errorRegexp: `expected length of bucket to be in the range \(3 - 63\), got`,
		},
		{
			name: "nonexistent-source-bucket",
			config: func(rName string) string {
				return testAccBucketLoggingConfig_sourceBucketLiteral(rName, fmt.Sprintf("%s-missing-source", rName))
			},
			errorRegexp: `api error NoSuchBucket`,
		},
		{
			name:        "consecutive-dots",
			config:      func(rName string) string { return testAccBucketLoggingConfig_sourceBucketLiteral(rName, "foo..bar") },
			errorRegexp: `(?i)api error InvalidBucketName`,
		},
		{
			name:        "uppercase",
			config:      func(rName string) string { return testAccBucketLoggingConfig_sourceBucketLiteral(rName, "Foo.Bar") },
			errorRegexp: `must contain only lowercase letters, numbers, periods, and hyphens`,
		},
		{
			name:        "public-ipv4",
			config:      func(rName string) string { return testAccBucketLoggingConfig_sourceBucketLiteral(rName, "192.168.0.1") },
			errorRegexp: `(?i)api error InvalidBucketName`,
		},
		{
			name:        "loopback-ipv4",
			config:      func(rName string) string { return testAccBucketLoggingConfig_sourceBucketLiteral(rName, "127.0.0.1") },
			errorRegexp: `(?i)api error InvalidBucketName`,
		},
		{
			name:        "leading-dot",
			config:      func(rName string) string { return testAccBucketLoggingConfig_sourceBucketLiteral(rName, ".foo") },
			errorRegexp: `(?i)api error InvalidBucketName`,
		},
		{
			name:        "trailing-dot",
			config:      func(rName string) string { return testAccBucketLoggingConfig_sourceBucketLiteral(rName, "bar.") },
			errorRegexp: `(?i)api error InvalidBucketName`,
		},
		{
			name:        "underscore",
			config:      func(rName string) string { return testAccBucketLoggingConfig_sourceBucketLiteral(rName, "foo_bar") },
			errorRegexp: `must contain only lowercase letters, numbers, periods, and hyphens`,
		},
		{
			name: "too-short",
			config: func(rName string) string {
				return testAccBucketLoggingConfig_sourceBucketLiteral(rName, strings.Repeat("a", 2))
			},
			errorRegexp: `expected length of bucket to be in the range \(3 - 63\), got`,
		},
		{
			name: "too-long",
			config: func(rName string) string {
				return testAccBucketLoggingConfig_sourceBucketLiteral(rName, strings.Repeat("a", 64))
			},
			errorRegexp: `expected length of bucket to be in the range \(3 - 63\), got`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := acctest.Context(t)
			rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

			acctest.ParallelTest(ctx, t, resource.TestCase{
				PreCheck:                 func() { acctest.PreCheck(ctx, t) },
				ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
				ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      testCase.config(rName),
						ExpectError: regexache.MustCompile(testCase.errorRegexp),
					},
				},
			})
		})
	}
}

// TC-S3-LOGGING-06 Input Validation And Disappears - invalid inputs
// TestAccS3BucketLogging_InputValidationAndDisappears_targetBucketInvalidInputs verifies that invalid target bucket inputs all return clear errors.
func TestAccS3BucketLogging_InputValidationAndDisappears_targetBucketInvalidInputs(t *testing.T) {
	testCases := []struct {
		name        string
		config      func(string) string
		errorRegexp string
	}{
		{
			name:        "empty-target-bucket",
			config:      func(rName string) string { return testAccBucketLoggingConfig_targetBucketLiteral(rName, "") },
			errorRegexp: `(?i)(invalid value for target_bucket|expected length of target_bucket)`,
		},
		{
			name: "nonexistent-target-bucket",
			config: func(rName string) string {
				return testAccBucketLoggingConfig_targetBucketLiteral(rName, fmt.Sprintf("%s-missing-target", rName))
			},
			errorRegexp: `(?i)InvalidTargetBucketForLogging`,
		},
		{
			name:        "invalid-target-consecutive-dots",
			config:      func(rName string) string { return testAccBucketLoggingConfig_targetBucketLiteral(rName, "foo..bar") },
			errorRegexp: `(?i)InvalidTargetBucketForLogging`,
		},
		{
			name:        "invalid-target-uppercase",
			config:      func(rName string) string { return testAccBucketLoggingConfig_targetBucketLiteral(rName, "Foo.Bar") },
			errorRegexp: `(?i)(invalid value for target_bucket|expected length of target_bucket)`,
		},
		{
			name:        "invalid-target-public-ipv4",
			config:      func(rName string) string { return testAccBucketLoggingConfig_targetBucketLiteral(rName, "192.168.0.1") },
			errorRegexp: `(?i)InvalidTargetBucketForLogging`,
		},
		{
			name:        "invalid-target-loopback-ipv4",
			config:      func(rName string) string { return testAccBucketLoggingConfig_targetBucketLiteral(rName, "127.0.0.1") },
			errorRegexp: `(?i)InvalidTargetBucketForLogging`,
		},
		{
			name:        "invalid-target-leading-dot",
			config:      func(rName string) string { return testAccBucketLoggingConfig_targetBucketLiteral(rName, ".foo") },
			errorRegexp: `(?i)InvalidTargetBucketForLogging`,
		},
		{
			name:        "invalid-target-trailing-dot",
			config:      func(rName string) string { return testAccBucketLoggingConfig_targetBucketLiteral(rName, "bar.") },
			errorRegexp: `(?i)InvalidTargetBucketForLogging`,
		},
		{
			name:        "invalid-target-underscore",
			config:      func(rName string) string { return testAccBucketLoggingConfig_targetBucketLiteral(rName, "foo_bar") },
			errorRegexp: `(?i)(invalid value for target_bucket|expected length of target_bucket)`,
		},
		{
			name: "invalid-target-too-long",
			config: func(rName string) string {
				return testAccBucketLoggingConfig_targetBucketLiteral(rName, strings.Repeat("a", 64))
			},
			errorRegexp: `(?i)(invalid value for target_bucket|expected length of target_bucket)`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := acctest.Context(t)
			rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

			acctest.ParallelTest(ctx, t, resource.TestCase{
				PreCheck:                 func() { acctest.PreCheck(ctx, t) },
				ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
				ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      testCase.config(rName),
						ExpectError: regexache.MustCompile(testCase.errorRegexp),
					},
				},
			})
		})
	}
}

// TC-S3-LOGGING-05 Prefix Idempotence - explicit empty prefix
// TestAccS3BucketLogging_PrefixIdempotence_empty verifies that an explicit empty-string prefix reads back stably and repeated plans produce no change.
func TestAccS3BucketLogging_PrefixIdempotence_empty(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_logging.test"
	config := testAccBucketLoggingConfig_basic(rName, "")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLoggingDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, "target_bucket", "ctyun_oos_bucket.log_bucket", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "target_prefix", ""),
				),
			},
			{
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TC-S3-LOGGING-05 Prefix Idempotence - omitted prefix
// TestAccS3BucketLogging_PrefixIdempotence_omitted verifies that an unset prefix reads back stably and repeated plans produce no change.
func TestAccS3BucketLogging_PrefixIdempotence_omitted(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_logging.test"
	config := testAccBucketLoggingConfig_targetBucketOnly(rName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLoggingDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, "target_bucket", "ctyun_oos_bucket.log_bucket", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "target_prefix", ""),
				),
			},
			{
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TC-S3-LOGGING-05 Prefix Idempotence - empty to omitted to empty prefix import
// TestAccS3BucketLogging_PrefixIdempotence_emptyToOmittedToEmpty_import verifies the ImportStateVerify behavior when target_prefix switches between an explicit empty value and an omitted value.
func TestAccS3BucketLogging_PrefixIdempotence_emptyToOmittedToEmpty_import(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_logging.test"

	emptyConfig := testAccBucketLoggingConfig_basic(rName, "")
	omittedConfig := testAccBucketLoggingConfig_targetBucketOnly(rName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLoggingDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: emptyConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttrPair(resourceName, "target_bucket", "ctyun_oos_bucket.log_bucket", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "target_prefix", ""),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: omittedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttrPair(resourceName, "target_bucket", "ctyun_oos_bucket.log_bucket", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "target_prefix", ""),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: emptyConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLoggingExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttrPair(resourceName, "target_bucket", "ctyun_oos_bucket.log_bucket", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "target_prefix", ""),
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

func testAccCheckBucketLoggingDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_oos_bucket_logging" {
				continue
			}

			_, err := tfs3.FindLoggingEnabled(ctx, conn, rs.Primary.Attributes[names.AttrBucket])

			if retry.NotFound(err) {
				continue
			}

			if err != nil {
				return err
			}

			return fmt.Errorf("OOS Bucket Logging %s still exists", rs.Primary.ID)
		}

		return nil
	}
}

func testAccCheckBucketLoggingExists(ctx context.Context, t *testing.T, n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		_, err := tfs3.FindLoggingEnabled(ctx, conn, rs.Primary.Attributes[names.AttrBucket])

		return err
	}
}

func testAccCheckBucketLoggingDisabled(ctx context.Context, t *testing.T, bucket string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		_, err := tfs3.FindLoggingEnabled(ctx, conn, bucket)
		if retry.NotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}

		return fmt.Errorf("OOS Bucket Logging for bucket %s still exists", bucket)
	}
}

func testAccBucketLoggingConfig_forceNew(rName, sourceBucket string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "log_bucket" {
  bucket = "%[1]s-log"
}

resource "ctyun_oos_bucket" "test" {
  bucket = %[2]q
}

resource "ctyun_oos_bucket_logging" "test" {
  bucket        = ctyun_oos_bucket.test.bucket
  target_bucket = ctyun_oos_bucket.log_bucket.bucket
  target_prefix = "log/"
}
`, rName, sourceBucket)
}
func testAccBucketLoggingConfig_basic(rName, targetPrefix string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "log_bucket" {
  bucket = "%[1]s-log"
}

resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_logging" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  target_bucket = ctyun_oos_bucket.log_bucket.bucket
  target_prefix = %[2]q
}
`, rName, targetPrefix)
}

func testAccBucketLoggingConfig_targetBucketOnly(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "log_bucket" {
  bucket = "%[1]s-log"
}

resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_logging" "test" {
  bucket        = ctyun_oos_bucket.test.bucket
  target_bucket = ctyun_oos_bucket.log_bucket.bucket
}
`, rName)
}
func testAccBucketLoggingConfig_updateTargetBucket(rName, targetPrefix string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "log_bucket" {
  bucket = "%[1]s-log"
}

resource "ctyun_oos_bucket" "alt_log_bucket" {
  bucket = "%[1]s-alt-log"
}

resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_logging" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  target_bucket = ctyun_oos_bucket.alt_log_bucket.bucket
  target_prefix = %[2]q
}
`, rName, targetPrefix)
}

func testAccBucketLoggingConfig_sourceBucketLiteral(rName, sourceBucket string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "log_bucket" {
  bucket = "%[1]s-log"
}

resource "ctyun_oos_bucket_logging" "test" {
  bucket = %[2]q

  target_bucket = ctyun_oos_bucket.log_bucket.bucket
  target_prefix = "log/"
}
`, rName, sourceBucket)
}
func testAccBucketLoggingConfig_targetBucketLiteral(rName, targetBucket string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "log_bucket" {
  bucket = "%[1]s-log"
}

resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_logging" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  target_bucket = %[2]q
  target_prefix = "log/"
}
`, rName, targetBucket)
}
func testAccBucketLoggingConfig_bucketOnly(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "log_bucket" {
  bucket = "%[1]s-log"
}

resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}
`, rName)
}
