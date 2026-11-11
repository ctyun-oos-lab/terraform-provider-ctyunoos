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
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Verifies basic lifecycle rule creation, state read-back, key attributes, and the import flow.
// TC-S3-LC-13: basic lifecycle CRUD and import with an explicit rule ID.
func TestAccS3BucketLifecycleConfiguration_BasicCRUDAndImport(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.id", rName),
					resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", "prefix/"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.0.days", "365"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.#", "0"),
				),
			},
			{
				ResourceName: resourceName,
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[resourceName]
					if !ok {
						return "", fmt.Errorf("Not found: %s", resourceName)
					}

					return rs.Primary.Attributes[names.AttrBucket], nil
				},
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: names.AttrBucket,
			},
		},
	})
}

// TC-S3-LC-12: verifies that illegal bucket names, remotely non-existent buckets, and empty bucket names all return clear errors.
func TestAccS3BucketLifecycleConfiguration_BucketValidation_invalidName(t *testing.T) {
	invalidNames := []struct {
		name        string
		bucket      string
		errorRegexp string
	}{
		{name: "consecutive-dots", bucket: "foo..bar", errorRegexp: `InvalidBucketName`},
		{name: "uppercase", bucket: "Foo.Bar", errorRegexp: `(?is)must contain only lowercase letters, numbers, periods, and\s+hyphens`},
		{name: "private-ipv4", bucket: "192.168.0.1", errorRegexp: `InvalidBucketName`},
		{name: "loopback-ipv4", bucket: "127.0.0.1", errorRegexp: `InvalidBucketName`},
		{name: "leading-dot", bucket: ".foo", errorRegexp: `InvalidBucketName`},
		{name: "trailing-dot", bucket: "bar.", errorRegexp: `InvalidBucketName`},
		{name: "underscore", bucket: "foo_bar", errorRegexp: `(?is)must contain only lowercase letters, numbers, periods, and\s+hyphens`},
		{name: "too-long", bucket: strings.Repeat("a", 64), errorRegexp: `(?i)bucket string length must be between 3 and 63, got`},
		{name: "empty-string", bucket: "", errorRegexp: `(?i)bucket string length must be between 3 and 63, got`},
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
						Config:      testAccBucketLifecycleConfigurationConfig_lifecycleOnly(testCase.bucket),
						ExpectError: regexache.MustCompile(testCase.errorRegexp),
					},
				},
			})
		})
	}
}

// TC-S3-LC-12: verifies that a well-formed bucket name that does not exist remotely returns not found on the read/write paths.
func TestAccS3BucketLifecycleConfiguration_BucketValidation_nonexistent(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, "tf-missing-lifecycle")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationConfig_lifecycleOnly(bucketName),
				ExpectError: regexache.MustCompile("creating OOS Bucket .* Lifecycle Configuration"),
			},
		},
	})
}

// TC-S3-LC-12: verifies that an empty bucket name is not accepted on the create/read/delete paths.
func TestAccS3BucketLifecycleConfiguration_BucketValidation_empty(t *testing.T) {
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationConfig_lifecycleOnly(""),
				ExpectError: regexache.MustCompile(`Attribute bucket string length must be between 3 and 63, got`),
			},
		},
	})
}

// TC-S3-LC-01: verifies that the minimum valid expiration days of 1 can be created and read back correctly.
func TestAccS3BucketLifecycleConfiguration_ExpirationByDays_minimum(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationConfig_expirationDays(rName, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.id", rName),
					resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Disabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", "prefix/"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.0.days", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.#", "0"),
				),
			},
			{
				ResourceName: resourceName,
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[resourceName]
					if !ok {
						return "", fmt.Errorf("Not found: %s", resourceName)
					}
					return rs.Primary.Attributes[names.AttrBucket], nil
				},
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: names.AttrBucket,
			},
		},
	})
}

