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
	awspolicy "github.com/hashicorp/awspolicyequivalence"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TC-S3-POLICY-01 Basic CRUD - create, read, and delete a bucket policy.
// TestAccS3BucketPolicy_Basic_basic verifies the creation, read, and deletion of a basic Bucket Policy.
func TestAccS3BucketPolicy_Basic_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_policy.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketPolicyConfig_basic(rName, "oos:*"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicyExists(ctx, t, resourceName),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, rName),
					resource.TestCheckResourceAttrPair(resourceName, names.AttrBucket, "ctyun_oos_bucket.test", names.AttrBucket),
					testAccCheckBucketPolicyEquivalent(resourceName, names.AttrPolicy, testAccBucketPolicyExpectedPolicy(rName, "oos:*")),
				),
			},
		},
	})
}

// TC-S3-POLICY-09 Bucket ForceNew - replace a policy when its bucket changes.
// TestAccS3BucketPolicy_BucketForceNew_bucketChange verifies that modifying bucket destroys and re-creates the policy, with the new state using the new bucket ID.
func TestAccS3BucketPolicy_BucketForceNew_bucketChange(t *testing.T) {
	ctx := acctest.Context(t)
	initialBucket := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	replacementBucket := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_policy.test"
	initialPolicy := testAccBucketPolicyExpectedPolicy(initialBucket, "oos:GetObject")
	replacementPolicy := testAccBucketPolicyExpectedPolicy(replacementBucket, "oos:GetObject")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketPolicyConfig_policy(initialBucket, initialPolicy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, initialPolicy),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, initialBucket),
					resource.TestCheckResourceAttr(resourceName, names.AttrBucket, initialBucket),
				),
			},
			{
				Config: testAccBucketPolicyConfig_policy(replacementBucket, replacementPolicy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, replacementPolicy),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, replacementBucket),
					resource.TestCheckResourceAttr(resourceName, names.AttrBucket, replacementBucket),
				),
			},
		},
	})
}

// TC-S3-POLICY-05 Disappears - remote policy deletion.
// TestAccS3BucketPolicy_Disappears_policy verifies that the provider corrects the state after only the policy is deleted remotely.
func TestAccS3BucketPolicy_Disappears_policy(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_policy.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketPolicyConfig_basic(rName, "oos:*"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicyExists(ctx, t, resourceName),
					acctest.CheckSDKResourceDisappears(ctx, t, tfs3.ResourceBucketPolicy(), resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TC-S3-POLICY-05 Disappears - remote parent bucket deletion.
// TestAccS3BucketPolicy_Disappears_bucket verifies that after the entire Bucket is deleted remotely, the provider detects the policy is gone and corrects the state.
func TestAccS3BucketPolicy_Disappears_bucket(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_policy.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketPolicyConfig_basic(rName, "oos:*"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicyExists(ctx, t, resourceName),
					acctest.CheckSDKResourceDisappears(ctx, t, tfs3.ResourceBucket(), "ctyun_oos_bucket.test"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TC-S3-POLICY-06 Import - import an existing policy by bucket name.
// TestAccS3BucketPolicy_Import_bucketOnly verifies that only bucket is accepted as the import ID, and that the imported state matches a normal creation.
func TestAccS3BucketPolicy_Import_bucketOnly(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_policy.test"
	policy := testAccBucketPolicyExpectedPolicy(rName, "oos:*")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketPolicyConfig_policy(rName, policy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, policy),
				),
			},
			{
				ResourceName: resourceName,
				ImportState:  true,
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return rName, nil
				},
				ImportStateVerify: true,
			},
			{
				ResourceName: resourceName,
				ImportState:  true,
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("arn:ctyun:oos:::%s", rName), nil
				},
				ExpectError: regexache.MustCompile(`(?i)(unexpected format|expected BUCKET|reading OOS Bucket Policy)`),
			},
			{
				ResourceName: resourceName,
				ImportState:  true,
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s/policy", rName), nil
				},
				ExpectError: regexache.MustCompile(`(?i)(unexpected format|expected BUCKET|reading OOS Bucket Policy)`),
			},
		},
	})
}

