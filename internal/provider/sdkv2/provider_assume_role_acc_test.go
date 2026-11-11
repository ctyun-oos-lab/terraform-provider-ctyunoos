// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package sdkv2_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TC-OOS-PROVIDER-12 Single assume_role: creates a source IAM user and access key, then assumes a role with that user's credentials.
// Expected: after obtaining role temporary credentials, the provider creates and reads an S3 bucket.
func TestAccOOSProvider_assumeRoleSingle(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)
	fixture := testAccCreateOOSAssumeRoleFixture(ctx, t)
	// Explicit source-user AK/SK values must not inherit an unrelated session token from the environment.
	t.Setenv("CTYUNOOS_SESSION_TOKEN", "invalid-env-token-that-must-not-mix")
	bucketName := acctest.RandomWithPrefix(t, "tf-acc-test-provider-assume-role")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{{
			Config: testAccOOSProviderConfigAssumeRoleSingle(
				os.Getenv("CTYUNOOS_ENDPOINT"), os.Getenv("CTYUNOOS_ENDPOINT_IAM"), fixture.roleARN,
				fixture.accessKeyID, fixture.secretAccessKey, bucketName,
			),
			Check: resource.TestCheckResourceAttr("ctyun_oos_bucket.test", names.AttrBucket, bucketName),
		}},
	})
}

// TC-OOS-PROVIDER-13 Chained assume_role: the source user first assumes role A, then role A assumes role B.
// Expected: only role B has OOS bucket permissions; creating and reading the bucket proves that the provider ultimately uses role B temporary credentials.
func TestAccOOSProvider_assumeRoleChain(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)
	fixture := testAccCreateOOSAssumeRoleChainFixture(ctx, t)
	t.Setenv("CTYUNOOS_SESSION_TOKEN", "invalid-env-token-that-must-not-mix")
	bucketName := acctest.RandomWithPrefix(t, "tf-acc-test-provider-assume-role-chain")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{{
			Config: testAccOOSProviderConfigAssumeRoleChain(
				os.Getenv("CTYUNOOS_ENDPOINT"), os.Getenv("CTYUNOOS_ENDPOINT_IAM"),
				fixture.roleAARN, fixture.roleBARN, fixture.accessKeyID, fixture.secretAccessKey, bucketName,
			),
			Check: resource.TestCheckResourceAttr("ctyun_oos_bucket.test", names.AttrBucket, bucketName),
		}},
	})
}

// TC-OOS-PROVIDER-14 A single empty assume_role block: skips AssumeRole and continues using the original credentials.
// Expected: provider configuration succeeds and reads the account summary with the original credentials.
func TestAccOOSProvider_assumeRoleEmpty(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)
	fixture := testAccCreateOOSAssumeRoleEmptyFixture(ctx, t)
	// The Terraform subprocess can obtain only this user's original AK/SK values. The user is granted only
	// GetAccountSummary permission, so a successful response proves that the empty assume_role did not replace the credentials.
	t.Setenv("CTYUNOOS_ACCESS_KEY", fixture.accessKeyID)
	t.Setenv("CTYUNOOS_SECRET_KEY", fixture.secretAccessKey)
	t.Setenv("CTYUNOOS_SESSION_TOKEN", "")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOOSProviderConfigIAMEmptyAssumeRole(os.Getenv("CTYUNOOS_ENDPOINT_IAM")),
				Check:  resource.TestCheckResourceAttr("data.ctyun_iam_account_summary.test", names.AttrID, "iam_account_summary"),
			},
			{
				Config: testAccOOSProviderConfigIAMEmptyAssumeRoleString(os.Getenv("CTYUNOOS_ENDPOINT_IAM")),
				Check:  resource.TestCheckResourceAttr("data.ctyun_iam_account_summary.test", names.AttrID, "iam_account_summary"),
			},
		},
	})
}

