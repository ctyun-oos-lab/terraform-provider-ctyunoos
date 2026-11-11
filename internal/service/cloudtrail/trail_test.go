// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package cloudtrail_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/provider/sdkv2"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	tfcloudtrail "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/cloudtrail"
	tfs3 "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/s3"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	sdkretry "github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const (
	cloudTrailResourcePropagationTimeout = 2 * time.Minute
	cloudTrailLimitFixtureMax            = 100
)

// TC-CT-TRAIL-01 Basic CRUD: creates a Trail with a prefix, verifies the remote service and state, then deletes it with Terraform.
func TestAccCloudTrailTrail_basic(t *testing.T) {
	ctx := acctest.Context(t)
	var trail types.Trail
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_cloudtrail.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccCloudTrailConfig_basic(rName, "prefix"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailAttributes(&trail, rName, rName, "prefix"),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
					resource.TestCheckResourceAttr(resourceName, "enable_logging", "true"),
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

// TC-CT-TRAIL-01 Basic CRUD: creates a Trail without a prefix, verifies the remote service and state, then deletes it with Terraform.
func TestAccCloudTrailTrail_noPrefix(t *testing.T) {
	ctx := acctest.Context(t)
	var trail types.Trail
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_cloudtrail.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccCloudTrailConfig_basic(rName, ""),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailAttributes(&trail, rName, rName, ""),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
			},
		},
	})
}

// TC-CT-TRAIL-01 Basic CRUD: creates a Trail with an explicitly empty string prefix and verifies remote/state readback.
func TestAccCloudTrailTrail_emptyPrefix(t *testing.T) {
	ctx := acctest.Context(t)
	var trail types.Trail
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_cloudtrail.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccCloudTrailConfig_withExplicitPrefix(rName, rName, ""),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailAttributes(&trail, rName, rName, ""),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
			},
		},
	})
}

// TC-CT-TRAIL-01 Basic CRUD: creates a Trail with a slash-containing prefix and verifies remote/state readback.
func TestAccCloudTrailTrail_slashPrefix(t *testing.T) {
	ctx := acctest.Context(t)
	var trail types.Trail
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_cloudtrail.test"
	const prefix = "audit/logs/"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccCloudTrailConfig_basic(rName, prefix),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailAttributes(&trail, rName, rName, prefix),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
			},
		},
	})
}

// TC-CT-TRAIL-01 Basic CRUD: the service must reject creating a Trail with a duplicate name.
func TestAccCloudTrailTrail_duplicateName(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccCloudTrailConfig_duplicateName(rName, false),
				Check:  testAccCheckTrailExists(ctx, t, "ctyun_cloudtrail.first"),
			},
			{
				Config:      testAccCloudTrailConfig_duplicateName(rName, true),
				ExpectError: regexache.MustCompile(`(?i)TrailAlreadyExistsException`),
			},
		},
	})
}

// TC-CT-TRAIL-01 Basic CRUD: Terraform must fail during planning when the required bucket argument is absent.
func TestAccCloudTrailTrail_missingBucket(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccCloudTrailConfig_missingBucket(rName),
				ExpectError: regexache.MustCompile(`(?i)s3_bucket_name.*required`),
			},
		},
	})
}

// TC-CT-TRAIL-01 Basic CRUD: creating one Trail beyond the configured limit must trigger the service-side quota.
func TestAccCloudTrailTrail_limitExceeded(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccCTYUNCloudTrailConfig_bucket(rName),
			},
			{
				PreConfig: func() {
					testAccFillTrailLimitFixture(ctx, t, rName)
				},
				Config:      testAccCloudTrailConfig_limitExceeded(rName),
				ExpectError: regexache.MustCompile(`(?i)MaximumNumberOfTrailsExceededException`),
			},
		},
	})
}

// TC-CT-TRAIL-02 Name Validation Matrix: verifies provider-schema name-length boundaries and that valid names reach service-side validation.
func TestUnitCloudTrailTrail_NameValidation(t *testing.T) {
	t.Parallel()

	provider, err := sdkv2.NewProvider(context.Background())
	if err != nil {
		t.Fatalf("creating SDKv2 provider: %s", err)
	}

	validate := provider.ResourcesMap["ctyun_cloudtrail"].Schema[names.AttrName].ValidateFunc
	if validate == nil {
		t.Fatal("expected name validation")
	}

	for _, tc := range []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "minimum_length", value: "abc", wantErr: false},
		{name: "maximum_length", value: strings.Repeat("a", 128), wantErr: false},
		{name: "too_short", value: "ab", wantErr: true},
		{name: "too_long", value: strings.Repeat("a", 129), wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := validate(tc.value, names.AttrName)
			if gotErr := len(errs) > 0; gotErr != tc.wantErr {
				t.Fatalf("ValidateFunc(%q) error = %t, want %t: %v", tc.value, gotErr, tc.wantErr, errs)
			}
		})
	}
}

// TC-CT-TRAIL-03 Prefix And Bucket Validation: verifies prefix schema-length boundaries, allowing 200 characters through to the service and rejecting 201 locally.
func TestUnitCloudTrailTrail_PrefixValidation(t *testing.T) {
	t.Parallel()

	provider, err := sdkv2.NewProvider(context.Background())
	if err != nil {
		t.Fatalf("creating SDKv2 provider: %s", err)
	}

	validate := provider.ResourcesMap["ctyun_cloudtrail"].Schema[names.AttrS3KeyPrefix].ValidateFunc
	if validate == nil {
		t.Fatal("expected s3_key_prefix validation")
	}

	for _, tc := range []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "empty", value: "", wantErr: false},
		{name: "maximum_length", value: strings.Repeat("a", 200), wantErr: false},
		{name: "too_long", value: strings.Repeat("a", 201), wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := validate(tc.value, names.AttrS3KeyPrefix)
			if gotErr := len(errs) > 0; gotErr != tc.wantErr {
				t.Fatalf("ValidateFunc(%q) error = %t, want %t: %v", tc.value, gotErr, tc.wantErr, errs)
			}
		})
	}
}