// TC-S3-POLICY-06 Import - import a policy with multiple statements and a condition.
// TestAccS3BucketPolicy_Import_complexPolicy verifies that importing a policy with multiple Statements and a Condition yields state identical to a normal creation.
func TestAccS3BucketPolicy_Import_complexPolicy(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_policy.test"
	policy := testAccBucketPolicyExpectedPolicyWithAdditionalStatementAndCondition(rName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketPolicyConfig_policy(rName, policy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, policy),
				),
			},
			{
				ResourceName: resourceName,
				ImportState:  true,
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return rName, nil
				},
				ImportStateVerify: true,
			},
		},
	})
}

// TC-S3-POLICY-07 Invalid Policy Negative - invalid policy and bucket inputs.
// TestAccS3BucketPolicy_InvalidPolicyNegative_invalidInputs verifies that invalid policy or bucket inputs return clear errors.
func TestAccS3BucketPolicy_InvalidPolicyNegative_invalidInputs(t *testing.T) {
	testCases := []struct {
		name        string
		config      func(string) string
		errorRegexp string
	}{
		{
			name: "missing policy",
			config: func(rName string) string {
				return testAccBucketPolicyConfig_missingPolicy(rName)
			},
			errorRegexp: `(?i)(missing required argument|argument "policy" is required)`,
		},
		{
			name: "empty policy",
			config: func(rName string) string {
				return testAccBucketPolicyConfig_policyExpression(rName, `""`)
			},
			errorRegexp: `api error InvalidPolicyDocument`,
		},
		{
			name: "invalid JSON",
			config: func(rName string) string {
				return testAccBucketPolicyConfig_policyExpression(rName, `"{ invalid JSON"`)
			},
			errorRegexp: `"policy" contains an invalid JSON`,
		},
		{
			name: "malformed policy accepted by JSON parser",
			config: func(rName string) string {
				return testAccBucketPolicyConfig_policyExpression(rName, `jsonencode({
  Version = "2012-10-17"
  Statement = [{
    Effect = "Allow"
  }]
})`)
			},
			errorRegexp: `api error InvalidPolicyDocument`,
		},
		{
			name: "invalid bucket name consecutive dots",
			config: func(_ string) string {
				return testAccBucketPolicyConfig_policyOnlyExpression("foo..bar", testAccBucketPolicyValidPolicyExpression)
			},
			errorRegexp: `(?i)(InvalidBucketName|invalid bucket|bucket[^\n]*invalid)`,
		},
		{
			name: "invalid bucket name uppercase",
			config: func(_ string) string {
				return testAccBucketPolicyConfig_policyOnlyExpression("Foo.Bar", testAccBucketPolicyValidPolicyExpression)
			},
			errorRegexp: `must contain only lowercase letters, numbers, periods, and hyphens`,
		},
		{
			name: "invalid bucket name public IPv4",
			config: func(_ string) string {
				return testAccBucketPolicyConfig_policyOnlyExpression("192.168.0.1", testAccBucketPolicyValidPolicyExpression)
			},
			errorRegexp: `(?i)(InvalidBucketName|invalid bucket|bucket[^\n]*invalid)`,
		},
		{
			name: "invalid bucket name loopback IPv4",
			config: func(_ string) string {
				return testAccBucketPolicyConfig_policyOnlyExpression("127.0.0.1", testAccBucketPolicyValidPolicyExpression)
			},
			errorRegexp: `(?i)(InvalidBucketName|invalid bucket|bucket[^\n]*invalid)`,
		},
		{
			name: "invalid bucket name leading dot",
			config: func(_ string) string {
				return testAccBucketPolicyConfig_policyOnlyExpression(".foo", testAccBucketPolicyValidPolicyExpression)
			},
			errorRegexp: `(?i)(InvalidBucketName|invalid bucket|bucket[^\n]*invalid)`,
		},
		{
			name: "invalid bucket name trailing dot",
			config: func(_ string) string {
				return testAccBucketPolicyConfig_policyOnlyExpression("bar.", testAccBucketPolicyValidPolicyExpression)
			},
			errorRegexp: `(?i)(InvalidBucketName|invalid bucket|bucket[^\n]*invalid)`,
		},
		{
			name: "invalid bucket name underscore",
			config: func(_ string) string {
				return testAccBucketPolicyConfig_policyOnlyExpression("foo_bar", testAccBucketPolicyValidPolicyExpression)
			},
			errorRegexp: `must contain only lowercase letters, numbers, periods, and hyphens`,
		},
		{
			name: "invalid bucket name too long",
			config: func(_ string) string {
				return testAccBucketPolicyConfig_policyOnlyExpression(strings.Repeat("a", 64), testAccBucketPolicyValidPolicyExpression)
			},
			errorRegexp: `expected length of bucket to be in the range \(3 - 63\), got`,
		},
		{
			name: "bucket does not exist",
			config: func(rName string) string {
				return testAccBucketPolicyConfig_policyOnlyExpression("missing-"+rName, testAccBucketPolicyValidPolicyExpression)
			},
			errorRegexp: `api error NoSuchBucket`,
		},
		{
			name: "empty bucket name",
			config: func(_ string) string {
				return testAccBucketPolicyConfig_policyOnlyExpression("", testAccBucketPolicyValidPolicyExpression)
			},
			errorRegexp: `(?i)(expected length of bucket to be in the range \(3 - 63\)|invalid value for bucket)`,
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
				CheckDestroy:             testAccCheckBucketPolicyDestroy(ctx, t),
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

// TC-S3-POLICY-08 Cross Account Read Negative - read a policy owned by another account.
// TestAccS3BucketPolicy_CrossAccountReadNegative_read verifies that another account's policy is not visible to the current account.
func TestAccS3BucketPolicy_CrossAccountReadNegative_read(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.SkipIfEnvVarNotSet(t, "CTYUNOOS_OTHER_ACCOUNT_BUCKET_NAME")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketPolicyConfig_crossAccountRead(bucketName),
				ExpectError: regexache.MustCompile(`(?i)AccessDenied:\s*Access Denied`),
			},
		},
	})
}