// TC-S3-LC-12: verifies that illegal bucket names, remotely non-existent buckets, and empty bucket names all return clear errors.
// TC-S3-LC-01: verifies that an expiration days of 0 is rejected by schema validation.
func TestAccS3BucketLifecycleConfiguration_ExpirationByDays_zero(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationConfig_expirationDays(rName, 0),
				ExpectError: regexache.MustCompile("(?i)must be at least 1"),
			},
		},
	})
}

// TC-S3-LC-12: verifies that illegal bucket names, remotely non-existent buckets, and empty bucket names all return clear errors.
// TC-S3-LC-01: verifies that a negative expiration days is rejected by schema validation.
func TestAccS3BucketLifecycleConfiguration_ExpirationByDays_negative(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationConfig_expirationDays(rName, -1),
				ExpectError: regexache.MustCompile("(?i)must be at least 1"),
			},
		},
	})
}

// TC-S3-LC-01: verifies that the minimum valid transition days of 1 is created successfully and read back in full.
func TestAccS3BucketLifecycleConfiguration_TransitionByDays_minimum(t *testing.T) {
	t.Skip("temporarily skipped")

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationConfig_transitionDays(rName, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.id", rName),
					resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", "transition/"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.0.days", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.0.storage_class", "STANDARD_IA"),
				),
			},
		},
	})
}

// TC-S3-LC-01: verifies that a transition days of 0 is rejected by schema validation.
func TestAccS3BucketLifecycleConfiguration_TransitionByDays_zero(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationConfig_transitionDays(rName, 0),
				ExpectError: regexache.MustCompile(`(?i)must be at least 1`),
			},
		},
	})
}

// TC-S3-LC-01: verifies that a negative transition days is rejected by schema validation.
func TestAccS3BucketLifecycleConfiguration_TransitionByDays_negative(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationConfig_transitionDays(rName, -1),
				ExpectError: regexache.MustCompile(`(?i)must be at least 1`),
			},
		},
	})
}

// TC-S3-LC-07: verifies that removing the lifecycle configuration deletes the remote rules, and that they can be re-created with the same ID and prefix.
func TestAccS3BucketLifecycleConfiguration_RemoveConfiguration(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"

	check := resource.ComposeAggregateTestCheckFunc(
		testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
		resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
		resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
		resource.TestCheckResourceAttr(resourceName, "rule.0.id", rName),
		resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Enabled"),
		resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", "prefix/"),
		resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.#", "1"),
		resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.0.days", "365"),
		resource.TestCheckResourceAttr(resourceName, "rule.0.transition.#", "0"),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationConfig_basic(rName),
				Check:  check,
			},
			{
				Config: testAccBucketLifecycleConfigurationConfig_bucketOnly(rName),
				Check:  testAccCheckBucketLifecycleConfigurationNotFound(ctx, t),
			},
			{
				Config: testAccBucketLifecycleConfigurationConfig_basic(rName),
				Check:  check,
			},
		},
	})
}