// TC-CT-TRAIL-06 Event Selectors: verifies selector count and read_write_type enum validation in the registered nested schema.
func TestUnitCloudTrailTrail_EventSelectorValidation(t *testing.T) {
	t.Parallel()

	provider, err := sdkv2.NewProvider(context.Background())
	if err != nil {
		t.Fatalf("creating SDKv2 provider: %s", err)
	}

	eventSelectorSchema := provider.ResourcesMap["ctyun_cloudtrail"].Schema["event_selector"]
	if eventSelectorSchema == nil || !eventSelectorSchema.Optional || !eventSelectorSchema.Computed || eventSelectorSchema.MaxItems != 1 {
		t.Fatalf("event_selector schema = %#v, want optional+computed with MaxItems 1", eventSelectorSchema)
	}
	eventSelectorResource, ok := eventSelectorSchema.Elem.(*schema.Resource)
	if !ok {
		t.Fatalf("event_selector Elem = %T, want *schema.Resource", eventSelectorSchema.Elem)
	}
	readWriteTypeSchema := eventSelectorResource.Schema["read_write_type"]
	if readWriteTypeSchema == nil || !readWriteTypeSchema.Optional || !readWriteTypeSchema.Computed || readWriteTypeSchema.ValidateDiagFunc == nil {
		t.Fatalf("read_write_type schema = %#v, want optional+computed with enum validation", readWriteTypeSchema)
	}

	for _, tc := range []struct {
		value   string
		wantErr bool
	}{
		{value: string(types.ReadWriteTypeAll)},
		{value: string(types.ReadWriteTypeReadOnly)},
		{value: string(types.ReadWriteTypeWriteOnly)},
		{value: "invalid", wantErr: true},
	} {
		diags := readWriteTypeSchema.ValidateDiagFunc(tc.value, nil)
		if gotErr := diags.HasError(); gotErr != tc.wantErr {
			t.Fatalf("read_write_type ValidateDiagFunc(%q) error = %t, want %t: %v", tc.value, gotErr, tc.wantErr, diags)
		}
	}
}

// TC-CT-TRAIL-02 Name Validation Matrix: creates valid names containing dots, hyphens, and 128 characters, then verifies the service and state.
func TestAccCloudTrailTrail_validNameValidation(t *testing.T) {
	ctx := acctest.Context(t)
	var trail types.Trail
	bucketName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_cloudtrail.test"

	validNames := []string{
		"trail-basic-01",
		"trail.name-01",
		"prod.audit-trail",
		"90D",
		testAccCloudTrailTrailNameWithLength(bucketName, 128),
	}
	steps := make([]resource.TestStep, 0, len(validNames))
	for i, trailName := range validNames {
		step := resource.TestStep{
			Config: testAccCloudTrailConfig_withName(bucketName, trailName, ""),
			Check: resource.ComposeTestCheckFunc(
				testAccCheckTrailExists(ctx, t, resourceName, &trail),
				testAccCheckTrailAttributes(&trail, trailName, bucketName, ""),
				testAccCheckTrailState(ctx, t, resourceName, &trail),
			),
		}
		if i > 0 {
			previousTrailName := validNames[i-1]
			step.Check = resource.ComposeTestCheckFunc(
				step.Check,
				testAccCheckTrailNotExists(ctx, t, previousTrailName),
			)
			step.ConfigPlanChecks = resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionReplace),
				},
				PostApplyPostRefresh: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			}
		}
		steps = append(steps, step)
	}

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDestroy(ctx, t),
		Steps:                    steps,
	})
}

// TC-CT-TRAIL-02 Name Validation Matrix: verifies service-side naming rules for names that pass length validation.
func TestAccCloudTrailTrail_invalidNameValidation(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	invalidNames := []string{
		"59.44.20.98",
		".44.20.98",
		"-4ccc8",
		"sq8dj_",
		"trail name",
		"123$@#^&*%sawq",
		"123..eq",
	}
	steps := make([]resource.TestStep, 0, len(invalidNames))
	for _, trailName := range invalidNames {
		steps = append(steps, resource.TestStep{
			Config:      testAccCloudTrailConfig_withName(bucketName, trailName, ""),
			ExpectError: regexache.MustCompile(`(?i)InvalidTrailNameException`),
		})
	}

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDestroy(ctx, t),
		Steps:                    steps,
	})
}

// TC-CT-TRAIL-02 Name Validation Matrix: names below the minimum length, above the maximum length, or blank fail at the provider-schema stage.
func TestAccCloudTrailTrail_invalidNameLengthValidation(t *testing.T) {
	ctx := acctest.Context(t)

	for _, tc := range []struct {
		name      string
		trailName string
	}{
		{name: "two_characters", trailName: "ab"},
		{name: "too_long", trailName: strings.Repeat("a", 129)},
		{name: "blank", trailName: " "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bucketName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
			acctest.ParallelTest(ctx, t, resource.TestCase{
				PreCheck:                 func() { acctest.PreCheck(ctx, t) },
				ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
				ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      testAccCloudTrailConfig_withName(bucketName, tc.trailName, ""),
						ExpectError: regexache.MustCompile(`(?i)expected length of name.*range`),
					},
				},
			})
		})
	}
}

// TC-CT-TRAIL-03 Prefix And Bucket Validation: covers null, empty-string, slash, special-character, whitespace, and 200-character prefixes.
func TestAccCloudTrailTrail_prefixValidation(t *testing.T) {
	ctx := acctest.Context(t)
	var trail types.Trail
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_cloudtrail.test"
	prefixes := []struct {
		name     string
		value    string
		explicit bool
	}{
		{name: "null"},
		{name: "empty", value: "", explicit: true},
		{name: "single_slash", value: "/", explicit: true},
		{name: "multiple_slashes", value: "audit///logs", explicit: true},
		{name: "special_characters", value: "!@#$%^&*(()~_++{}<>|?/''[]", explicit: true},
		{name: "spaces", value: "audit logs", explicit: true},
		{name: "maximum_length", value: strings.Repeat("a", 200), explicit: true},
	}
	steps := make([]resource.TestStep, 0, len(prefixes))
	for _, tc := range prefixes {
		config := testAccCloudTrailConfig_basic(rName, "")
		if tc.explicit {
			config = testAccCloudTrailConfig_withExplicitPrefix(rName, rName, tc.value)
		}

		steps = append(steps, resource.TestStep{
			Config: config,
			Check: resource.ComposeTestCheckFunc(
				testAccCheckTrailExists(ctx, t, resourceName, &trail),
				testAccCheckTrailAttributes(&trail, rName, rName, tc.value),
				testAccCheckTrailState(ctx, t, resourceName, &trail),
			),
		})
	}

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDestroy(ctx, t),
		Steps:                    steps,
	})
}