// TC-OOS-PROVIDER-35 AssumeRole With All Optional Fields: temporary credentials with complete optional arguments can create and read back an S3 bucket.
func TestAccOOSProvider_assumeRoleAllOptionalFields(t *testing.T) {
	ctx := acctest.Context(t)
	acctest.PreCheck(ctx, t)
	fixture := testAccCreateOOSAssumeRoleFixture(ctx, t)
	// Explicit source-user AK/SK values must not inherit an unrelated session token from the environment.
	t.Setenv("CTYUNOOS_SESSION_TOKEN", "invalid-env-token-that-must-not-mix")
	bucketName := acctest.RandomWithPrefix(t, "tf-acc-test-provider-assume-role-full")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{{
			Config: testAccOOSProviderConfigAssumeRoleAllOptionalFields(
				fixture.roleARN, fixture.accessKeyID, fixture.secretAccessKey,
				os.Getenv("CTYUNOOS_ENDPOINT"), os.Getenv("CTYUNOOS_ENDPOINT_IAM"), bucketName,
			),
			Check: resource.TestCheckResourceAttr("ctyun_oos_bucket.test", names.AttrBucket, bucketName),
		}},
	})
}

type oosAssumeRoleSourceUserFixture struct {
	conn            *iam.Client
	userName        string
	roleName        string
	roleARN         string
	accessKeyID     string
	secretAccessKey string
	userPolicyARN   string
	rolePolicyARN   string
}

type oosAssumeRoleChainFixture struct {
	conn            *iam.Client
	userName        string
	roleAName       string
	roleBName       string
	roleAARN        string
	roleBARN        string
	accessKeyID     string
	secretAccessKey string
	userPolicyARN   string
	roleAPolicyARN  string
	roleBPolicyARN  string
}

type oosAssumeRoleEmptyFixture struct {
	conn            *iam.Client
	userName        string
	accessKeyID     string
	secretAccessKey string
	policyARN       string
	policyAttached  bool
}