// Verifies that Terraform detects drift when the lifecycle configuration disappears remotely.
// TC-S3-LC-08: refreshes after the lifecycle configuration is deleted remotely, verifying the framework resource disappears and a re-creation plan is produced.
func TestAccS3BucketLifecycleConfiguration_Disappears_lifecycleConfiguration(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
					acctest.CheckFrameworkResourceDisappears(ctx, t, tfs3.ResourceBucketLifecycleConfiguration, resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// Verifies that drift is detected for the lifecycle configuration when the associated bucket disappears.
// TC-S3-LC-08: refreshes after the associated Bucket is deleted remotely, verifying the lifecycle resource produces a re-creation plan.
func TestAccS3BucketLifecycleConfiguration_Disappears_bucket(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
					acctest.CheckSDKResourceDisappears(ctx, t, tfs3.ResourceBucket(), "ctyun_oos_bucket.test"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TC-S3-LC-06: after repeated refresh and re-apply of a multi-rule configuration, the order stays stable and the plan no longer produces duplicate changes.
func TestAccS3BucketLifecycleConfiguration_RuleOrderStability(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"
	config := func() string { return testAccBucketLifecycleConfigurationConfig_multipleExpirationRules(rName, 30) }
	check := resource.ComposeAggregateTestCheckFunc(
		testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
		resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
		resource.TestCheckResourceAttr(resourceName, "rule.#", "2"),
		resource.TestCheckResourceAttr(resourceName, "rule.0.id", "days-target"),
		resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Enabled"),
		resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", "days/"),
		resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.#", "1"),
		resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.0.days", "30"),
		resource.TestCheckResourceAttr(resourceName, "rule.0.transition.#", "0"),
		resource.TestCheckResourceAttr(resourceName, "rule.1.id", "days-stable"),
		resource.TestCheckResourceAttr(resourceName, "rule.1.status", "Enabled"),
		resource.TestCheckResourceAttr(resourceName, "rule.1.prefix", "stable/"),
		resource.TestCheckResourceAttr(resourceName, "rule.1.expiration.#", "1"),
		resource.TestCheckResourceAttr(resourceName, "rule.1.expiration.0.days", "365"),
		resource.TestCheckResourceAttr(resourceName, "rule.1.transition.#", "0"),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{Config: config(), Check: check},
			{RefreshState: true, Check: check},
			{Config: config(), PlanOnly: true, ExpectNonEmptyPlan: false},
		},
	})
}

// TC-S3-LC-03: verifies that a rule containing only a transition can be created, with a valid storage_class read back correctly.
func TestAccS3BucketLifecycleConfiguration_TransitionVariants_transitionOnly(t *testing.T) {
	t.Skip("temporarily skipped")

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationConfig_transition(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.id", rName),
					resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", "logs/"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.0.days", "30"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.0.storage_class", "STANDARD_IA"),
				),
			},
			{
				ResourceName: resourceName,
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[resourceName]
					if !ok {
						return "", fmt.Errorf("Not found: %s", resourceName)
					}

					return rs.Primary.Attributes[names.AttrBucket], nil
				},
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: names.AttrBucket,
			},
		},
	})
}

// TC-S3-LC-03: verifies that a rule with both expiration and transition configured can be created and read back correctly.
func TestAccS3BucketLifecycleConfiguration_TransitionVariants_expirationAndTransition(t *testing.T) {
	t.Skip("temporarily skipped")

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationConfig_expirationAndTransition(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.id", rName),
					resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", "archive/"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.0.days", "365"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.0.days", "30"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.0.storage_class", "STANDARD_IA"),
				),
			},
		},
	})
}

// TC-S3-LC-03: verifies that an illegal storage_class value is rejected by schema validation.
func TestAccS3BucketLifecycleConfiguration_TransitionVariants_invalidStorageClass(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationConfig_transitionStorageClass(rName, "GLACIER"),
				ExpectError: regexache.MustCompile(`(?i)value must be one of: \["STANDARD_IA"\], got:`),
			},
		},
	})
}

// TC-S3-LC-04: verifies that the provider auto-generates a stable prefix when no ID is explicitly set.
func TestAccS3BucketLifecycleConfiguration_RuleIDAndPrefixBoundaries_generatedID(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationConfig_ruleBoundary(rName, "", "prefix/", false, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestMatchResourceAttr(resourceName, "rule.0.id", regexache.MustCompile("^tf-s3-lifecycle-")),
					resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", "prefix/"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.0.days", "30"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.#", "0"),
				),
			},
		},
	})
}

// TC-S3-LC-04: verifies that a rule ID of the maximum length 255 can be created and read back correctly.
func TestAccS3BucketLifecycleConfiguration_RuleIDAndPrefixBoundaries_maxID(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"
	id := strings.Repeat("i", 255)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationConfig_ruleBoundary(rName, id, "prefix/", true, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.id", id),
					resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", "prefix/"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.0.days", "30"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.#", "0"),
				),
			},
		},
	})
}