// TC-CT-TRAIL-03 Prefix And Bucket Validation: the service separately rejects empty and nonexistent buckets with explicit error codes.
func TestAccCloudTrailTrail_invalidBucket(t *testing.T) {
	ctx := acctest.Context(t)
	emptyBucketTrailName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	missingBucketTrailName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	missingBucketName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	leadingSpaceBucketTrailName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccCloudTrailConfig_withBucket(emptyBucketTrailName, "", ""),
				ExpectError: regexache.MustCompile(`(?i)S3BucketDoesNotExistException`),
			},
			{
				Config:      testAccCloudTrailConfig_withBucket(missingBucketTrailName, missingBucketName, ""),
				ExpectError: regexache.MustCompile(`(?i)S3BucketDoesNotExistException`),
			},
			{
				Config:      testAccCloudTrailConfig_withBucket(leadingSpaceBucketTrailName, " "+missingBucketName, ""),
				ExpectError: regexache.MustCompile(`(?i)S3BucketDoesNotExistException`),
			},
		},
	})
}

// TC-CT-TRAIL-03 Prefix And Bucket Validation: a bucket belonging to another account must return AccessDenied.
func TestAccCloudTrailTrail_bucketAccessDenied(t *testing.T) {
	secondBucketName := os.Getenv("CTYUNOOS_CLOUDTRAIL_S3_BUCKET_SECOND")
	if secondBucketName == "" {
		t.Skip("skipping CloudTrail other-account bucket test: CTYUNOOS_CLOUDTRAIL_S3_BUCKET_SECOND must be configured")
	}

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccCloudTrailConfig_withBucket(rName, secondBucketName, ""),
				ExpectError: regexache.MustCompile(`(?i)AccessDenied.*Can not access bucket`),
			},
		},
	})
}

// TC-CT-TRAIL-04 Basic Update: updates the bucket, prefix, and their combination independently, verifying that the remote object and state agree.
func TestAccCloudTrailTrail_basicUpdate(t *testing.T) {
	ctx := acctest.Context(t)
	var trail types.Trail
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	initialBucketName := rName + "-initial"
	updatedBucketName := rName + "-updated"
	resourceName := "ctyun_cloudtrail.test"
	updates := []struct {
		name             string
		prefix           string
		omitPrefix       bool
		useUpdatedBucket bool
	}{
		{name: "initial", omitPrefix: true},
		{name: "prefix_only", prefix: "initial-prefix"},
		{name: "bucket_only", prefix: "initial-prefix", useUpdatedBucket: true},
		{name: "remove_prefix", prefix: "", useUpdatedBucket: true},
		{name: "prefix_and_bucket", prefix: "updated-prefix", useUpdatedBucket: true},
		{name: "bucket_and_prefix", prefix: "combined-prefix"},
		{name: "maximum_prefix", prefix: strings.Repeat("a", 200)},
		{name: "spaces_prefix", prefix: "prefix with spaces"},
	}
	steps := make([]resource.TestStep, 0, len(updates))
	for i, tc := range updates {
		bucketName := initialBucketName
		if tc.useUpdatedBucket {
			bucketName = updatedBucketName
		}

		config := testAccCloudTrailConfig_basicUpdate(rName, initialBucketName, updatedBucketName, tc.prefix, tc.useUpdatedBucket)
		if tc.omitPrefix {
			config = testAccCloudTrailConfig_basicUpdateWithoutPrefix(rName, initialBucketName, updatedBucketName, tc.useUpdatedBucket)
		}

		step := resource.TestStep{
			Config: config,
			Check: resource.ComposeTestCheckFunc(
				testAccCheckTrailExists(ctx, t, resourceName, &trail),
				testAccCheckTrailAttributes(&trail, rName, bucketName, tc.prefix),
				testAccCheckTrailState(ctx, t, resourceName, &trail),
			),
		}
		if i > 0 {
			step.ConfigPlanChecks = resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
				},
				PostApplyPostRefresh: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				},
			}
		}
		steps = append(steps, step)
	}

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDestroy(ctx, t),
		Steps:                    steps,
	})
}