func testAccCreateOOSAssumeRoleFixture(ctx context.Context, t *testing.T) *oosAssumeRoleSourceUserFixture {
	t.Helper()
	fixture := &oosAssumeRoleSourceUserFixture{
		conn:     acctest.ProviderMeta(ctx, t).IAMClient(ctx),
		userName: acctest.RandomWithPrefix(t, "tf-acc-test-assume-role-source"),
		roleName: acctest.RandomWithPrefix(t, "tf-acc-test-assume-role"),
	}
	t.Cleanup(func() {
		if err := testAccDestroyOOSAssumeRoleSourceUserFixture(context.Background(), fixture); err != nil {
			t.Errorf("destroying AssumeRole source IAM user fixture: %s", err)
		}
	})

	user, err := fixture.conn.CreateUser(ctx, &iam.CreateUserInput{UserName: aws.String(fixture.userName)})
	if err != nil || user == nil || user.User == nil || aws.ToString(user.User.Arn) == "" {
		t.Fatalf("creating AssumeRole source IAM user: %s", err)
	}
	trustPolicy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"CTYUN":[%q]},"Action":["iam:AssumeRole"]}]}`, aws.ToString(user.User.Arn))
	role, err := fixture.conn.CreateRole(ctx, &iam.CreateRoleInput{
		RoleName:                 aws.String(fixture.roleName),
		AssumeRolePolicyDocument: aws.String(trustPolicy),
	})
	if err != nil || role == nil || role.Role == nil || aws.ToString(role.Role.Arn) == "" {
		t.Fatalf("creating AssumeRole target IAM role: %v", err)
	}
	fixture.roleARN = aws.ToString(role.Role.Arn)

	rolePolicy, err := fixture.conn.CreatePolicy(ctx, &iam.CreatePolicyInput{
		PolicyName: aws.String(acctest.RandomWithPrefix(t, "tf-acc-test-assume-role-s3-policy")),
		// The bucket resource performs Put/Get/Delete calls and several readback
		// operations. This fixture isolates provider AssumeRole behavior, so grant
		// the temporary role OOS service permissions rather than AWS S3 actions.
		PolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"oos:*","Resource":"*"}]}`),
	})
	if err != nil || rolePolicy == nil || rolePolicy.Policy == nil || aws.ToString(rolePolicy.Policy.Arn) == "" {
		t.Fatalf("creating AssumeRole target policy: %v", err)
	}
	fixture.rolePolicyARN = aws.ToString(rolePolicy.Policy.Arn)
	if _, err := fixture.conn.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{RoleName: aws.String(fixture.roleName), PolicyArn: aws.String(fixture.rolePolicyARN)}); err != nil {
		t.Fatalf("attaching AssumeRole target policy: %s", err)
	}

	userPolicy, err := fixture.conn.CreatePolicy(ctx, &iam.CreatePolicyInput{
		PolicyName:     aws.String(acctest.RandomWithPrefix(t, "tf-acc-test-assume-role-policy")),
		PolicyDocument: aws.String(fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"iam:AssumeRole","Resource":%q}]}`, fixture.roleARN)),
	})
	if err != nil || userPolicy == nil || userPolicy.Policy == nil || aws.ToString(userPolicy.Policy.Arn) == "" {
		t.Fatalf("creating AssumeRole source IAM policy: %v", err)
	}
	fixture.userPolicyARN = aws.ToString(userPolicy.Policy.Arn)
	if _, err := fixture.conn.AttachUserPolicy(ctx, &iam.AttachUserPolicyInput{UserName: aws.String(fixture.userName), PolicyArn: aws.String(fixture.userPolicyARN)}); err != nil {
		t.Fatalf("attaching AssumeRole source IAM policy: %s", err)
	}
	accessKey, err := fixture.conn.CreateAccessKey(ctx, &iam.CreateAccessKeyInput{UserName: aws.String(fixture.userName)})
	if err != nil || accessKey == nil || accessKey.AccessKey == nil {
		t.Fatalf("creating AssumeRole source access key: %v", err)
	}
	fixture.accessKeyID = aws.ToString(accessKey.AccessKey.AccessKeyId)
	fixture.secretAccessKey = aws.ToString(accessKey.AccessKey.SecretAccessKey)
	return fixture
}

func testAccCreateOOSAssumeRoleChainFixture(ctx context.Context, t *testing.T) *oosAssumeRoleChainFixture {
	t.Helper()
	fixture := &oosAssumeRoleChainFixture{
		conn:      acctest.ProviderMeta(ctx, t).IAMClient(ctx),
		userName:  acctest.RandomWithPrefix(t, "tf-acc-test-assume-role-chain-source"),
		roleAName: acctest.RandomWithPrefix(t, "tf-acc-test-assume-role-chain-a"),
		roleBName: acctest.RandomWithPrefix(t, "tf-acc-test-assume-role-chain-b"),
	}
	t.Cleanup(func() {
		if err := testAccDestroyOOSAssumeRoleChainFixture(context.Background(), fixture); err != nil {
			t.Errorf("destroying AssumeRole chain fixture: %s", err)
		}
	})

	user, err := fixture.conn.CreateUser(ctx, &iam.CreateUserInput{UserName: aws.String(fixture.userName)})
	if err != nil || user == nil || user.User == nil || aws.ToString(user.User.Arn) == "" {
		t.Fatalf("creating AssumeRole chain source IAM user: %s", err)
	}

	roleA, err := fixture.conn.CreateRole(ctx, &iam.CreateRoleInput{
		RoleName: aws.String(fixture.roleAName),
		AssumeRolePolicyDocument: aws.String(fmt.Sprintf(
			`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"CTYUN":[%q]},"Action":["iam:AssumeRole"]}]}`,
			aws.ToString(user.User.Arn),
		)),
	})
	if err != nil || roleA == nil || roleA.Role == nil || aws.ToString(roleA.Role.Arn) == "" {
		t.Fatalf("creating AssumeRole chain role A: %v", err)
	}
	fixture.roleAARN = aws.ToString(roleA.Role.Arn)

	roleB, err := fixture.conn.CreateRole(ctx, &iam.CreateRoleInput{
		RoleName: aws.String(fixture.roleBName),
		AssumeRolePolicyDocument: aws.String(fmt.Sprintf(
			`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"CTYUN":[%q]},"Action":["iam:AssumeRole"]}]}`,
			fixture.roleAARN,
		)),
	})
	if err != nil || roleB == nil || roleB.Role == nil || aws.ToString(roleB.Role.Arn) == "" {
		t.Fatalf("creating AssumeRole chain role B: %v", err)
	}
	fixture.roleBARN = aws.ToString(roleB.Role.Arn)

	userPolicy, err := fixture.conn.CreatePolicy(ctx, &iam.CreatePolicyInput{
		PolicyName:     aws.String(acctest.RandomWithPrefix(t, "tf-acc-test-assume-role-chain-source-policy")),
		PolicyDocument: aws.String(fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"iam:AssumeRole","Resource":%q}]}`, fixture.roleAARN)),
	})
	if err != nil || userPolicy == nil || userPolicy.Policy == nil || aws.ToString(userPolicy.Policy.Arn) == "" {
		t.Fatalf("creating AssumeRole chain source policy: %v", err)
	}
	fixture.userPolicyARN = aws.ToString(userPolicy.Policy.Arn)
	if _, err := fixture.conn.AttachUserPolicy(ctx, &iam.AttachUserPolicyInput{UserName: aws.String(fixture.userName), PolicyArn: aws.String(fixture.userPolicyARN)}); err != nil {
		t.Fatalf("attaching AssumeRole chain source policy: %s", err)
	}

	roleAPolicy, err := fixture.conn.CreatePolicy(ctx, &iam.CreatePolicyInput{
		PolicyName:     aws.String(acctest.RandomWithPrefix(t, "tf-acc-test-assume-role-chain-a-policy")),
		PolicyDocument: aws.String(fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"iam:AssumeRole","Resource":%q}]}`, fixture.roleBARN)),
	})
	if err != nil || roleAPolicy == nil || roleAPolicy.Policy == nil || aws.ToString(roleAPolicy.Policy.Arn) == "" {
		t.Fatalf("creating AssumeRole chain role A policy: %v", err)
	}
	fixture.roleAPolicyARN = aws.ToString(roleAPolicy.Policy.Arn)
	if _, err := fixture.conn.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{RoleName: aws.String(fixture.roleAName), PolicyArn: aws.String(fixture.roleAPolicyARN)}); err != nil {
		t.Fatalf("attaching AssumeRole chain role A policy: %s", err)
	}

	roleBPolicy, err := fixture.conn.CreatePolicy(ctx, &iam.CreatePolicyInput{
		PolicyName:     aws.String(acctest.RandomWithPrefix(t, "tf-acc-test-assume-role-chain-b-policy")),
		PolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"oos:*","Resource":"*"}]}`),
	})
	if err != nil || roleBPolicy == nil || roleBPolicy.Policy == nil || aws.ToString(roleBPolicy.Policy.Arn) == "" {
		t.Fatalf("creating AssumeRole chain role B policy: %v", err)
	}
	fixture.roleBPolicyARN = aws.ToString(roleBPolicy.Policy.Arn)
	if _, err := fixture.conn.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{RoleName: aws.String(fixture.roleBName), PolicyArn: aws.String(fixture.roleBPolicyARN)}); err != nil {
		t.Fatalf("attaching AssumeRole chain role B policy: %s", err)
	}

	accessKey, err := fixture.conn.CreateAccessKey(ctx, &iam.CreateAccessKeyInput{UserName: aws.String(fixture.userName)})
	if err != nil || accessKey == nil || accessKey.AccessKey == nil {
		t.Fatalf("creating AssumeRole chain source access key: %v", err)
	}
	fixture.accessKeyID = aws.ToString(accessKey.AccessKey.AccessKeyId)
	fixture.secretAccessKey = aws.ToString(accessKey.AccessKey.SecretAccessKey)
	return fixture
}