// TC-S3-LC-04: verifies that a rule ID exceeding 255 characters is rejected by validation.
func TestAccS3BucketLifecycleConfiguration_RuleIDAndPrefixBoundaries_overlongID(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationConfig_ruleBoundary(rName, strings.Repeat("i", 256), "prefix/", true, true),
				ExpectError: regexache.MustCompile("(?i)must be between 1 and 255"),
			},
		},
	})
}

// TC-S3-LC-04: verifies that validation fails when the required prefix is not set.
func TestAccS3BucketLifecycleConfiguration_RuleIDAndPrefixBoundaries_missingPrefix(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationConfig_ruleBoundary(rName, "id", "", true, false),
				ExpectError: regexache.MustCompile(`The argument "prefix" is required`),
			},
		},
	})
}

// TC-S3-LC-04: verifies that a prefix of the maximum length 1024 can be created and read back correctly.
func TestAccS3BucketLifecycleConfiguration_RuleIDAndPrefixBoundaries_maxPrefix(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"
	prefix := strings.Repeat("p", 1024)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationConfig_ruleBoundary(rName, "id", prefix, true, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.id", "id"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", prefix),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.0.days", "30"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.#", "0"),
				),
			},
		},
	})
}

// TC-S3-LC-04: verifies that a prefix exceeding 1024 characters is rejected by validation.
func TestAccS3BucketLifecycleConfiguration_RuleIDAndPrefixBoundaries_overlongPrefix(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationConfig_ruleBoundary(rName, "id", strings.Repeat("p", 1025), true, true),
				ExpectError: regexache.MustCompile("(?i)must be between 0 and 1024"),
			},
		},
	})
}

// TC-S3-LC-04: verifies that the valid rule status value Disabled can be created and read back correctly.
func TestAccS3BucketLifecycleConfiguration_RuleIDAndPrefixBoundaries_validStatus(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationConfig_status(rName, "Disabled"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.id", rName),
					resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Disabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", "status/"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.0.days", "30"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.#", "0"),
				),
			},
		},
	})
}

// TC-S3-LC-04: verifies that an illegal rule status value is rejected by schema validation.
func TestAccS3BucketLifecycleConfiguration_RuleIDAndPrefixBoundaries_invalidStatus(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationConfig_status(rName, "Paused"),
				ExpectError: regexache.MustCompile(`value must be one of: \["Enabled" "Disabled"\], got`),
			},
		},
	})
}

// TC-S3-LC-05: verifies that only the target rule's expiration days is updated among multiple rules.
func TestAccS3BucketLifecycleConfiguration_MultipleRulesUpdate_days(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationConfig_multipleExpirationRules(rName, 30),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.id", "days-target"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", "days/"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.0.days", "30"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.id", "days-stable"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.status", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.prefix", "stable/"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.expiration.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.expiration.0.days", "365"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.transition.#", "0"),
				),
			},
			{
				Config: testAccBucketLifecycleConfigurationConfig_multipleExpirationRules(rName, 60),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "rule.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.id", "days-target"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", "days/"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.0.days", "60"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.id", "days-stable"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.status", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.prefix", "stable/"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.expiration.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.expiration.0.days", "365"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.transition.#", "0"),
				),
			},
		},
	})
}

// TC-S3-LC-05: verifies that only the target transition's days is updated among multiple rules.
func TestAccS3BucketLifecycleConfiguration_MultipleRulesUpdate_transition(t *testing.T) {
	t.Skip("temporarily skipped")

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationConfig_multipleTransitionRules(rName, 30),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.id", "expiration-stable"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", "expiration/"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.0.days", "365"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.id", "transition-target"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.status", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.prefix", "transition/"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.expiration.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.transition.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.transition.0.days", "30"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.transition.0.storage_class", "STANDARD_IA"),
				),
			},
			{
				Config: testAccBucketLifecycleConfigurationConfig_multipleTransitionRules(rName, 60),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "rule.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.id", "expiration-stable"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", "expiration/"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.0.days", "365"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.id", "transition-target"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.status", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.prefix", "transition/"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.expiration.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.transition.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.transition.0.days", "60"),
					resource.TestCheckResourceAttr(resourceName, "rule.1.transition.0.storage_class", "STANDARD_IA"),
				),
			},
		},
	})
}