// TC-CT-TRAIL-04 Basic Update: after external changes to the bucket and prefix, Terraform should reconcile drift in the basic fields.
func TestAccCloudTrailTrail_basicFieldDriftRecovery(t *testing.T) {
	ctx := acctest.Context(t)
	var trail types.Trail
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	initialBucketName := rName + "-initial"
	updatedBucketName := rName + "-updated"
	resourceName := "ctyun_cloudtrail.test"
	const initialPrefix = "initial-prefix"
	const externalPrefix = "external-prefix"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccCloudTrailConfig_basicUpdate(rName, initialBucketName, updatedBucketName, initialPrefix, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailAttributes(&trail, rName, initialBucketName, initialPrefix),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
			},
			{
				Config:             testAccCloudTrailConfig_basicUpdate(rName, initialBucketName, updatedBucketName, initialPrefix, false),
				Check:              testAccUpdateTrailExternally(ctx, t, resourceName, updatedBucketName, externalPrefix),
				ExpectNonEmptyPlan: true,
			},
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailAttributes(&trail, rName, updatedBucketName, externalPrefix),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
			},
			{
				Config: testAccCloudTrailConfig_basicUpdate(rName, initialBucketName, updatedBucketName, initialPrefix, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailAttributes(&trail, rName, initialBucketName, initialPrefix),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TC-CT-TRAIL-05 Logging Toggle: verifies the default stopped state, Start/Stop idempotency, and the Start -> Stop -> Start round trip.
func TestAccCloudTrailTrail_loggingToggle(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_cloudtrail.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccCTYUNCloudTrailConfig_enableLogging(rName, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "enable_logging", "false"),
					testAccCheckCTYUNLoggingStatus(ctx, t, resourceName, false),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				Config: testAccCTYUNCloudTrailConfig_enableLogging(rName, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "enable_logging", "true"),
					testAccCheckCTYUNLoggingStatus(ctx, t, resourceName, true),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				Config: testAccCTYUNCloudTrailConfig_enableLogging(rName, true),
				Check: resource.ComposeTestCheckFunc(
					testAccStartTrailLoggingExternally(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "enable_logging", "true"),
					testAccCheckCTYUNLoggingStatus(ctx, t, resourceName, true),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				Config: testAccCTYUNCloudTrailConfig_enableLogging(rName, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "enable_logging", "false"),
					testAccCheckCTYUNLoggingStatus(ctx, t, resourceName, false),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				Config: testAccCTYUNCloudTrailConfig_enableLogging(rName, false),
				Check: resource.ComposeTestCheckFunc(
					testAccStopTrailLoggingExternally(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "enable_logging", "false"),
					testAccCheckCTYUNLoggingStatus(ctx, t, resourceName, false),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				Config: testAccCTYUNCloudTrailConfig_enableLogging(rName, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "enable_logging", "true"),
					testAccCheckCTYUNLoggingStatus(ctx, t, resourceName, true),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TC-CT-TRAIL-05 Logging Toggle: after logging is started externally, Terraform should restore the declared stopped state.
func TestAccCloudTrailTrail_loggingDriftRecovery(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_cloudtrail.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccCTYUNCloudTrailConfig_enableLogging(rName, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "enable_logging", "false"),
					testAccCheckCTYUNLoggingStatus(ctx, t, resourceName, false),
				),
			},
			{
				Config: testAccCTYUNCloudTrailConfig_enableLogging(rName, false),
				Check: resource.ComposeTestCheckFunc(
					testAccStartTrailLoggingExternally(ctx, t, resourceName),
					testAccCheckCTYUNLoggingStatus(ctx, t, resourceName, true),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "enable_logging", "true"),
					testAccCheckCTYUNLoggingStatus(ctx, t, resourceName, true),
				),
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
			},
			{
				Config: testAccCTYUNCloudTrailConfig_enableLogging(rName, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "enable_logging", "false"),
					testAccCheckCTYUNLoggingStatus(ctx, t, resourceName, false),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TC-CT-TRAIL-06 Event Selectors: covers the default All value, three valid values, fallback from a service-side invalid value, WriteOnly -> ReadOnly updates, and remote-value readback without a block.
func TestAccCloudTrailTrail_eventSelector(t *testing.T) {
	ctx := acctest.Context(t)
	var trail types.Trail
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_cloudtrail.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccCloudTrailConfig_basic(rName, ""),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailEventSelectors(ctx, t, resourceName, types.ReadWriteTypeAll),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				Config: testAccCloudTrailConfig_eventSelector(rName, types.ReadWriteTypeAll),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailEventSelectors(ctx, t, resourceName, types.ReadWriteTypeAll),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
			},
			{
				Config: testAccCloudTrailConfig_eventSelector(rName, types.ReadWriteTypeReadOnly),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailEventSelectors(ctx, t, resourceName, types.ReadWriteTypeReadOnly),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				Config: testAccCloudTrailConfig_eventSelector(rName, types.ReadWriteTypeWriteOnly),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailEventSelectors(ctx, t, resourceName, types.ReadWriteTypeWriteOnly),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
			},
			{
				Config:             testAccCloudTrailConfig_eventSelector(rName, types.ReadWriteTypeWriteOnly),
				Check:              testAccPutTrailEventSelectorExternally(ctx, t, resourceName, types.ReadWriteType("invalid")),
				ExpectNonEmptyPlan: true,
			},
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailEventSelectors(ctx, t, resourceName, types.ReadWriteTypeAll),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
			},
			{
				Config: testAccCloudTrailConfig_eventSelector(rName, types.ReadWriteTypeWriteOnly),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailEventSelectors(ctx, t, resourceName, types.ReadWriteTypeWriteOnly),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
			},
			{
				Config: testAccCloudTrailConfig_eventSelector(rName, types.ReadWriteTypeReadOnly),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailEventSelectors(ctx, t, resourceName, types.ReadWriteTypeReadOnly),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				Config: testAccCloudTrailConfig_withoutEventSelector(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccPutTrailEventSelectorExternallyExpected(ctx, t, resourceName, types.ReadWriteTypeWriteOnly, types.ReadWriteTypeWriteOnly),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: false,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailEventSelectors(ctx, t, resourceName, types.ReadWriteTypeWriteOnly),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TC-CT-TRAIL-07 Selector Order Stability: repeatedly refreshes and reapplies a single managed event selector without producing a diff.
func TestAccCloudTrailTrail_eventSelectorStability(t *testing.T) {
	ctx := acctest.Context(t)
	var trail types.Trail
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_cloudtrail.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccCloudTrailConfig_eventSelector(rName, types.ReadWriteTypeReadOnly),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailEventSelectors(ctx, t, resourceName, types.ReadWriteTypeReadOnly),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
			},
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: false,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailEventSelectors(ctx, t, resourceName, types.ReadWriteTypeReadOnly),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				Config: testAccCloudTrailConfig_eventSelector(rName, types.ReadWriteTypeReadOnly),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailEventSelectors(ctx, t, resourceName, types.ReadWriteTypeReadOnly),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TC-CT-TRAIL-08 Disappears: after external Trail deletion, verifies that refresh removes state and creates a replacement plan.
func TestAccCloudTrailTrail_disappears(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_cloudtrail.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccCloudTrailConfig_basic(rName, ""),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName),
					testAccCheckTrailDisappearsExternally(ctx, t, resourceName),
				),
				ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
				},
			},
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check:              testAccCheckTrailRemovedFromState(resourceName),
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

// TC-CT-TRAIL-13 UpdateTrail Split: updates only the bucket and verifies that the configured event selector is unchanged.
func TestAccCloudTrailTrail_updateBucketPreservesEventSelector(t *testing.T) {
	ctx := acctest.Context(t)
	var trail types.Trail
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	initialBucketName := rName + "-initial"
	updatedBucketName := rName + "-updated"
	resourceName := "ctyun_cloudtrail.test"
	selector := types.ReadWriteTypeWriteOnly

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccCloudTrailConfig_updateWithEventSelector(rName, initialBucketName, updatedBucketName, "fixed-prefix", false, selector),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailAttributes(&trail, rName, initialBucketName, "fixed-prefix"),
					testAccCheckTrailEventSelectors(ctx, t, resourceName, selector),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
			},
			{
				Config: testAccCloudTrailConfig_updateWithEventSelector(rName, initialBucketName, updatedBucketName, "fixed-prefix", true, selector),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailAttributes(&trail, rName, updatedBucketName, "fixed-prefix"),
					testAccCheckTrailEventSelectors(ctx, t, resourceName, selector),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// Extra coverage: updates the prefix and bucket independently and verifies that the configured event selector is unchanged.
func TestAccCloudTrailTrail_updatePrefixAndBucketPreservesEventSelector(t *testing.T) {
	ctx := acctest.Context(t)
	var trail types.Trail
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	initialBucketName := rName + "-initial"
	updatedBucketName := rName + "-updated"
	resourceName := "ctyun_cloudtrail.test"
	selector := types.ReadWriteTypeWriteOnly

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.CloudTrailServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckTrailDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccCloudTrailConfig_updateWithEventSelector(rName, initialBucketName, updatedBucketName, "initial-prefix", false, selector),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailAttributes(&trail, rName, initialBucketName, "initial-prefix"),
					testAccCheckTrailEventSelectors(ctx, t, resourceName, selector),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
			},
			{
				Config: testAccCloudTrailConfig_updateWithEventSelector(rName, initialBucketName, updatedBucketName, "updated-prefix", false, selector),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailAttributes(&trail, rName, initialBucketName, "updated-prefix"),
					testAccCheckTrailEventSelectors(ctx, t, resourceName, selector),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				Config: testAccCloudTrailConfig_updateWithEventSelector(rName, initialBucketName, updatedBucketName, "updated-prefix", true, selector),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTrailExists(ctx, t, resourceName, &trail),
					testAccCheckTrailAttributes(&trail, rName, updatedBucketName, "updated-prefix"),
					testAccCheckTrailEventSelectors(ctx, t, resourceName, selector),
					testAccCheckTrailState(ctx, t, resourceName, &trail),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccCheckTrailExists(ctx context.Context, t *testing.T, n string, values ...*types.Trail) resource.TestCheckFunc {
	t.Helper()

	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).CloudTrailClient(ctx)

		trail, err := tfcloudtrail.FindTrailByARN(ctx, conn, rs.Primary.ID)
		if err != nil {
			return err
		}
		if len(values) > 0 && values[0] != nil {
			*values[0] = *trail
		}

		return nil
	}
}

func testAccCheckTrailNotExists(ctx context.Context, t *testing.T, trailName string) resource.TestCheckFunc {
	t.Helper()

	return func(_ *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).CloudTrailClient(ctx)

		_, err := tfcloudtrail.FindTrailByName(ctx, conn, trailName)
		if retry.NotFound(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("finding replaced CloudTrail Trail (%s): %w", trailName, err)
		}

		return fmt.Errorf("replaced CloudTrail Trail (%s) still exists", trailName)
	}
}

func testAccCheckTrailAttributes(trail *types.Trail, wantName, wantBucket, wantPrefix string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		if got := aws.ToString(trail.Name); got != wantName {
			return fmt.Errorf("expected Trail name %q, got %q", wantName, got)
		}

		if got := aws.ToString(trail.S3BucketName); got != wantBucket {
			return fmt.Errorf("expected Trail S3 bucket %q, got %q", wantBucket, got)
		}

		if got := aws.ToString(trail.S3KeyPrefix); got != wantPrefix {
			return fmt.Errorf("expected Trail S3 key prefix %q, got %q", wantPrefix, got)
		}

		return nil
	}
}