// TC-S3-POLICY-08 Cross Account Read Negative - write a policy owned by another account.
// TestAccS3BucketPolicy_CrossAccountReadNegative_write verifies that the current account cannot write another account's policy.
func TestAccS3BucketPolicy_CrossAccountReadNegative_write(t *testing.T) {
	ctx := acctest.Context(t)
	bucketName := acctest.SkipIfEnvVarNotSet(t, "CTYUNOOS_OTHER_ACCOUNT_BUCKET_NAME")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyNoop,
		Steps: []resource.TestStep{
			{
				Config:      testAccBucketPolicyConfig_crossAccountWrite(bucketName),
				ExpectError: regexache.MustCompile(`(?i)AccessDenied:\s*Access Denied`),
			},
		},
	})
}

// TC-S3-POLICY-02 Policy Update - action narrowing.
// TestAccS3BucketPolicy_PolicyUpdate_action verifies that the policy shows no drift after the Action is updated.
func TestAccS3BucketPolicy_PolicyUpdate_action(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_policy.test"
	initialPolicy := testAccBucketPolicyExpectedPolicy(rName, "oos:*")
	updatedPolicy := testAccBucketPolicyExpectedPolicy(rName, "oos:GetObject")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketPolicyConfig_policy(rName, initialPolicy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, initialPolicy),
				),
			},
			{
				Config: testAccBucketPolicyConfig_policy(rName, updatedPolicy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, updatedPolicy),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config:   testAccBucketPolicyConfig_policy(rName, updatedPolicy),
				PlanOnly: true,
			},
		},
	})
}

// TC-S3-POLICY-02 Policy Update - effect replacement.
// TestAccS3BucketPolicy_PolicyUpdate_deny verifies that after the Effect is updated to Deny, the remote policy and state stay in sync and the plan remains stable.
func TestAccS3BucketPolicy_PolicyUpdate_deny(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_policy.test"
	initialPolicy := testAccBucketPolicyExpectedPolicyWithEffect(rName, "Allow")
	updatedPolicy := testAccBucketPolicyExpectedPolicyWithEffect(rName, "Deny")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketPolicyConfig_policy(rName, initialPolicy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, initialPolicy),
				),
			},
			{
				Config: testAccBucketPolicyConfig_policy(rName, updatedPolicy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, updatedPolicy),
				),
			},
			{
				Config:   testAccBucketPolicyConfig_policy(rName, updatedPolicy),
				PlanOnly: true,
			},
		},
	})
}