func testAccCreateOOSAssumeRoleEmptyFixture(ctx context.Context, t *testing.T) *oosAssumeRoleEmptyFixture {
	t.Helper()
	fixture := &oosAssumeRoleEmptyFixture{
		conn:     acctest.ProviderMeta(ctx, t).IAMClient(ctx),
		userName: acctest.RandomWithPrefix(t, "tf-acc-test-empty-assume-role"),
	}
	t.Cleanup(func() {
		if err := testAccDestroyOOSAssumeRoleEmptyFixture(context.Background(), fixture); err != nil {
			t.Errorf("destroying empty AssumeRole fixture: %s", err)
		}
	})

	if _, err := fixture.conn.CreateUser(ctx, &iam.CreateUserInput{UserName: aws.String(fixture.userName)}); err != nil {
		t.Fatalf("creating empty AssumeRole source IAM user: %s", err)
	}
	policy, err := fixture.conn.CreatePolicy(ctx, &iam.CreatePolicyInput{
		PolicyName:     aws.String(acctest.RandomWithPrefix(t, "tf-acc-test-empty-assume-role-policy")),
		PolicyDocument: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"iam:GetAccountSummary","Resource":"*"}]}`),
	})
	if err != nil || policy == nil || policy.Policy == nil || aws.ToString(policy.Policy.Arn) == "" {
		t.Fatalf("creating empty AssumeRole source policy: %v", err)
	}
	fixture.policyARN = aws.ToString(policy.Policy.Arn)
	if _, err := fixture.conn.AttachUserPolicy(ctx, &iam.AttachUserPolicyInput{
		UserName:  aws.String(fixture.userName),
		PolicyArn: aws.String(fixture.policyARN),
	}); err != nil {
		t.Fatalf("attaching empty AssumeRole source policy: %s", err)
	}
	fixture.policyAttached = true

	accessKey, err := fixture.conn.CreateAccessKey(ctx, &iam.CreateAccessKeyInput{UserName: aws.String(fixture.userName)})
	if err != nil || accessKey == nil || accessKey.AccessKey == nil {
		t.Fatalf("creating empty AssumeRole source access key: %v", err)
	}
	fixture.accessKeyID = aws.ToString(accessKey.AccessKey.AccessKeyId)
	fixture.secretAccessKey = aws.ToString(accessKey.AccessKey.SecretAccessKey)
	return fixture
}

func testAccDestroyOOSAssumeRoleSourceUserFixture(ctx context.Context, fixture *oosAssumeRoleSourceUserFixture) error {
	if fixture == nil || fixture.conn == nil {
		return nil
	}
	if fixture.accessKeyID != "" {
		if _, err := fixture.conn.UpdateAccessKey(ctx, &iam.UpdateAccessKeyInput{UserName: aws.String(fixture.userName), AccessKeyId: aws.String(fixture.accessKeyID), Status: awstypes.StatusTypeInactive}); err != nil {
			return fmt.Errorf("deactivating access key: %w", err)
		}
		if _, err := fixture.conn.DeleteAccessKey(ctx, &iam.DeleteAccessKeyInput{UserName: aws.String(fixture.userName), AccessKeyId: aws.String(fixture.accessKeyID)}); err != nil {
			return fmt.Errorf("deleting access key: %w", err)
		}
	}
	if fixture.userPolicyARN != "" {
		if _, err := fixture.conn.DetachUserPolicy(ctx, &iam.DetachUserPolicyInput{UserName: aws.String(fixture.userName), PolicyArn: aws.String(fixture.userPolicyARN)}); err != nil {
			return fmt.Errorf("detaching policy: %w", err)
		}
		if _, err := fixture.conn.DeletePolicy(ctx, &iam.DeletePolicyInput{PolicyArn: aws.String(fixture.userPolicyARN)}); err != nil {
			return fmt.Errorf("deleting policy: %w", err)
		}
	}
	if fixture.rolePolicyARN != "" {
		if _, err := fixture.conn.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{RoleName: aws.String(fixture.roleName), PolicyArn: aws.String(fixture.rolePolicyARN)}); err != nil {
			return fmt.Errorf("detaching role policy: %w", err)
		}
		if _, err := fixture.conn.DeletePolicy(ctx, &iam.DeletePolicyInput{PolicyArn: aws.String(fixture.rolePolicyARN)}); err != nil {
			return fmt.Errorf("deleting role policy: %w", err)
		}
	}
	if fixture.roleName != "" {
		if _, err := fixture.conn.DeleteRole(ctx, &iam.DeleteRoleInput{RoleName: aws.String(fixture.roleName)}); err != nil {
			return fmt.Errorf("deleting role: %w", err)
		}
	}
	if fixture.userName != "" {
		if _, err := fixture.conn.DeleteUser(ctx, &iam.DeleteUserInput{UserName: aws.String(fixture.userName)}); err != nil {
			return fmt.Errorf("deleting user: %w", err)
		}
	}
	return nil
}

func testAccDestroyOOSAssumeRoleChainFixture(ctx context.Context, fixture *oosAssumeRoleChainFixture) error {
	if fixture == nil || fixture.conn == nil {
		return nil
	}
	if fixture.accessKeyID != "" {
		if _, err := fixture.conn.UpdateAccessKey(ctx, &iam.UpdateAccessKeyInput{UserName: aws.String(fixture.userName), AccessKeyId: aws.String(fixture.accessKeyID), Status: awstypes.StatusTypeInactive}); err != nil {
			return fmt.Errorf("deactivating source access key: %w", err)
		}
		if _, err := fixture.conn.DeleteAccessKey(ctx, &iam.DeleteAccessKeyInput{UserName: aws.String(fixture.userName), AccessKeyId: aws.String(fixture.accessKeyID)}); err != nil {
			return fmt.Errorf("deleting source access key: %w", err)
		}
	}
	if fixture.userPolicyARN != "" {
		if _, err := fixture.conn.DetachUserPolicy(ctx, &iam.DetachUserPolicyInput{UserName: aws.String(fixture.userName), PolicyArn: aws.String(fixture.userPolicyARN)}); err != nil {
			return fmt.Errorf("detaching source policy: %w", err)
		}
		if _, err := fixture.conn.DeletePolicy(ctx, &iam.DeletePolicyInput{PolicyArn: aws.String(fixture.userPolicyARN)}); err != nil {
			return fmt.Errorf("deleting source policy: %w", err)
		}
	}
	if fixture.roleAPolicyARN != "" {
		if _, err := fixture.conn.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{RoleName: aws.String(fixture.roleAName), PolicyArn: aws.String(fixture.roleAPolicyARN)}); err != nil {
			return fmt.Errorf("detaching role A policy: %w", err)
		}
		if _, err := fixture.conn.DeletePolicy(ctx, &iam.DeletePolicyInput{PolicyArn: aws.String(fixture.roleAPolicyARN)}); err != nil {
			return fmt.Errorf("deleting role A policy: %w", err)
		}
	}
	if fixture.roleBPolicyARN != "" {
		if _, err := fixture.conn.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{RoleName: aws.String(fixture.roleBName), PolicyArn: aws.String(fixture.roleBPolicyARN)}); err != nil {
			return fmt.Errorf("detaching role B policy: %w", err)
		}
		if _, err := fixture.conn.DeletePolicy(ctx, &iam.DeletePolicyInput{PolicyArn: aws.String(fixture.roleBPolicyARN)}); err != nil {
			return fmt.Errorf("deleting role B policy: %w", err)
		}
	}
	if fixture.roleBName != "" {
		if _, err := fixture.conn.DeleteRole(ctx, &iam.DeleteRoleInput{RoleName: aws.String(fixture.roleBName)}); err != nil {
			return fmt.Errorf("deleting role B: %w", err)
		}
	}
	if fixture.roleAName != "" {
		if _, err := fixture.conn.DeleteRole(ctx, &iam.DeleteRoleInput{RoleName: aws.String(fixture.roleAName)}); err != nil {
			return fmt.Errorf("deleting role A: %w", err)
		}
	}
	if fixture.userName != "" {
		if _, err := fixture.conn.DeleteUser(ctx, &iam.DeleteUserInput{UserName: aws.String(fixture.userName)}); err != nil {
			return fmt.Errorf("deleting source user: %w", err)
		}
	}
	return nil
}

func testAccDestroyOOSAssumeRoleEmptyFixture(ctx context.Context, fixture *oosAssumeRoleEmptyFixture) error {
	if fixture == nil || fixture.conn == nil {
		return nil
	}
	if fixture.accessKeyID != "" {
		if _, err := fixture.conn.UpdateAccessKey(ctx, &iam.UpdateAccessKeyInput{
			UserName:    aws.String(fixture.userName),
			AccessKeyId: aws.String(fixture.accessKeyID),
			Status:      awstypes.StatusTypeInactive,
		}); err != nil {
			return fmt.Errorf("deactivating empty AssumeRole source access key: %w", err)
		}
		if _, err := fixture.conn.DeleteAccessKey(ctx, &iam.DeleteAccessKeyInput{
			UserName:    aws.String(fixture.userName),
			AccessKeyId: aws.String(fixture.accessKeyID),
		}); err != nil {
			return fmt.Errorf("deleting empty AssumeRole source access key: %w", err)
		}
	}
	if fixture.policyAttached {
		if _, err := fixture.conn.DetachUserPolicy(ctx, &iam.DetachUserPolicyInput{
			UserName:  aws.String(fixture.userName),
			PolicyArn: aws.String(fixture.policyARN),
		}); err != nil {
			return fmt.Errorf("detaching empty AssumeRole source policy: %w", err)
		}
	}
	if fixture.policyARN != "" {
		if _, err := fixture.conn.DeletePolicy(ctx, &iam.DeletePolicyInput{PolicyArn: aws.String(fixture.policyARN)}); err != nil {
			return fmt.Errorf("deleting empty AssumeRole source policy: %w", err)
		}
	}
	if fixture.userName != "" {
		if _, err := fixture.conn.DeleteUser(ctx, &iam.DeleteUserInput{UserName: aws.String(fixture.userName)}); err != nil {
			return fmt.Errorf("deleting empty AssumeRole source user: %w", err)
		}
	}
	return nil
}

func testAccOOSProviderConfigAssumeRoleSingle(s3Endpoint, iamEndpoint, roleARN, accessKey, secretKey, bucketName string) string {
	return fmt.Sprintf(`