func testAccCheckTrailState(ctx context.Context, t *testing.T, n string, trail *types.Trail) resource.TestCheckFunc {
	t.Helper()

	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		state := rs.Primary.Attributes
		for key, want := range map[string]string{
			names.AttrARN:          aws.ToString(trail.TrailARN),
			names.AttrName:         aws.ToString(trail.Name),
			names.AttrS3BucketName: aws.ToString(trail.S3BucketName),
			names.AttrS3KeyPrefix:  aws.ToString(trail.S3KeyPrefix),
		} {
			if got := state[key]; got != want {
				return fmt.Errorf("expected state %s %q from SDK response, got %q", key, want, got)
			}
		}

		conn := acctest.ProviderMeta(ctx, t).CloudTrailClient(ctx)
		trailName := aws.ToString(trail.Name)
		status, err := conn.GetTrailStatus(ctx, &cloudtrail.GetTrailStatusInput{
			Name: aws.String(trailName),
		})
		if err != nil {
			return err
		}
		if want, got := fmt.Sprintf("%t", aws.ToBool(status.IsLogging)), state["enable_logging"]; got != want {
			return fmt.Errorf("expected state enable_logging %q from SDK response, got %q", want, got)
		}

		selectors, err := conn.GetEventSelectors(ctx, &cloudtrail.GetEventSelectorsInput{
			TrailName: aws.String(trailName),
		})
		if err != nil {
			return err
		}
		if selectors == nil {
			return fmt.Errorf("GetEventSelectors returned an empty response")
		}

		if want, got := fmt.Sprintf("%d", len(selectors.EventSelectors)), state["event_selector.#"]; got != want {
			return fmt.Errorf("expected state event_selector count %q from SDK response, got %q", want, got)
		}
		for i, selector := range selectors.EventSelectors {
			key := fmt.Sprintf("event_selector.%d.read_write_type", i)
			if want, got := string(selector.ReadWriteType), state[key]; got != want {
				return fmt.Errorf("expected state %s %q from SDK response, got %q", key, want, got)
			}
		}

		return nil
	}
}