// TC-S3-POLICY-02 Policy Update - resource add and remove.
// TestAccS3BucketPolicy_PolicyUpdate_resource verifies that the policy shows no drift after the Resource list is updated.
func TestAccS3BucketPolicy_PolicyUpdate_resource(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_policy.test"
	initialPolicy := testAccBucketPolicyExpectedPolicy(rName, "oos:GetObject")
	updatedPolicy := testAccBucketPolicyExpectedPolicyWithResources(rName, `[
        "arn:ctyun:oos:::%[1]s/*",
        "arn:ctyun:oos:::%[1]s/logs/*"
      ]`)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketPolicyConfig_policy(rName, initialPolicy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, initialPolicy),
				),
			},
			{
				Config: testAccBucketPolicyConfig_policy(rName, updatedPolicy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, updatedPolicy),
				),
			},
			{
				Config: testAccBucketPolicyConfig_policy(rName, initialPolicy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, initialPolicy),
				),
			},
			{
				Config:   testAccBucketPolicyConfig_policy(rName, initialPolicy),
				PlanOnly: true,
			},
		},
	})
}

// TC-S3-POLICY-02 Policy Update - statement add and remove.
// TestAccS3BucketPolicy_PolicyUpdate_statement verifies that the policy shows no drift after Statements are updated.
func TestAccS3BucketPolicy_PolicyUpdate_statement(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_policy.test"
	initialPolicy := testAccBucketPolicyExpectedPolicy(rName, "oos:GetObject")
	updatedPolicy := testAccBucketPolicyExpectedPolicyWithAdditionalStatement(rName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketPolicyConfig_policy(rName, initialPolicy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, initialPolicy),
				),
			},
			{
				Config: testAccBucketPolicyConfig_policy(rName, updatedPolicy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, updatedPolicy),
				),
			},
			{
				Config: testAccBucketPolicyConfig_policy(rName, initialPolicy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, initialPolicy),
				),
			},
			{
				Config:   testAccBucketPolicyConfig_policy(rName, initialPolicy),
				PlanOnly: true,
			},
		},
	})
}

// TC-S3-POLICY-02 Policy Update - condition modification.
// TestAccS3BucketPolicy_PolicyUpdate_condition verifies that the policy shows no drift after the Condition is updated.
func TestAccS3BucketPolicy_PolicyUpdate_condition(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_policy.test"
	initialPolicy := testAccBucketPolicyExpectedPolicy(rName, "oos:GetObject")
	updatedPolicy := testAccBucketPolicyExpectedPolicyWithCondition(rName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketPolicyConfig_policy(rName, initialPolicy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, initialPolicy),
				),
			},
			{
				Config: testAccBucketPolicyConfig_policy(rName, updatedPolicy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, updatedPolicy),
				),
			},
			{
				Config:   testAccBucketPolicyConfig_policy(rName, updatedPolicy),
				PlanOnly: true,
			},
		},
	})
}

// TC-S3-POLICY-02 Policy Update - principal content replacement.
// TestAccS3BucketPolicy_PolicyUpdate_principal verifies that after the Principal content is updated, the remote policy and state stay in sync.
func TestAccS3BucketPolicy_PolicyUpdate_principal(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_policy.test"
	initialPolicy := testAccBucketPolicyExpectedPolicyWithPrincipal(rName, "arn:ctyun:iam:::user/test")
	updatedPolicy := testAccBucketPolicyExpectedPolicyWithPrincipal(rName, "arn:ctyun:iam:::user/updated-test")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketPolicyConfig_policy(rName, initialPolicy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, initialPolicy),
				),
			},
			{
				Config: testAccBucketPolicyConfig_policy(rName, updatedPolicy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, updatedPolicy),
				),
			},
			{
				Config:   testAccBucketPolicyConfig_policy(rName, updatedPolicy),
				PlanOnly: true,
			},
		},
	})
}