provider "ctyun" {
  access_key = %q
  secret_key = %q
  assume_role {
    role_arn     = %q
    session_name = "tf-acc-provider-assume-role"
  }
  endpoints {
    s3  = %q
    iam = %q
  }
}
resource "ctyun_oos_bucket" "test" {
  bucket = %q
}
`, accessKey, secretKey, roleARN, s3Endpoint, iamEndpoint, bucketName)
}

func testAccOOSProviderConfigAssumeRoleChain(s3Endpoint, iamEndpoint, roleAARN, roleBARN, accessKey, secretKey, bucketName string) string {
	return fmt.Sprintf(`
provider "ctyun" {
  access_key = %q
  secret_key = %q
  assume_role {
    role_arn     = %q
    session_name = "tf-acc-provider-assume-role-a"
  }
  assume_role {
    role_arn     = %q
    session_name = "tf-acc-provider-assume-role-b"
  }
  endpoints {
    s3  = %q
    iam = %q
  }
}
resource "ctyun_oos_bucket" "test" {
  bucket = %q
}
`, accessKey, secretKey, roleAARN, roleBARN, s3Endpoint, iamEndpoint, bucketName)
}

func testAccOOSProviderConfigAssumeRoleAllOptionalFields(roleARN, accessKey, secretKey, s3Endpoint, iamEndpoint, bucketName string) string {
	return fmt.Sprintf(`