func testAccCheckTrailEventSelectors(ctx context.Context, t *testing.T, n string, want ...types.ReadWriteType) resource.TestCheckFunc {
	t.Helper()

	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		trailName := rs.Primary.Attributes[names.AttrName]
		conn := acctest.ProviderMeta(ctx, t).CloudTrailClient(ctx)
		output, err := conn.GetEventSelectors(ctx, &cloudtrail.GetEventSelectorsInput{
			TrailName: aws.String(trailName),
		})
		if err != nil {
			return fmt.Errorf("getting CloudTrail Trail (%s) event selectors: %w", trailName, err)
		}
		if got := len(output.EventSelectors); got != len(want) {
			return fmt.Errorf("CloudTrail Trail (%s) event selector count = %d, want %d", trailName, got, len(want))
		}

		for i, selector := range output.EventSelectors {
			if got := selector.ReadWriteType; got != want[i] {
				return fmt.Errorf("CloudTrail Trail (%s) event selector %d read_write_type = %q, want %q", trailName, i, got, want[i])
			}
		}

		return nil
	}
}

func testAccCheckTrailDisappearsExternally(ctx context.Context, t *testing.T, resourceName string) resource.TestCheckFunc {
	t.Helper()

	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		trailName := rs.Primary.Attributes[names.AttrName]
		if trailName == "" {
			return fmt.Errorf("CloudTrail Trail (%s) has no name", resourceName)
		}

		conn := acctest.ProviderMeta(ctx, t).CloudTrailClient(ctx)
		if _, err := conn.DeleteTrail(ctx, &cloudtrail.DeleteTrailInput{Name: aws.String(trailName)}); err != nil {
			return fmt.Errorf("externally deleting CloudTrail Trail (%s): %w", trailName, err)
		}

		if _, err := tfresource.RetryUntilNotFound(ctx, cloudTrailResourcePropagationTimeout, func(ctx context.Context) (any, error) {
			return tfcloudtrail.FindTrailByARN(ctx, conn, rs.Primary.ID)
		}); err != nil {
			return fmt.Errorf("waiting for CloudTrail Trail (%s) deletion: %w", trailName, err)
		}

		return nil
	}
}

func testAccCheckTrailRemovedFromState(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if _, ok := s.RootModule().Resources[resourceName]; ok {
			return fmt.Errorf("CloudTrail Trail resource (%s) is still present in state", resourceName)
		}

		return nil
	}
}

func testAccCheckTrailDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return resource.ComposeTestCheckFunc(
		testAccCheckCTYUNTrailDestroy(ctx, t),
		testAccCheckCloudTrailBucketDestroy(ctx, t),
	)
}

// testAccFillTrailLimitFixture creates out-of-band trails until the account quota is reached.
// These fixtures are intentionally outside Terraform state and must be cleaned independently.
func testAccFillTrailLimitFixture(ctx context.Context, t *testing.T, bucketName string) {
	t.Helper()

	conn := acctest.ProviderMeta(ctx, t).CloudTrailClient(ctx)
	fixturePrefix := acctest.RandomWithPrefix(t, bucketName+"-limit")
	fixtureNames := make([]string, 0, cloudTrailLimitFixtureMax)

	t.Cleanup(func() {
		for i := len(fixtureNames) - 1; i >= 0; i-- {
			if _, err := conn.DeleteTrail(context.Background(), &cloudtrail.DeleteTrailInput{Name: aws.String(fixtureNames[i])}); err != nil && !retry.NotFound(err) {
				t.Errorf("destroying CloudTrail Trail limit fixture (%s): %s", fixtureNames[i], err)
			}
		}
	})

	for i := range cloudTrailLimitFixtureMax {
		trailName := fmt.Sprintf("%s-%03d", fixturePrefix, i)

		_, err := conn.CreateTrail(ctx, &cloudtrail.CreateTrailInput{
			Name:         aws.String(trailName),
			S3BucketName: aws.String(bucketName),
		})
		if err == nil {
			fixtureNames = append(fixtureNames, trailName)
			continue
		}

		if tfawserr.ErrCodeEquals(err, (&types.TrailAlreadyExistsException{}).ErrorCode()) {
			trail, findErr := tfcloudtrail.FindTrailByName(ctx, conn, trailName)
			if findErr != nil {
				t.Fatalf("reading existing CloudTrail Trail limit fixture (%s): %s", trailName, findErr)
			}
			if got := aws.ToString(trail.S3BucketName); got != bucketName {
				t.Fatalf("existing CloudTrail Trail limit fixture (%s) bucket = %q, want %q", trailName, got, bucketName)
			}

			fixtureNames = append(fixtureNames, trailName)
			continue
		}

		if tfawserr.ErrCodeEquals(err, (&types.MaximumNumberOfTrailsExceededException{}).ErrorCode()) {
			return
		}

		t.Fatalf("creating CloudTrail Trail limit fixture (%s): %s", trailName, err)
	}

	t.Fatalf("CloudTrail Trail limit was not reached after %d fixtures", cloudTrailLimitFixtureMax)
}

func testAccCheckCloudTrailBucketDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	t.Helper()

	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_oos_bucket" {
				continue
			}

			_, err := tfresource.RetryUntilNotFound(ctx, cloudTrailResourcePropagationTimeout, func(ctx context.Context) (any, error) {
				return tfs3.FindOOSBucket(ctx, conn, rs.Primary.ID)
			})
			if errors.Is(err, tfresource.ErrFoundResource) {
				return fmt.Errorf("S3 Bucket %s still exists", rs.Primary.ID)
			}
			if err != nil {
				return fmt.Errorf("waiting for S3 Bucket (%s) deletion: %w", rs.Primary.ID, err)
			}
		}

		return nil
	}
}