// TC-S3-POLICY-02 Policy Update - wildcard principal.
// TestAccS3BucketPolicy_PolicyUpdate_principalWildcard verifies that after the Principal is updated to a wildcard, the remote policy and state stay in sync and the plan remains stable.
func TestAccS3BucketPolicy_PolicyUpdate_principalWildcard(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_policy.test"
	initialPolicy := testAccBucketPolicyExpectedPolicyWithPrincipal(rName, "arn:ctyun:iam:::user/test")
	updatedPolicy := testAccBucketPolicyExpectedPolicyWithWildcardPrincipal(rName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketPolicyConfig_policy(rName, initialPolicy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, initialPolicy),
				),
			},
			{
				Config: testAccBucketPolicyConfig_policy(rName, updatedPolicy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, updatedPolicy),
				),
			},
			{
				Config:   testAccBucketPolicyConfig_policy(rName, updatedPolicy),
				PlanOnly: true,
			},
		},
	})
}

// TC-S3-POLICY-03 Equivalent JSON - whitespace, indentation, and line breaks.
// TestAccS3BucketPolicy_EquivalentJSON_whitespace verifies that adjusting only whitespace, indentation, and line breaks produces no drift.
func TestAccS3BucketPolicy_EquivalentJSON_whitespace(t *testing.T) {
	testAccBucketPolicyEquivalentJSON(t,
		func(rName string) string { return testAccBucketPolicyExpectedPolicy(rName, "oos:GetObject") },
		testAccBucketPolicyEquivalentJSONWhitespace,
	)
}

// TC-S3-POLICY-03 Equivalent JSON - object key order.
// TestAccS3BucketPolicy_EquivalentJSON_keyOrder verifies that changing the object key order produces no drift.
func TestAccS3BucketPolicy_EquivalentJSON_keyOrder(t *testing.T) {
	testAccBucketPolicyEquivalentJSON(t,
		func(rName string) string { return testAccBucketPolicyExpectedPolicy(rName, "oos:GetObject") },
		testAccBucketPolicyEquivalentJSONKeyOrder,
	)
}

// TC-S3-POLICY-03 Equivalent JSON - statement, action, and resource list order.
// TestAccS3BucketPolicy_EquivalentJSON_listOrder verifies that changing the order of the Statement, Action, and Resource lists produces no drift.
func TestAccS3BucketPolicy_EquivalentJSON_listOrder(t *testing.T) {
	testAccBucketPolicyEquivalentJSON(t,
		testAccBucketPolicyEquivalentJSONListOrderInitial,
		testAccBucketPolicyEquivalentJSONListOrderReordered,
	)
}

// TC-S3-POLICY-03 Equivalent JSON - apply one policy then plan with an equivalent replacement.
// TestAccS3BucketPolicy_EquivalentJSON_applyThenPlan verifies that the plan after an equivalent JSON replacement shows no diff.
func TestAccS3BucketPolicy_EquivalentJSON_applyThenPlan(t *testing.T) {
	testAccBucketPolicyEquivalentJSON(t,
		func(rName string) string { return testAccBucketPolicyExpectedPolicy(rName, "oos:*") },
		testAccBucketPolicyEquivalentJSONApplyThenPlan,
	)
}

func testAccBucketPolicyEquivalentJSON(t *testing.T, initialPolicy, equivalentPolicy func(string) string) {
	t.Helper()

	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_policy.test"
	initial := initialPolicy(rName)
	equivalent := equivalentPolicy(rName)

	areEquivalent, err := awspolicy.PoliciesAreEquivalent(initial, equivalent)
	if err != nil {
		t.Fatalf("comparing equivalent IAM Policies failed: %s", err)
	}
	if !areEquivalent {
		t.Fatal("test policies must be semantically equivalent")
	}

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketPolicyConfig_policy(rName, initial),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, initial),
				),
			},
			{
				Config: testAccBucketPolicyConfig_policy(rName, equivalent),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, equivalent),
				),
			},
			{
				Config:   testAccBucketPolicyConfig_policy(rName, equivalent),
				PlanOnly: true,
			},
		},
	})
}

