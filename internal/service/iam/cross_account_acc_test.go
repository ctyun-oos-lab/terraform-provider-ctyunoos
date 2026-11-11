// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
)

const testAccCrossAccountIAMPolicyDocument = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": ["iam:GetUser"],
      "Resource": "*",
      "Effect": "Allow"
    }
  ]
}`

// Cross-account ACCs use a second set of static credentials to create resources in another account, destroyed via t.Cleanup at the end of the test.
// The test is Skipped when CTYUNOOS_ACCESS_KEY_SECOND / CTYUNOOS_SECRET_KEY_SECOND (and CTYUNOOS_ENDPOINT_IAM) are not set.
// The test is also Skipped when the second credential set is identical to the primary account, otherwise the negative cases cannot cover cross-account scenarios.

func testAccSecondAccountIAMClient(ctx context.Context, t *testing.T) *iam.Client {
	t.Helper()

	secondAccessKey := os.Getenv("CTYUNOOS_ACCESS_KEY_SECOND")
	secondSecretKey := os.Getenv("CTYUNOOS_SECRET_KEY_SECOND")
	endpoint := os.Getenv("CTYUNOOS_ENDPOINT_IAM")
	if secondAccessKey == "" || secondSecretKey == "" {
		t.Skip("skipping: CTYUNOOS_ACCESS_KEY_SECOND and CTYUNOOS_SECRET_KEY_SECOND must be configured")
	}
	if endpoint == "" {
		t.Skip("skipping: CTYUNOOS_ENDPOINT_IAM must be configured")
	}

	acctest.PreCheck(ctx, t)
	conn, err := tfiam.IAMClientWithStaticCredentials(ctx, acctest.ProviderMeta(ctx, t), endpoint, secondAccessKey, secondSecretKey)
	if err != nil {
		t.Fatalf("creating IAM client with second-account credentials: %s", err)
	}
	return conn
}

func testAccMustDifferentAccountARN(t *testing.T, resourceARN string) {
	t.Helper()

	parsed, err := arn.Parse(resourceARN)
	if err != nil {
		t.Fatalf("parsing cross-account ARN %q: %s", resourceARN, err)
	}
	primary := acctest.AccountID(acctest.Context(t))
	if parsed.AccountID == "" {
		t.Fatalf("cross-account ARN %q has empty account ID", resourceARN)
	}
	if primary != "" && parsed.AccountID == primary {
		t.Skipf("skipping: CTYUNOOS_ACCESS_KEY_SECOND is the same account (%s) as the primary credentials", primary)
	}
}

func testAccIgnoreIAMMissing(err error) error {
	if err == nil {
		return nil
	}
	if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
		return nil
	}
	return err
}

func testAccCleanupIAMErr(t *testing.T, kind, id string, err error) {
	t.Helper()
	if err := testAccIgnoreIAMMissing(err); err != nil {
		t.Errorf("deleting cross-account IAM %s (%s): %s", kind, id, err)
	}
}

func testAccCrossAccountPolicyARN(t *testing.T) string {
	t.Helper()

	ctx := acctest.Context(t)
	conn := testAccSecondAccountIAMClient(ctx, t)
	name := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	out, err := conn.CreatePolicy(ctx, &iam.CreatePolicyInput{
		Description:    aws.String("cross-account ACC policy"),
		PolicyDocument: aws.String(testAccCrossAccountIAMPolicyDocument),
		PolicyName:     aws.String(name),
	})
	if err != nil {
		t.Fatalf("creating cross-account IAM Policy (%s): %s", name, err)
	}
	if out == nil || out.Policy == nil || aws.ToString(out.Policy.Arn) == "" {
		t.Fatalf("creating cross-account IAM Policy (%s): empty ARN", name)
	}
	policyARN := aws.ToString(out.Policy.Arn)
	t.Cleanup(func() {
		_, err := conn.DeletePolicy(context.Background(), &iam.DeletePolicyInput{
			PolicyArn: aws.String(policyARN),
		})
		testAccCleanupIAMErr(t, "Policy", policyARN, err)
	})
	testAccMustDifferentAccountARN(t, policyARN)

	return policyARN
}

func testAccCrossAccountUserName(t *testing.T) string {
	t.Helper()

	ctx := acctest.Context(t)
	conn := testAccSecondAccountIAMClient(ctx, t)
	name := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	out, err := conn.CreateUser(ctx, &iam.CreateUserInput{
		UserName: aws.String(name),
	})
	if err != nil {
		t.Fatalf("creating cross-account IAM User (%s): %s", name, err)
	}
	t.Cleanup(func() {
		_, err := conn.DeleteUser(context.Background(), &iam.DeleteUserInput{
			UserName: aws.String(name),
		})
		testAccCleanupIAMErr(t, "User", name, err)
	})

	userARN := ""
	if out != nil && out.User != nil {
		userARN = aws.ToString(out.User.Arn)
	}
	if userARN == "" {
		got, getErr := conn.GetUser(ctx, &iam.GetUserInput{UserName: aws.String(name)})
		if getErr != nil || got == nil || got.User == nil {
			t.Fatalf("creating cross-account IAM User (%s): empty ARN", name)
		}
		userARN = aws.ToString(got.User.Arn)
	}
	testAccMustDifferentAccountARN(t, userARN)

	return name
}

func testAccCrossAccountGroupName(t *testing.T) string {
	t.Helper()

	ctx := acctest.Context(t)
	conn := testAccSecondAccountIAMClient(ctx, t)
	name := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	out, err := conn.CreateGroup(ctx, &iam.CreateGroupInput{
		GroupName: aws.String(name),
	})
	if err != nil {
		t.Fatalf("creating cross-account IAM Group (%s): %s", name, err)
	}
	t.Cleanup(func() {
		_, err := conn.DeleteGroup(context.Background(), &iam.DeleteGroupInput{
			GroupName: aws.String(name),
		})
		testAccCleanupIAMErr(t, "Group", name, err)
	})

	groupARN := ""
	if out != nil && out.Group != nil {
		groupARN = aws.ToString(out.Group.Arn)
	}
	if groupARN == "" {
		t.Fatalf("creating cross-account IAM Group (%s): empty ARN", name)
	}
	testAccMustDifferentAccountARN(t, groupARN)

	return name
}

func testAccCrossAccountRoleName(t *testing.T) string {
	t.Helper()

	ctx := acctest.Context(t)
	conn := testAccSecondAccountIAMClient(ctx, t)
	name := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	out, err := conn.CreateRole(ctx, &iam.CreateRoleInput{
		RoleName:                 aws.String(name),
		AssumeRolePolicyDocument: aws.String(testAccRoleAssumePolicy("ec2.amazonaws.com")),
		Description:              aws.String("cross-account ACC role"),
		MaxSessionDuration:       aws.Int32(3600),
	})
	if err != nil {
		t.Fatalf("creating cross-account IAM Role (%s): %s", name, err)
	}
	t.Cleanup(func() {
		_, err := conn.DeleteRole(context.Background(), &iam.DeleteRoleInput{
			RoleName: aws.String(name),
		})
		testAccCleanupIAMErr(t, "Role", name, err)
	})

	roleARN := ""
	if out != nil && out.Role != nil {
		roleARN = aws.ToString(out.Role.Arn)
	}
	if roleARN == "" {
		t.Fatalf("creating cross-account IAM Role (%s): empty ARN", name)
	}
	testAccMustDifferentAccountARN(t, roleARN)

	return name
}

func testAccCrossAccountSAMLProviderARN(t *testing.T) string {
	t.Helper()

	ctx := acctest.Context(t)
	conn := testAccSecondAccountIAMClient(ctx, t)
	name := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	metadata := testAccSAMLProviderMetadataDocument(t, fmt.Sprintf("https://example.com/%s", name))

	out, err := conn.CreateSAMLProvider(ctx, &iam.CreateSAMLProviderInput{
		Name:                 aws.String(name),
		SAMLMetadataDocument: aws.String(metadata),
	})
	if err != nil {
		t.Fatalf("creating cross-account IAM SAML Provider (%s): %s", name, err)
	}
	if out == nil || aws.ToString(out.SAMLProviderArn) == "" {
		t.Fatalf("creating cross-account IAM SAML Provider (%s): empty ARN", name)
	}
	providerARN := aws.ToString(out.SAMLProviderArn)
	t.Cleanup(func() {
		_, err := conn.DeleteSAMLProvider(context.Background(), &iam.DeleteSAMLProviderInput{
			SAMLProviderArn: aws.String(providerARN),
		})
		testAccCleanupIAMErr(t, "SAML Provider", providerARN, err)
	})
	testAccMustDifferentAccountARN(t, providerARN)

	return providerARN
}