func testAccCheckCTYUNLoggingStatus(ctx context.Context, t *testing.T, n string, wantLogging bool) resource.TestCheckFunc {
	t.Helper()

	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		trailName := rs.Primary.Attributes[names.AttrName]
		if trailName == "" {
			return fmt.Errorf("missing name attribute for %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).CloudTrailClient(ctx)

		output, err := conn.GetTrailStatus(ctx, &cloudtrail.GetTrailStatusInput{
			Name: aws.String(trailName),
		})
		if err != nil {
			return err
		}

		if got := aws.ToBool(output.IsLogging); got != wantLogging {
			return fmt.Errorf("expected logging status %t, got %t", wantLogging, got)
		}

		return nil
	}
}

func testAccStartTrailLoggingExternally(ctx context.Context, t *testing.T, n string) resource.TestCheckFunc {
	t.Helper()

	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		trailName := rs.Primary.Attributes[names.AttrName]
		conn := acctest.ProviderMeta(ctx, t).CloudTrailClient(ctx)
		if _, err := conn.StartLogging(ctx, &cloudtrail.StartLoggingInput{Name: aws.String(trailName)}); err != nil {
			return fmt.Errorf("externally starting CloudTrail Trail (%s) logging: %w", trailName, err)
		}

		return nil
	}
}

func testAccStopTrailLoggingExternally(ctx context.Context, t *testing.T, n string) resource.TestCheckFunc {
	t.Helper()

	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		trailName := rs.Primary.Attributes[names.AttrName]
		conn := acctest.ProviderMeta(ctx, t).CloudTrailClient(ctx)
		if _, err := conn.StopLogging(ctx, &cloudtrail.StopLoggingInput{Name: aws.String(trailName)}); err != nil {
			return fmt.Errorf("externally stopping CloudTrail Trail (%s) logging: %w", trailName, err)
		}

		return nil
	}
}

func testAccUpdateTrailExternally(ctx context.Context, t *testing.T, n, bucketName, s3KeyPrefix string) resource.TestCheckFunc {
	t.Helper()

	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		trailName := rs.Primary.Attributes[names.AttrName]
		conn := acctest.ProviderMeta(ctx, t).CloudTrailClient(ctx)
		if _, err := conn.UpdateTrail(ctx, &cloudtrail.UpdateTrailInput{
			Name:         aws.String(trailName),
			S3BucketName: aws.String(bucketName),
			S3KeyPrefix:  aws.String(s3KeyPrefix),
		}); err != nil {
			return fmt.Errorf("externally updating CloudTrail Trail (%s): %w", trailName, err)
		}

		if err := sdkretry.RetryContext(ctx, cloudTrailResourcePropagationTimeout, func() *sdkretry.RetryError {
			trail, err := tfcloudtrail.FindTrailByARN(ctx, conn, rs.Primary.ID)
			if err != nil {
				return sdkretry.RetryableError(fmt.Errorf("reading externally updated CloudTrail Trail (%s): %w", trailName, err))
			}
			if aws.ToString(trail.S3BucketName) != bucketName || aws.ToString(trail.S3KeyPrefix) != s3KeyPrefix {
				return sdkretry.RetryableError(fmt.Errorf("waiting for CloudTrail Trail (%s) external update", trailName))
			}

			return nil
		}); err != nil {
			return err
		}

		return nil
	}
}

func testAccPutTrailEventSelectorExternally(ctx context.Context, t *testing.T, n string, readWriteType types.ReadWriteType) resource.TestCheckFunc {
	return testAccPutTrailEventSelectorExternallyExpected(ctx, t, n, readWriteType, types.ReadWriteTypeAll)
}

func testAccPutTrailEventSelectorExternallyExpected(ctx context.Context, t *testing.T, n string, readWriteType, expected types.ReadWriteType) resource.TestCheckFunc {
	t.Helper()

	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		trailName := rs.Primary.Attributes[names.AttrName]
		conn := acctest.ProviderMeta(ctx, t).CloudTrailClient(ctx)
		if _, err := conn.PutEventSelectors(ctx, &cloudtrail.PutEventSelectorsInput{
			TrailName: aws.String(trailName),
			EventSelectors: []types.EventSelector{
				{ReadWriteType: readWriteType},
			},
		}); err != nil {
			return fmt.Errorf("externally setting CloudTrail Trail (%s) event selector: %w", trailName, err)
		}

		return testAccCheckTrailEventSelectors(ctx, t, n, expected)(s)
	}
}

func testAccCheckCTYUNTrailDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	t.Helper()

	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).CloudTrailClient(ctx)

		for resourceName, rs := range s.RootModule().Resources {
			if strings.HasPrefix(resourceName, "data.") || rs.Type != "ctyun_cloudtrail" {
				continue
			}

			_, err := tfcloudtrail.FindTrailByARN(ctx, conn, rs.Primary.ID)

			if retry.NotFound(err) {
				continue
			}

			if err != nil {
				return fmt.Errorf("reading CloudTrail Trail (%s) during destroy check: %w", rs.Primary.ID, err)
			}

			return fmt.Errorf("CloudTrail Trail (%s) still exists", rs.Primary.ID)
		}

		return nil
	}
}

func testAccCTYUNCloudTrailConfig_bucket(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}
`, rName)
}

func testAccCloudTrailConfig_basic(rName, s3KeyPrefix string) string {
	return testAccCloudTrailConfig_withName(rName, rName, s3KeyPrefix)
}

func testAccCloudTrailConfig_withName(bucketName, trailName, s3KeyPrefix string) string {
	if s3KeyPrefix == "" {
		return testAccCloudTrailConfig_withOptionalPrefix(bucketName, trailName, nil)
	}

	return testAccCloudTrailConfig_withExplicitPrefix(bucketName, trailName, s3KeyPrefix)
}

func testAccCloudTrailConfig_withExplicitPrefix(bucketName, trailName, s3KeyPrefix string) string {
	return testAccCloudTrailConfig_withOptionalPrefix(bucketName, trailName, &s3KeyPrefix)
}

func testAccCloudTrailConfig_withOptionalPrefix(bucketName, trailName string, s3KeyPrefix *string) string {
	prefixConfig := ""
	if s3KeyPrefix != nil {
		prefixConfig = fmt.Sprintf("  s3_key_prefix  = %q\n", *s3KeyPrefix)
	}

	return acctest.ConfigCompose(
		testAccCTYUNCloudTrailConfig_bucket(bucketName),
		fmt.Sprintf(`