// TC-S3-POLICY-04 Remove Policy - remove and restore a bucket policy.
// TestAccS3BucketPolicy_RemovePolicy_fromConfig verifies that after the policy is removed, the remote deletion succeeds, and it can be restored from a state with no stale residue.
func TestAccS3BucketPolicy_RemovePolicy_fromConfig(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_oos_bucket_policy.test"
	policy := testAccBucketPolicyExpectedPolicy(rName, "oos:*")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.S3ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckBucketPolicyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccBucketPolicyConfig_policy(rName, policy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, policy),
				),
			},
			{
				Config: testAccBucketPolicyConfig_bucketOnly(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicyNotExists(ctx, t, rName),
				),
			},
			{
				Config: testAccBucketPolicyConfig_policy(rName, policy),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckBucketPolicySynchronized(ctx, t, resourceName, policy),
				),
			},
			{
				Config:   testAccBucketPolicyConfig_policy(rName, policy),
				PlanOnly: true,
			},
		},
	})
}

func testAccCheckBucketPolicyDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_oos_bucket_policy" {
				continue
			}

			_, err := tfs3.FindBucketPolicy(ctx, conn, rs.Primary.ID)

			if retry.NotFound(err) {
				continue
			}

			if err != nil {
				return err
			}

			return fmt.Errorf("OOS Bucket Policy %s still exists", rs.Primary.ID)
		}

		return nil
	}
}

func testAccCheckBucketPolicyExists(ctx context.Context, t *testing.T, n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		_, err := tfs3.FindBucketPolicy(ctx, conn, rs.Primary.ID)
		return err
	}
}

func testAccCheckBucketPolicyNotExists(ctx context.Context, t *testing.T, bucket string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)

		_, err := tfs3.FindBucketPolicy(ctx, conn, bucket)
		if retry.NotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}

		return fmt.Errorf("OOS Bucket Policy %s still exists", bucket)
	}
}

func testAccCheckBucketPolicySynchronized(ctx context.Context, t *testing.T, name, expected string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		testAccCheckBucketPolicyExists(ctx, t, name),
		testAccCheckBucketPolicyEquivalent(name, names.AttrPolicy, expected),
		testAccCheckBucketPolicyRemoteEquivalent(ctx, t, name, expected),
	)
}

func testAccCheckBucketPolicyRemoteEquivalent(ctx context.Context, t *testing.T, name, expected string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("Not found: %s", name)
		}

		conn := acctest.ProviderMeta(ctx, t).S3Client(ctx)
		actual, err := tfs3.FindBucketPolicy(ctx, conn, rs.Primary.ID)
		if err != nil {
			return err
		}

		areEquivalent, err := awspolicy.PoliciesAreEquivalent(actual, expected)
		if err != nil {
			return fmt.Errorf("comparing remote IAM Policies failed: %w", err)
		}
		if !areEquivalent {
			return fmt.Errorf("remote OOS bucket policy differs from expected.\nactual: %s\nexpected: %s", actual, expected)
		}

		return nil
	}
}
func testAccCheckBucketPolicyEquivalent(name, key, expected string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("Not found: %s", name)
		}

		actual, ok := rs.Primary.Attributes[key]
		if !ok {
			return fmt.Errorf("attribute %q not found for %q", key, name)
		}

		areEquivalent, err := awspolicy.PoliciesAreEquivalent(actual, expected)
		if err != nil {
			return fmt.Errorf("comparing IAM Policies failed: %w", err)
		}
		if !areEquivalent {
			return fmt.Errorf("S3 bucket policy differs from expected.\nactual: %s\nexpected: %s", actual, expected)
		}

		return nil
	}
}

func testAccBucketPolicyExpectedPolicy(rName, action string) string {
	return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Id": "*",
  "Statement": [
    {
      "Sid": "*",
      "Effect": "Allow",
      "Principal": {
        "CTYUN": "arn:ctyun:iam:::user/test"
      },
      "Action": %[2]q,
      "Resource": "arn:ctyun:oos:::%[1]s/*"
    }
  ]
}`, rName, action)
}

func testAccBucketPolicyExpectedPolicyWithEffect(rName, effect string) string {
	return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Id": "*",
  "Statement": [
    {
      "Sid": "*",
      "Effect": %[2]q,
      "Principal": {
        "CTYUN": "arn:ctyun:iam:::user/test"
      },
      "Action": "oos:GetObject",
      "Resource": "arn:ctyun:oos:::%[1]s/*"
    }
  ]
}`, rName, effect)
}