// TC-S3-LC-05: verifies that creation fails when multiple rules use the same ID.
func TestAccS3BucketLifecycleConfiguration_MultipleRulesUpdate_duplicateID(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationConfig_duplicateRuleID(rName),
				ExpectError: regexache.MustCompile("(?i)(InvalidRequest: Rule ID must be unique.)"),
			},
		},
	})
}

// TC-S3-LC-05: verifies that creation fails when multiple rules use conflicting prefixes.
func TestAccS3BucketLifecycleConfiguration_MultipleRulesUpdate_conflictPrefix(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationConfig_conflictRulePrefix(rName),
				ExpectError: regexache.MustCompile("Found two rules with same prefix"),
			},
		},
	})
}

// TC-S3-LC-05: verifies that creation succeeds when the rule count reaches the OOS limit of 1000.
func TestAccS3BucketLifecycleConfiguration_MultipleRulesUpdate_maxRules(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationConfig_manyRules(rName, 1000),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1000"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.id", "rule-0"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", "rule-0/"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.0.days", "30"),
					resource.TestCheckResourceAttr(resourceName, "rule.999.id", "rule-999"),
					resource.TestCheckResourceAttr(resourceName, "rule.999.status", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.999.prefix", "rule-999/"),
					resource.TestCheckResourceAttr(resourceName, "rule.999.expiration.0.days", "30"),
				),
			},
		},
	})
}

// TC-S3-LC-05: verifies that validation fails when the rule count exceeds the OOS limit.
func TestAccS3BucketLifecycleConfiguration_MultipleRulesUpdate_overMaxRules(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationConfig_manyRules(rName, 1001),
				ExpectError: regexache.MustCompile("(?i)MalformedXML"),
			},
		},
	})
}

// TC-S3-LC-11: verifies that validation fails when expiration sets both date and days.
func TestAccS3BucketLifecycleConfiguration_DateDaysConflict_expiration(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationConfig_expirationDateDaysConflict(rName),
				ExpectError: regexache.MustCompile("(?is)(2 attributes specified|one \\(and only one\\)).*(date|days)"),
			},
		},
	})
}

// TC-S3-LC-11: verifies that validation fails when transition sets both date and days.
func TestAccS3BucketLifecycleConfiguration_DateDaysConflict_transition(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationConfig_transitionDateDaysConflict(rName),
				ExpectError: regexache.MustCompile("(?is)exactly one of transition date or\\s+days\\s+is required"),
			},
		},
	})
}

// TC-S3-LC-02: verifies that a standard RFC3339 UTC expiration date is created successfully and matches state.
func TestAccS3BucketLifecycleConfiguration_ExpirationByDate_standard(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"
	dateValue := "2030-01-02T00:00:00.000Z"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationConfig_expirationDate(rName, dateValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.id", rName),
					resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", "date/"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.0.date", dateValue),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.#", "0"),
				),
			},
			{
				ResourceName: resourceName,
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[resourceName]
					if !ok {
						return "", fmt.Errorf("Not found: %s", resourceName)
					}

					return rs.Primary.Attributes[names.AttrBucket], nil
				},
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: names.AttrBucket,
			},
		},
	})
}

// TC-S3-LC-02: verifies that an illegal date format is rejected by schema validation.
func TestAccS3BucketLifecycleConfiguration_ExpirationByDate_invalidFormat(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationConfig_expirationDate(rName, "2030-01-02"),
				ExpectError: regexache.MustCompile("(?i)not valid RFC3339 string format"),
			},
		},
	})
}

// TC-S3-LC-02: verifies that a non-UTC timezone format is rejected by schema validation.
func TestAccS3BucketLifecycleConfiguration_ExpirationByDate_invalidTimezone(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationConfig_expirationDate(rName, "2030-01-02T00:00:00.000+08:00"),
				ExpectError: regexache.MustCompile("(?is)must be a UTC midnight timestamp in the\\s+exact form"),
			},
		},
	})
}