resource "ctyun_cloudtrail" "test" {
  name           = %[1]q
  s3_bucket_name = ctyun_oos_bucket.test.bucket
%[2]s
}
`, trailName, prefixConfig),
	)
}

func testAccCloudTrailConfig_duplicateName(rName string, includeDuplicate bool) string {
	secondResource := ""
	if includeDuplicate {
		secondResource = fmt.Sprintf(`
resource "ctyun_cloudtrail" "second" {
  name           = %[1]q
  s3_bucket_name = ctyun_oos_bucket.test.bucket
  enable_logging = false
}
`, rName)
	}

	return acctest.ConfigCompose(
		testAccCTYUNCloudTrailConfig_bucket(rName),
		fmt.Sprintf(`
resource "ctyun_cloudtrail" "first" {
  name           = %[1]q
  s3_bucket_name = ctyun_oos_bucket.test.bucket
  enable_logging = false
}
%[2]s`, rName, secondResource),
	)
}

func testAccCloudTrailConfig_missingBucket(trailName string) string {
	return fmt.Sprintf(`
resource "ctyun_cloudtrail" "test" {
  name = %[1]q
}
`, trailName)
}

func testAccCloudTrailConfig_limitExceeded(rName string) string {
	return acctest.ConfigCompose(
		testAccCTYUNCloudTrailConfig_bucket(rName),
		fmt.Sprintf(`
resource "ctyun_cloudtrail" "test" {
  name           = %[1]q
  s3_bucket_name = ctyun_oos_bucket.test.bucket
  enable_logging = false
}
`, rName),
	)
}

func testAccCloudTrailTrailNameWithLength(prefix string, length int) string {
	if len(prefix) >= length {
		return prefix[:length]
	}

	return prefix + strings.Repeat("a", length-len(prefix))
}

func testAccCloudTrailConfig_withBucket(trailName, bucketName, s3KeyPrefix string) string {
	prefixConfig := ""
	if s3KeyPrefix != "" {
		prefixConfig = fmt.Sprintf("  s3_key_prefix  = %q\n", s3KeyPrefix)
	}

	return fmt.Sprintf(`
resource "ctyun_cloudtrail" "test" {
  name           = %[1]q
  s3_bucket_name = %[2]q
%[3]s
}
`, trailName, bucketName, prefixConfig)
}

func testAccCloudTrailConfig_basicUpdate(trailName, initialBucketName, updatedBucketName, s3KeyPrefix string, useUpdatedBucket bool) string {
	return testAccCloudTrailConfig_basicUpdateWithOptionalPrefix(trailName, initialBucketName, updatedBucketName, &s3KeyPrefix, useUpdatedBucket)
}

func testAccCloudTrailConfig_basicUpdateWithoutPrefix(trailName, initialBucketName, updatedBucketName string, useUpdatedBucket bool) string {
	return testAccCloudTrailConfig_basicUpdateWithOptionalPrefix(trailName, initialBucketName, updatedBucketName, nil, useUpdatedBucket)
}

func testAccCloudTrailConfig_basicUpdateWithOptionalPrefix(trailName, initialBucketName, updatedBucketName string, s3KeyPrefix *string, useUpdatedBucket bool) string {
	bucketResourceName := "initial"
	if useUpdatedBucket {
		bucketResourceName = "updated"
	}
	prefixConfig := ""
	if s3KeyPrefix != nil {
		prefixConfig = fmt.Sprintf("  s3_key_prefix  = %q\n", *s3KeyPrefix)
	}

	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "initial" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket" "updated" {
  bucket = %[2]q
}

resource "ctyun_cloudtrail" "test" {
  name           = %[3]q
  s3_bucket_name = ctyun_oos_bucket.%[4]s.bucket
%[5]s
}
`, initialBucketName, updatedBucketName, trailName, bucketResourceName, prefixConfig)
}

func testAccCTYUNCloudTrailConfig_enableLogging(rName string, enableLogging bool) string {
	return acctest.ConfigCompose(
		testAccCTYUNCloudTrailConfig_bucket(rName),
		fmt.Sprintf(`
resource "ctyun_cloudtrail" "test" {
  name           = %[1]q
  s3_bucket_name = ctyun_oos_bucket.test.bucket
  enable_logging = %[2]t
}
`, rName, enableLogging),
	)
}

func testAccCloudTrailConfig_eventSelector(rName string, readWriteType types.ReadWriteType) string {
	return acctest.ConfigCompose(
		testAccCTYUNCloudTrailConfig_bucket(rName),
		fmt.Sprintf(`
resource "ctyun_cloudtrail" "test" {
  name           = %[1]q
  s3_bucket_name = ctyun_oos_bucket.test.bucket
  enable_logging = false

  event_selector {
    read_write_type = %[2]q
  }
}
`, rName, readWriteType),
	)
}

func testAccCloudTrailConfig_withoutEventSelector(rName string) string {
	return acctest.ConfigCompose(
		testAccCTYUNCloudTrailConfig_bucket(rName),
		fmt.Sprintf(`
resource "ctyun_cloudtrail" "test" {
  name           = %[1]q
  s3_bucket_name = ctyun_oos_bucket.test.bucket
  enable_logging = false
}
`, rName),
	)
}

func testAccCloudTrailConfig_updateWithEventSelector(trailName, initialBucketName, updatedBucketName, s3KeyPrefix string, useUpdatedBucket bool, readWriteType types.ReadWriteType) string {
	bucketResourceName := "initial"
	if useUpdatedBucket {
		bucketResourceName = "updated"
	}

	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "initial" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket" "updated" {
  bucket = %[2]q
}

resource "ctyun_cloudtrail" "test" {
  name           = %[3]q
  s3_bucket_name = ctyun_oos_bucket.%[4]s.bucket
  s3_key_prefix  = %[5]q
  enable_logging = false

  event_selector {
    read_write_type = %[6]q
  }
}
`, initialBucketName, updatedBucketName, trailName, bucketResourceName, s3KeyPrefix, readWriteType)
}