provider "ctyun" {
  access_key = %q
  secret_key = %q
  assume_role {
    role_arn        = %q
    session_name    = "tf-acc-provider-assume-role-full"
    duration        = "1h"
    external_id     = "oos-test-external-id"
    source_identity = "oos-test-source"
    policy = jsonencode({
      Version = "2012-10-17"
      Statement = [{
        Effect   = "Allow"
        Action   = ["oos:*"]
        Resource = "*"
      }]
    })
  }
  endpoints {
    s3  = %q
    iam = %q
  }
}
resource "ctyun_oos_bucket" "test" {
  bucket = %q
}
`, accessKey, secretKey, roleARN, s3Endpoint, iamEndpoint, bucketName)
}

func testAccOOSProviderConfigIAMEmptyAssumeRole(endpoint string) string {
	return fmt.Sprintf(`
provider "ctyun" {
  assume_role {}
  endpoints {
    iam = %q
  }
}

data "ctyun_iam_account_summary" "test" {}
`, endpoint)
}

func testAccOOSProviderConfigIAMEmptyAssumeRoleString(endpoint string) string {
	return fmt.Sprintf(`
provider "ctyun" {
  assume_role {
    role_arn = ""
  }
  endpoints {
    iam = %q
  }
}

data "ctyun_iam_account_summary" "test" {}
`, endpoint)
}