// TC-S3-LC-02: verifies that a transition date in standard RFC3339 UTC format is created and read back in full.
func TestAccS3BucketLifecycleConfiguration_TransitionByDate_standard(t *testing.T) {
	t.Skip("temporarily skipped")

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"
	dateValue := "2030-01-02T00:00:00.000Z"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationConfig_transitionDate(rName, dateValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.id", rName),
					resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", "transition-date/"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.0.date", dateValue),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.0.storage_class", "STANDARD_IA"),
				),
			},
		},
	})
}

// TC-S3-LC-02: verifies that a transition date with an illegal date format is rejected by the schema.
func TestAccS3BucketLifecycleConfiguration_TransitionByDate_invalidFormat(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationConfig_transitionDate(rName, "2030-01-02"),
				ExpectError: regexache.MustCompile("(?i)not valid RFC3339 string format"),
			},
		},
	})
}

// TC-S3-LC-02: verifies that a transition date with a non-UTC timezone format is rejected by validation.
func TestAccS3BucketLifecycleConfiguration_TransitionByDate_invalidTimezone(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketLifecycleConfigurationConfig_transitionDate(rName, "2030-01-02T00:00:00.000+08:00"),
				ExpectError: regexache.MustCompile("(?is)must be a UTC midnight timestamp in the\\s+exact form"),
			},
		},
	})
}

// Verifies that the provider can generate and persist a computed ID when no rule ID is configured.
// TC-S3-LC-09: verifies auto-generated rule IDs, using the bucket as the import identifier.
func TestAccS3BucketLifecycleConfiguration_AutoIDAndImport(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationConfig_computedID(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestMatchResourceAttr(resourceName, "rule.0.id", regexache.MustCompile(`^tf-s3-lifecycle-`)),
					resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", "generated/"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.0.days", "30"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.#", "0"),
				),
			},
			{
				ResourceName: resourceName,
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[resourceName]
					if !ok {
						return "", fmt.Errorf("Not found: %s", resourceName)
					}

					return rs.Primary.Attributes[names.AttrBucket], nil
				},
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: names.AttrBucket,
			},
		},
	})
}

// TC-S3-LC-10: verifies that a clear error is returned when a rule has neither expiration nor transition.
func TestAccS3BucketLifecycleConfiguration_MissingActionsNegative(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:             testAccBucketLifecycleConfigurationConfig_missingAction(rName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config:      testAccBucketLifecycleConfigurationConfig_missingAction(rName),
				ExpectError: regexache.MustCompile(`(?is)Invalid Lifecycle Rule.*at least one of expiration or\s+transition is required`),
			},
		},
	})
}

// TC-S3-LC-14: single-rule expiration, transition, and status updates.
func TestAccS3BucketLifecycleConfiguration_SingleRuleUpdate(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_lifecycle_configuration.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketLifecycleConfigurationDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketLifecycleConfigurationConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.id", rName),
					resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Enabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", "prefix/"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.0.days", "365"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.#", "0"),
				),
			},
			{
				Config: testAccBucketLifecycleConfigurationConfig_expirationDays(rName, 90),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.id", rName),
					resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Disabled"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", "prefix/"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.0.days", "90"),
					resource.TestCheckResourceAttr(resourceName, "rule.0.transition.#", "0"),
				),
			},
			// {
			// 	Config: testAccBucketLifecycleConfigurationConfig_transitionOnly(rName),
			// 	Check: resource.ComposeAggregateTestCheckFunc(
			// 		testAccCheckBucketLifecycleConfigurationExists(ctx, t, resourceName),
			// 		resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
			// 		resource.TestCheckResourceAttr(resourceName, "rule.#", "1"),
			// 		resource.TestCheckResourceAttr(resourceName, "rule.0.id", rName),
			// 		resource.TestCheckResourceAttr(resourceName, "rule.0.status", "Enabled"),
			// 		resource.TestCheckResourceAttr(resourceName, "rule.0.prefix", "archive/"),
			// 		resource.TestCheckResourceAttr(resourceName, "rule.0.expiration.#", "0"),
			// 		resource.TestCheckResourceAttr(resourceName, "rule.0.transition.#", "1"),
			// 		resource.TestCheckResourceAttr(resourceName, "rule.0.transition.0.days", "30"),
			// 		resource.TestCheckResourceAttr(resourceName, "rule.0.transition.0.storage_class", "STANDARD_IA"),
			// 	),
			// },
			{
				ResourceName: resourceName,
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[resourceName]
					if !ok {
						return "", fmt.Errorf("Not found: %s", resourceName)
					}

					return rs.Primary.Attributes[names.AttrBucket], nil
				},
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: names.AttrBucket,
			},
		},
	})
}