func testAccBucketPolicyEquivalentJSONWhitespace(rName string) string {
	return fmt.Sprintf(`
{

  "Version" : "2012-10-17" ,
  "Id" : "*" ,
  "Statement" : [
    {
      "Sid" : "*" ,
      "Effect" : "Allow" ,
      "Principal" : { "CTYUN" : "arn:ctyun:iam:::user/test" },
      "Action" : "oos:GetObject" ,
      "Resource" : "arn:ctyun:oos:::%[1]s/*"
    }
  ]

}
`, rName)
}

func testAccBucketPolicyEquivalentJSONKeyOrder(rName string) string {
	return fmt.Sprintf(`{"Statement":[{"Resource":"arn:ctyun:oos:::%[1]s/*","Action":"oos:GetObject","Principal":{"CTYUN":"arn:ctyun:iam:::user/test"},"Effect":"Allow","Sid":"*"}],"Id":"*","Version":"2012-10-17"}`, rName)
}

func testAccBucketPolicyEquivalentJSONListOrderInitial(rName string) string {
	return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "ObjectAccess",
      "Effect": "Allow",
      "Principal": {"CTYUN": "arn:ctyun:iam:::user/test"},
      "Action": ["oos:GetObject", "oos:PutObject"],
      "Resource": ["arn:ctyun:oos:::%[1]s/*", "arn:ctyun:oos:::%[1]s/logs/*"]
    },
    {
      "Sid": "SecondaryAccess",
      "Effect": "Allow",
      "Principal": {"CTYUN": "arn:ctyun:iam:::user/test"},
      "Action": ["oos:GetObject", "oos:PutObject"],
      "Resource": ["arn:ctyun:oos:::%[1]s/*", "arn:ctyun:oos:::%[1]s/logs/*"]
    }
  ]
}`, rName)
}

func testAccBucketPolicyEquivalentJSONListOrderReordered(rName string) string {
	return fmt.Sprintf(`{
  "Statement": [
    {
      "Resource": ["arn:ctyun:oos:::%[1]s/logs/*", "arn:ctyun:oos:::%[1]s/*"],
      "Action": ["oos:PutObject", "oos:GetObject"],
      "Principal": {"CTYUN": "arn:ctyun:iam:::user/test"},
      "Effect": "Allow",
      "Sid": "SecondaryAccess"
    },
    {
      "Resource": ["arn:ctyun:oos:::%[1]s/logs/*", "arn:ctyun:oos:::%[1]s/*"],
      "Action": ["oos:PutObject", "oos:GetObject"],
      "Principal": {"CTYUN": "arn:ctyun:iam:::user/test"},
      "Effect": "Allow",
      "Sid": "ObjectAccess"
    }
  ],
  "Version": "2012-10-17"
}`, rName)
}

func testAccBucketPolicyEquivalentJSONApplyThenPlan(rName string) string {
	return fmt.Sprintf(`{
  "Statement": [
    {
      "Resource": "arn:ctyun:oos:::%[1]s/*",
      "Principal": {"CTYUN": "arn:ctyun:iam:::user/test"},
      "Action": "oos:*",
      "Sid": "*",
      "Effect": "Allow"
    }
  ],
  "Id": "*",
  "Version": "2012-10-17"
}`, rName)
}
func testAccBucketPolicyExpectedPolicyWithPrincipal(rName, principal string) string {
	return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Id": "*",
  "Statement": [
    {
      "Sid": "*",
      "Effect": "Allow",
      "Principal": {
        "CTYUN": %[2]q
      },
      "Action": "oos:GetObject",
      "Resource": "arn:ctyun:oos:::%[1]s/*"
    }
  ]
}`, rName, principal)
}