func testAccCheckBucketLifecycleConfigurationExists(ctx context.Context, t *testing.T, n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		_, err := tfs3.FindBucketLifecycleConfiguration(ctx, conn, rs.Primary.Attributes[names.AttrBucket])

		return err
	}
}

func testAccCheckBucketLifecycleConfigurationNotFound(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		bucketResource, ok := s.RootModule().Resources["ctyun_oos_bucket.test"]
		if !ok {
			return fmt.Errorf("Not found: ctyun_oos_bucket.test")
		}

		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)
		_, err := tfs3.FindBucketLifecycleConfiguration(ctx, conn, bucketResource.Primary.Attributes[names.AttrBucket])
		if retry.NotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}

		return fmt.Errorf("OOS Bucket Lifecycle Configuration (%s) still exists", bucketResource.Primary.Attributes[names.AttrBucket])
	}
}
func testAccCheckBucketLifecycleConfigurationDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_oos_bucket_lifecycle_configuration" {
				continue
			}

			_, err := tfs3.FindBucketLifecycleConfiguration(ctx, conn, rs.Primary.Attributes[names.AttrBucket])

			if retry.NotFound(err) {
				continue
			}

			if err != nil {
				return err
			}

			return fmt.Errorf("OOS Bucket Lifecycle Configuration (%s) still exists", rs.Primary.ID)
		}

		return nil
	}
}

func testAccBucketLifecycleConfigurationConfig_lifecycleOnly(bucketName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = %[1]q

  rule {
    id     = "validation"
    prefix = "validation/"
    status = "Enabled"

    expiration {
      days = 30
    }
  }
}
`, bucketName)
}
func testAccBucketLifecycleConfigurationConfig_bucketOnly(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}
`, rName)
}
func testAccBucketLifecycleConfigurationConfig_basic(rName string) string {
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
`, rName)
}

func testAccBucketLifecycleConfigurationConfig_expirationDays(rName string, days int) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    id     = %[1]q
    prefix = "prefix/"
    status = "Disabled"

    expiration {
      days = %[2]d
    }
  }
}
`, rName, days)
}

func testAccBucketLifecycleConfigurationConfig_expirationDateDaysConflict(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    id     = %[1]q
    prefix = "conflict/"
    status = "Enabled"

    expiration {
      date = "2030-01-02T00:00:00.000Z"
      days = 30
    }
  }
}
`, rName)
}

func testAccBucketLifecycleConfigurationConfig_transitionDateDaysConflict(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    id     = %[1]q
    prefix = "conflict/"
    status = "Enabled"

    transition {
      date          = "2030-01-02T00:00:00.000Z"
      days          = 30
      storage_class = "STANDARD_IA"
    }
  }
}
`, rName)
}
func testAccBucketLifecycleConfigurationConfig_ruleBoundary(rName, id, prefix string, includeID, includePrefix bool) string {
	idConfig := ""
	if includeID {
		idConfig = fmt.Sprintf("    id     = %q\n", id)
	}
	prefixConfig := ""
	if includePrefix {
		prefixConfig = fmt.Sprintf("    prefix = %q\n", prefix)
	}

	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
%[2]s
%[3]s
    status = "Enabled"

    expiration {
      days = 30
    }
  }
}
`, rName, idConfig, prefixConfig)
}
func testAccBucketLifecycleConfigurationConfig_multipleExpirationRules(rName string, days int) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    id     = "days-target"
    prefix = "days/"
    status = "Enabled"
    expiration {
      days = %[2]d
    }
  }

  rule {
    id     = "days-stable"
    prefix = "stable/"
    status = "Enabled"
    expiration {
      days = 365
    }
  }
}
`, rName, days)
}

func testAccBucketLifecycleConfigurationConfig_multipleTransitionRules(rName string, days int) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    id     = "expiration-stable"
    prefix = "expiration/"
    status = "Enabled"
    expiration { days = 365 }
  }

  rule {
    id     = "transition-target"
    prefix = "transition/"
    status = "Enabled"
    transition {
      days          = %[2]d
      storage_class = "STANDARD_IA"
    }
  }
}
`, rName, days)
}

func testAccBucketLifecycleConfigurationConfig_duplicateRuleID(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    id     = "duplicate"
    prefix = "first/"
    status = "Enabled"
    expiration { days = 30 }
  }
  rule {
    id     = "duplicate"
    prefix = "second/"
    status = "Enabled"
    expiration { days = 60 }
  }
}
`, rName)
}

func testAccBucketLifecycleConfigurationConfig_conflictRulePrefix(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    id     = "prefix-first"
    prefix = "conflict/"
    status = "Enabled"
    expiration { days = 30 }
  }
  rule {
    id     = "prefix-second"
    prefix = "conflict/"
    status = "Enabled"
    expiration { days = 60 }
  }
}
`, rName)
}

func testAccBucketLifecycleConfigurationConfig_manyRules(rName string, count int) string {
	var rules strings.Builder
	for i := 0; i < count; i++ {
		fmt.Fprintf(&rules, `

  rule {
    id     = "rule-%d"
    prefix = "rule-%d/"
    status = "Enabled"
    expiration {
      days = 30
    }
  }
`, i, i)
	}

	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket
%[2]s
}
`, rName, rules.String())
}
func testAccBucketLifecycleConfigurationConfig_status(rName, status string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    id     = %[1]q
    prefix = "status/"
    status = %[2]q

    expiration {
      days = 30
    }
  }
}
`, rName, status)
}

func testAccBucketLifecycleConfigurationConfig_transitionDate(rName, dateValue string) string {
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
`, rName, dateValue)
}

func testAccBucketLifecycleConfigurationConfig_expirationDate(rName, dateValue string) string {
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
`, rName, dateValue)
}

func testAccBucketLifecycleConfigurationConfig_transitionDays(rName string, days int) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    id     = %[1]q
    prefix = "transition/"
    status = "Enabled"

    transition {
      days          = %[2]d
      storage_class = "STANDARD_IA"
    }
  }
}
`, rName, days)
}

func testAccBucketLifecycleConfigurationConfig_transition(rName string) string {
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
`, rName)
}

func testAccBucketLifecycleConfigurationConfig_expirationAndTransition(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    id     = %[1]q
    prefix = "archive/"
    status = "Enabled"

    expiration {
      days = 365
    }

    transition {
      days          = 30
      storage_class = "STANDARD_IA"
    }
  }
}
`, rName)
}

func testAccBucketLifecycleConfigurationConfig_transitionStorageClass(rName, storageClass string) string {
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
      storage_class = %[2]q
    }
  }
}
`, rName, storageClass)
}

func testAccBucketLifecycleConfigurationConfig_computedID(rName string) string {
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
`, rName)
}

func testAccBucketLifecycleConfigurationConfig_missingAction(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_lifecycle_configuration" "test" {
  bucket = ctyun_oos_bucket.test.bucket

  rule {
    id     = %[1]q
    prefix = "invalid/"
    status = "Enabled"
  }
}
`, rName)
}