func testAccBucketPolicyExpectedPolicyWithWildcardPrincipal(rName string) string {
	return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Id": "*",
  "Statement": [
    {
      "Sid": "*",
      "Effect": "Allow",
      "Principal": "*",
      "Action": "oos:GetObject",
      "Resource": "arn:ctyun:oos:::%[1]s/*"
    }
  ]
}`, rName)
}

func testAccBucketPolicyExpectedPolicyWithConditionValue(rName, value string) string {
	return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Id": "*",
  "Statement": [
    {
      "Sid": "*",
      "Effect": "Allow",
      "Principal": {
        "CTYUN": "arn:ctyun:iam:::user/test"
      },
      "Action": "oos:GetObject",
      "Resource": "arn:ctyun:oos:::%[1]s/*",
      "Condition": {
        "StringLike": {
          "ctyun:Referer": [%[2]q]
        }
      }
    }
  ]
}`, rName, value)
}
func testAccBucketPolicyExpectedPolicyWithResources(rName, resources string) string {
	return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Id": "*",
  "Statement": [
    {
      "Sid": "*",
      "Effect": "Allow",
      "Principal": {
        "CTYUN": "arn:ctyun:iam:::user/test"
      },
      "Action": "oos:GetObject",
      "Resource": `+resources+`
    }
  ]
}`, rName)
}

func testAccBucketPolicyExpectedPolicyWithAdditionalStatement(rName string) string {
	return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Id": "*",
  "Statement": [
    {
      "Sid": "*",
      "Effect": "Allow",
      "Principal": {
        "CTYUN": "arn:ctyun:iam:::user/test"
      },
      "Action": "oos:GetObject",
      "Resource": "arn:ctyun:oos:::%[1]s/*"
    },
    {
      "Sid": "ListBucket",
      "Effect": "Allow",
      "Principal": {
        "CTYUN": "arn:ctyun:iam:::user/test"
      },
      "Action": "oos:ListBucket",
      "Resource": "arn:ctyun:oos:::%[1]s"
    }
  ]
}`, rName)
}

func testAccBucketPolicyExpectedPolicyWithAdditionalStatementAndCondition(rName string) string {
	return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Id": "*",
  "Statement": [
    {
      "Sid": "ObjectAccess",
      "Effect": "Allow",
      "Principal": {
        "CTYUN": "arn:ctyun:iam:::user/test"
      },
      "Action": "oos:GetObject",
      "Resource": "arn:ctyun:oos:::%[1]s/*",
      "Condition": {
        "StringLike": {
          "ctyun:Referer": ["https://www.ctyun.cn/*"]
        }
      }
    },
    {
      "Sid": "ListBucket",
      "Effect": "Allow",
      "Principal": {
        "CTYUN": "arn:ctyun:iam:::user/test"
      },
      "Action": "oos:ListBucket",
      "Resource": "arn:ctyun:oos:::%[1]s"
    }
  ]
}`, rName)
}

func testAccBucketPolicyExpectedPolicyWithCondition(rName string) string {
	return testAccBucketPolicyExpectedPolicyWithConditionValue(rName, "https://www.ctyun.cn/*")
}

const testAccBucketPolicyValidPolicyExpression = `jsonencode({
  Version = "2012-10-17"
  Statement = [{
    Effect    = "Allow"
    Principal = { CTYUN = "arn:ctyun:iam:::user/test" }
    Action    = "oos:*"
    Resource  = "*"
  }]
})`

func testAccBucketPolicyConfig_crossAccountRead(bucket string) string {
	return fmt.Sprintf(`
data "ctyun_oos_bucket_policy" "test" {
  bucket = %[1]q
}
`, bucket)
}

func testAccBucketPolicyConfig_crossAccountWrite(bucket string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket_policy" "test" {
  bucket = %[1]q
  policy = %[2]s
}
`, bucket, testAccBucketPolicyValidPolicyExpression)
}

func testAccBucketPolicyConfig_missingPolicy(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_policy" "test" {
  bucket = ctyun_oos_bucket.test.bucket
}
`, rName)
}
func testAccBucketPolicyConfig_policyExpression(rName, policyExpression string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_policy" "test" {
  bucket = ctyun_oos_bucket.test.bucket
  policy = %[2]s
}
`, rName, policyExpression)
}

func testAccBucketPolicyConfig_policyOnlyExpression(bucket, policyExpression string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket_policy" "test" {
  bucket = %[1]q
  policy = %[2]s
}
`, bucket, policyExpression)
}
func testAccBucketPolicyConfig_policy(rName, policy string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q
}

resource "ctyun_oos_bucket_policy" "test" {
  bucket = ctyun_oos_bucket.test.bucket
  policy = <<POLICY
%[2]s
POLICY
}
`, rName, policy)
}
func testAccBucketPolicyConfig_basic(rName, action string) string {
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

func testAccBucketPolicyConfig_bucketOnly(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_oos_bucket" "test" {
  bucket = %[1]q

}
`, rName)
}
