// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/vault/helper/pgpkeys"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	sdkretry "github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TC-IAM-AK-01-Basic Create verifies that an access key without an explicit status completes basic creation, reading, state population, and import.
func TestAccIAMAccessKey_basic(t *testing.T) {
	ctx := acctest.Context(t)
	var conf awstypes.AccessKeyMetadata
	resourceName := "ctyun_iam_access_key.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix+"-ak")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccessKeyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccAccessKeyConfig_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeyExists(ctx, t, resourceName, &conf),
					testAccCheckAccessKeyState(resourceName, &conf, rName, string(awstypes.StatusTypeActive)),
					resource.TestCheckResourceAttrSet(resourceName, "secret"),
					resource.TestCheckNoResourceAttr(resourceName, "encrypted_secret"),
					resource.TestCheckNoResourceAttr(resourceName, "key_fingerprint"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"encrypted_secret", "key_fingerprint", "last_used_date", "pgp_key", "secret", "service_name"},
			},
		},
	})
}

// TC-IAM-AK-01 Basic Create: concurrently creates two independent access keys for one user and verifies creation and destruction under the resource-level mutex.
func TestAccIAMAccessKey_concurrentCreate(t *testing.T) {
	ctx := acctest.Context(t)
	const accessKeyCount = 2

	accessKeys := make([]awstypes.AccessKeyMetadata, accessKeyCount)
	resourceNames := make([]string, accessKeyCount)
	checks := make([]resource.TestCheckFunc, 0, accessKeyCount*3+1)
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix+"-ak")

	for i := range accessKeys {
		resourceName := fmt.Sprintf("ctyun_iam_access_key.test.%d", i)
		resourceNames[i] = resourceName
		checks = append(checks,
			testAccCheckAccessKeyExists(ctx, t, resourceName, &accessKeys[i]),
			testAccCheckAccessKeyState(resourceName, &accessKeys[i], userName, string(awstypes.StatusTypeActive)),
			resource.TestCheckResourceAttrSet(resourceName, "secret"),
		)
	}
	checks = append(checks, testAccCheckAccessKeyIDsDistinct(resourceNames))

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccessKeyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccAccessKeyConfig_concurrentCreate(userName, accessKeyCount),
				Check:  resource.ComposeTestCheckFunc(checks...),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TC-IAM-AK-01-Basic Create verifies that creating an access key with pgp_key and no explicit status produces encrypted_secret and key_fingerprint; the decrypted SecretAccessKey works with AccessKeyId for GetUser, and refresh populates last-used attributes.
func TestAccIAMAccessKey_basicWithPGPKey(t *testing.T) {
	ctx := acctest.Context(t)
	var conf awstypes.AccessKeyMetadata
	resourceName := "ctyun_iam_access_key.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix+"-ak")
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix+"-ak-policy")
	pgpPublicKey, pgpPrivateKey := testAccPGPKeyPair(t)
	config := testAccAccessKeyConfig_pgpKey(rName, policyName, pgpPublicKey)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccessKeyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeyExists(ctx, t, resourceName, &conf),
					testAccCheckAccessKeyState(resourceName, &conf, rName, string(awstypes.StatusTypeActive)),
					resource.TestCheckNoResourceAttr(resourceName, "secret"),
					resource.TestCheckResourceAttrSet(resourceName, "encrypted_secret"),
					resource.TestCheckResourceAttrSet(resourceName, "key_fingerprint"),
					testAccCheckAccessKeyLastUsedEmpty(resourceName),
					testAccCheckAccessKeyDecryptAndUse(ctx, t, resourceName, pgpPrivateKey),
				),
			},
			{
				RefreshState: true,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeyLastUsedPopulated(ctx, t, resourceName),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"encrypted_secret", "key_fingerprint", "last_used_date", "pgp_key", "secret", "service_name"},
			},
		},
	})
}

// TC-IAM-AK-01 Basic Create: changing pgp_key makes Terraform replace the access key, delete the old key, and produce ciphertext decryptable by the new private key.
func TestAccIAMAccessKey_pgpKeyReplace(t *testing.T) {
	ctx := acctest.Context(t)
	var initialKey awstypes.AccessKeyMetadata
	var replacementKey awstypes.AccessKeyMetadata
	var initialFingerprint string
	resourceName := "ctyun_iam_access_key.test"
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix+"-ak")
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix+"-ak-policy")
	initialPublicKey, initialPrivateKey := testAccPGPKeyPair(t)
	replacementPublicKey, replacementPrivateKey := testAccPGPKeyPair(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccessKeyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccAccessKeyConfig_pgpKey(userName, policyName, initialPublicKey),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeyExists(ctx, t, resourceName, &initialKey),
					testAccCheckAccessKeyState(resourceName, &initialKey, userName, string(awstypes.StatusTypeActive)),
					resource.TestCheckNoResourceAttr(resourceName, "secret"),
					resource.TestCheckResourceAttrSet(resourceName, "encrypted_secret"),
					resource.TestCheckResourceAttrSet(resourceName, "key_fingerprint"),
					testAccCheckAccessKeyFingerprint(resourceName, &initialFingerprint),
					testAccCheckAccessKeyEncryptedSecretDecrypts(resourceName, initialPrivateKey),
				),
			},
			{
				Config: testAccAccessKeyConfig_pgpKey(userName, policyName, replacementPublicKey),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeyExists(ctx, t, resourceName, &replacementKey),
					testAccCheckAccessKeyState(resourceName, &replacementKey, userName, string(awstypes.StatusTypeActive)),
					testAccCheckAccessKeyReplacedAndDeleted(ctx, t, resourceName, userName, &initialKey),
					resource.TestCheckResourceAttrSet(resourceName, "key_fingerprint"),
					testAccCheckAccessKeyFingerprintChanged(resourceName, &initialFingerprint),
					testAccCheckAccessKeyDecryptAndUse(ctx, t, resourceName, replacementPrivateKey),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionReplace),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TC-IAM-AK-01 Basic Create: changing user makes Terraform replace the access key and delete the old key for the original user.
func TestAccIAMAccessKey_userReplace(t *testing.T) {
	ctx := acctest.Context(t)
	var initialKey awstypes.AccessKeyMetadata
	var replacementKey awstypes.AccessKeyMetadata
	resourceName := "ctyun_iam_access_key.test"
	initialUserName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix+"-ak-initial")
	replacementUserName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix+"-ak-replacement")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccessKeyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccAccessKeyConfig_userReplace(initialUserName, replacementUserName, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeyExists(ctx, t, resourceName, &initialKey),
					testAccCheckAccessKeyState(resourceName, &initialKey, initialUserName, string(awstypes.StatusTypeActive)),
				),
			},
			{
				Config: testAccAccessKeyConfig_userReplace(initialUserName, replacementUserName, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeyExists(ctx, t, resourceName, &replacementKey),
					testAccCheckAccessKeyState(resourceName, &replacementKey, replacementUserName, string(awstypes.StatusTypeActive)),
					testAccCheckAccessKeyReplacedAndDeleted(ctx, t, resourceName, initialUserName, &initialKey),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionReplace),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TC-IAM-AK-02-Status Update verifies that access-key status can switch in both directions between Active and Inactive.
func TestAccIAMAccessKey_status(t *testing.T) {
	ctx := acctest.Context(t)
	var conf awstypes.AccessKeyMetadata
	resourceName := "ctyun_iam_access_key.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix+"-ak")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccessKeyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccAccessKeyConfig_status(rName, string(awstypes.StatusTypeInactive)),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeyExists(ctx, t, resourceName, &conf),
					testAccCheckAccessKeyState(resourceName, &conf, rName, string(awstypes.StatusTypeInactive)),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"encrypted_secret", "key_fingerprint", "last_used_date", "pgp_key", "secret", "service_name"},
			},
			{
				Config: testAccAccessKeyConfig_status(rName, string(awstypes.StatusTypeActive)),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeyExists(ctx, t, resourceName, &conf),
					testAccCheckAccessKeyState(resourceName, &conf, rName, string(awstypes.StatusTypeActive)),
				),
			},
			{
				Config: testAccAccessKeyConfig_status(rName, string(awstypes.StatusTypeInactive)),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeyExists(ctx, t, resourceName, &conf),
					testAccCheckAccessKeyState(resourceName, &conf, rName, string(awstypes.StatusTypeInactive)),
				),
			},
		},
	})
}

// TC-IAM-AK-03-Status Validation verifies that status accepts only valid enum values and rejects invalid casing or unknown states.
func TestUnitIAMAccessKey_StatusValidation(t *testing.T) {
	t.Parallel()

	resource := tfiam.ResourceAccessKey()
	statusSchema := resource.Schema[names.AttrStatus]
	if !statusSchema.Optional {
		t.Fatal("expected status to be optional")
	}
	if !statusSchema.Computed {
		t.Fatal("expected status to be computed")
	}
	if statusSchema.Default != nil {
		t.Fatalf("expected status default to be nil, got %#v", statusSchema.Default)
	}

	validate := statusSchema.ValidateDiagFunc
	if validate == nil {
		t.Fatal("expected status validation")
	}

	testCases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "active", value: string(awstypes.StatusTypeActive), wantErr: false},
		{name: "inactive", value: string(awstypes.StatusTypeInactive), wantErr: false},
		{name: "wrong_case_lower", value: "inactive", wantErr: true},
		{name: "wrong_case_upper", value: "ACTIVE", wantErr: true},
		{name: "unknown", value: "disabled", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			diags := validate(tc.value, nil)
			if gotErr := diags.HasError(); gotErr != tc.wantErr {
				t.Fatalf("got error = %t, want %t; diagnostics: %v", gotErr, tc.wantErr, diags)
			}
		})
	}
}

// TC-IAM-AK-04-Delete Active Key verifies that an access key created Active by the service without explicit status can be destroyed normally.
func TestAccIAMAccessKey_deleteActive(t *testing.T) {
	ctx := acctest.Context(t)
	var conf awstypes.AccessKeyMetadata
	resourceName := "ctyun_iam_access_key.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccessKeyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccAccessKeyConfig_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeyExists(ctx, t, resourceName, &conf),
					testAccCheckAccessKeyState(resourceName, &conf, rName, string(awstypes.StatusTypeActive)),
				),
			},
		},
	})
}

// TC-IAM-AK-05-LastUsed Null Readback verifies that a newly created, unused access key has empty last_used_date and service_name when read.
func TestAccIAMAccessKey_lastUsedNull(t *testing.T) {
	ctx := acctest.Context(t)
	var conf awstypes.AccessKeyMetadata
	resourceName := "ctyun_iam_access_key.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccessKeyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccAccessKeyConfig_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeyExists(ctx, t, resourceName, &conf),
					testAccCheckAccessKeyLastUsedEmpty(resourceName),
				),
			},
		},
	})
}

// TC-IAM-AK-06-LastUsed Populated Readback verifies that, after attaching the minimum iam:GetUser policy, a new access key uses itself for GetUser, the service records its use time and IAM service name, and refresh populates the corresponding state.
func TestAccIAMAccessKey_lastUsedPopulated(t *testing.T) {
	ctx := acctest.Context(t)
	var conf awstypes.AccessKeyMetadata
	resourceName := "ctyun_iam_access_key.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix+"-ak")
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix+"-ak-policy")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccessKeyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccAccessKeyConfig_lastUsedPopulated(rName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeyExists(ctx, t, resourceName, &conf),
					testAccCheckAccessKeyLastUsedEmpty(resourceName),
				),
			},
			{
				Config: testAccAccessKeyConfig_lastUsedPopulated(rName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeyUseAndWaitForLastUsed(ctx, t, resourceName),
				),
			},
			{
				RefreshState: true,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeyLastUsedPopulated(ctx, t, resourceName),
				),
			},
		},
	})
}

// TC-IAM-AK-07-LastUsed Evolution verifies that after cloud-side use of an access key, a plan with identical configuration refreshes state itself and produces no diff before or after apply.
func TestAccIAMAccessKey_lastUsedEvolution(t *testing.T) {
	ctx := acctest.Context(t)
	var conf awstypes.AccessKeyMetadata
	accessKeyResourceName := "ctyun_iam_access_key.test"
	policyResourceName := "ctyun_iam_policy.test"
	attachmentResourceName := "ctyun_iam_user_policy_attachment.test"
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix+"-ak")
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix+"-ak-policy")
	config := testAccAccessKeyConfig_lastUsedPopulated(userName, policyName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccessKeyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeyExists(ctx, t, accessKeyResourceName, &conf),
					testAccCheckAccessKeyLastUsedEmpty(accessKeyResourceName),
				),
			},
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeyUseAndWaitForLastUsed(ctx, t, accessKeyResourceName),
				),
			},
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeyLastUsedPopulated(ctx, t, accessKeyResourceName),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(accessKeyResourceName, plancheck.ResourceActionNoop),
						plancheck.ExpectResourceAction(policyResourceName, plancheck.ResourceActionNoop),
						plancheck.ExpectResourceAction(attachmentResourceName, plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(accessKeyResourceName, plancheck.ResourceActionNoop),
						plancheck.ExpectResourceAction(policyResourceName, plancheck.ResourceActionNoop),
						plancheck.ExpectResourceAction(attachmentResourceName, plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

// TC-IAM-AK-08-Import Via GetAccessKeyLastUsed verifies that after an access key is externally created and used, importing by ID alone lets the provider populate user and last-used fields through GetAccessKeyLastUsed, then normalize status on subsequent Read.
func TestAccIAMAccessKey_importByID(t *testing.T) {
	ctx := acctest.Context(t)
	resourceName := "ctyun_iam_access_key.test"
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix+"-ak")
	policyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix+"-ak-policy")
	acctest.PreCheck(ctx, t)
	conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
	fixture := testAccCreateAccessKeyImportFixture(ctx, t, conn, userName, policyName)
	config := testAccAccessKeyConfig_importByID(fixture.userName)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccessKeyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config:             config,
				ResourceName:       resourceName,
				ImportState:        true,
				ImportStatePersist: true,
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					if fixture.accessKey.ID == "" {
						return "", fmt.Errorf("externally created IAM Access Key ID is empty")
					}

					return fixture.accessKey.ID, nil
				},
				ImportStateCheck: testAccCheckAccessKeyImportState(fixture.accessKey),
			},
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeyLastUsedPopulated(ctx, t, resourceName),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

// TC-IAM-AK-09-AccessDenied Negative creates a user/key with a second principal through the SDK, imports with the primary provider and asserts AccessDenied, then uses a read-only plan to verify the failed import left no state; skipped when second-principal credentials are unset.
func TestAccIAMAccessKey_accessDenied(t *testing.T) {
	ctx := acctest.Context(t)
	resourceName := "ctyun_iam_access_key.test"
	secondAccessKey := os.Getenv("CTYUNOOS_ACCESS_KEY_SECOND")
	secondSecretKey := os.Getenv("CTYUNOOS_SECRET_KEY_SECOND")
	endpoint := os.Getenv("CTYUNOOS_ENDPOINT_IAM")
	if secondAccessKey == "" || secondSecretKey == "" {
		t.Skip("skipping IAM Access Key AccessDenied test: CTYUNOOS_ACCESS_KEY_SECOND and CTYUNOOS_SECRET_KEY_SECOND must be configured")
	}
	if endpoint == "" {
		t.Skip("skipping IAM Access Key AccessDenied test: CTYUNOOS_ENDPOINT_IAM must be configured")
	}

	acctest.PreCheck(ctx, t)
	secondConn, err := tfiam.IAMClientWithStaticCredentials(ctx, acctest.ProviderMeta(ctx, t), endpoint, secondAccessKey, secondSecretKey)
	if err != nil {
		t.Fatalf("creating IAM client with second credentials: %s", err)
	}
	fixture := testAccCreateAccessKeyAccessDeniedFixture(ctx, t, secondConn)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:             testAccAccessKeyConfig_accessDeniedImport(fixture.userName),
				ResourceName:       resourceName,
				ImportState:        true,
				ImportStateId:      fixture.accessKeyID,
				ImportStatePersist: false,
				ExpectError:        regexp.MustCompile(`(?i)AccessDenied`),
			},
			{
				Config:             testAccAccessKeyConfig_accessDeniedImport(fixture.userName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					// PlanOnly runs these as its non-refresh and refresh plans, respectively.
					PostApplyPreRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
				},
			},
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check:              testAccCheckAccessKeyRemovedFromState(resourceName),
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

type accessKeyAccessDeniedFixture struct {
	accessKeyID string
	userCreated bool
	userName    string
}

func testAccCreateAccessKeyAccessDeniedFixture(ctx context.Context, t *testing.T, conn *iam.Client) *accessKeyAccessDeniedFixture {
	t.Helper()

	fixture := &accessKeyAccessDeniedFixture{
		userName: acctest.RandomWithPrefix(t, acctest.ResourcePrefix+"-ak-denied"),
	}
	t.Cleanup(func() {
		if err := testAccDestroyAccessKeyAccessDeniedFixture(context.Background(), conn, fixture); err != nil {
			t.Errorf("destroying IAM Access Key AccessDenied fixture: %s", err)
		}
	})

	_, err := conn.CreateUser(ctx, &iam.CreateUserInput{
		UserName: aws.String(fixture.userName),
	})
	if err != nil {
		t.Fatalf("creating second-principal IAM User (%s): %s", fixture.userName, err)
	}
	fixture.userCreated = true

	output, err := conn.CreateAccessKey(ctx, &iam.CreateAccessKeyInput{
		UserName: aws.String(fixture.userName),
	})
	if err != nil {
		t.Fatalf("creating second-principal IAM Access Key for User (%s): %s", fixture.userName, err)
	}
	if output == nil || output.AccessKey == nil || output.AccessKey.AccessKeyId == nil {
		t.Fatalf("creating second-principal IAM Access Key for User (%s): empty response", fixture.userName)
	}

	fixture.accessKeyID = aws.ToString(output.AccessKey.AccessKeyId)
	return fixture
}

func testAccDestroyAccessKeyAccessDeniedFixture(ctx context.Context, conn *iam.Client, fixture *accessKeyAccessDeniedFixture) error {
	if fixture == nil || !fixture.userCreated || fixture.userName == "" {
		return nil
	}

	if fixture.accessKeyID != "" {
		_, err := conn.UpdateAccessKey(ctx, &iam.UpdateAccessKeyInput{
			AccessKeyId: aws.String(fixture.accessKeyID),
			Status:      awstypes.StatusTypeInactive,
			UserName:    aws.String(fixture.userName),
		})
		if err != nil {
			return fmt.Errorf("deactivating second-principal IAM Access Key (%s): %w", fixture.accessKeyID, err)
		}

		_, err = conn.DeleteAccessKey(ctx, &iam.DeleteAccessKeyInput{
			AccessKeyId: aws.String(fixture.accessKeyID),
			UserName:    aws.String(fixture.userName),
		})
		if err != nil {
			return fmt.Errorf("deleting second-principal IAM Access Key (%s): %w", fixture.accessKeyID, err)
		}
	}

	_, err := conn.DeleteUser(ctx, &iam.DeleteUserInput{
		UserName: aws.String(fixture.userName),
	})
	if err != nil {
		return fmt.Errorf("deleting second-principal IAM User (%s): %w", fixture.userName, err)
	}

	return nil
}

// TC-IAM-AK-10-Disappears verifies that after an access key is deleted outside Terraform through the IAM API, refresh removes its resource state and produces a Create plan.
func TestAccIAMAccessKey_disappears(t *testing.T) {
	ctx := acctest.Context(t)
	var conf awstypes.AccessKeyMetadata
	resourceName := "ctyun_iam_access_key.test"
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckAccessKeyDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccAccessKeyConfig_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeyExists(ctx, t, resourceName, &conf),
					testAccCheckAccessKeyDisappearsExternally(ctx, t, resourceName),
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
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAccessKeyRemovedFromState(resourceName),
				),
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

// TestUnitIAMAccessKey_UserValidation verifies length and character-set validation for the user argument.
func TestUnitIAMAccessKey_UserValidation(t *testing.T) {
	t.Parallel()

	userSchema := tfiam.ResourceAccessKey().Schema["user"]
	if !userSchema.Required {
		t.Fatal("expected user to be required")
	}
	if userSchema.Optional {
		t.Fatal("expected user not to be optional")
	}
	if userSchema.Computed {
		t.Fatal("expected user not to be computed")
	}
	if !userSchema.ForceNew {
		t.Fatal("expected user changes to force a new access key")
	}

	validate := userSchema.ValidateFunc
	if validate == nil {
		t.Fatal("expected user validation")
	}

	testCases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "valid", value: "User_-,.+@=123", wantErr: false},
		{name: "empty", value: "", wantErr: true},
		{name: "too_long", value: strings.Repeat("a", 65), wantErr: true},
		{name: "invalid_character", value: "invalid user!", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := validate(tc.value, "user")
			if gotErr := len(errs) > 0; gotErr != tc.wantErr {
				t.Fatalf("got error = %t, want %t; errors: %v", gotErr, tc.wantErr, errs)
			}
		})
	}
}

// TestResourceAccessKeyReadResult verifies that access-key metadata is correctly written to Terraform state.
func TestResourceAccessKeyReadResult(t *testing.T) {
	t.Parallel()

	createDate := time.Date(2026, 6, 24, 9, 30, 0, 0, time.UTC)
	d := schema.TestResourceDataRaw(t, tfiam.ResourceAccessKey().Schema, map[string]any{})

	tfiam.AccessKeyReadResult(d, &awstypes.AccessKeyMetadata{
		AccessKeyId: aws.String("AKIATEST"),
		CreateDate:  aws.Time(createDate),
		Status:      awstypes.StatusTypeInactive,
		UserName:    aws.String("test-user"),
	})

	if got, want := d.Id(), "AKIATEST"; got != want {
		t.Fatalf("id = %q, want %q", got, want)
	}

	for name, want := range map[string]string{
		"create_date":    createDate.Format(time.RFC3339),
		names.AttrStatus: string(awstypes.StatusTypeInactive),
		"user":           "test-user",
	} {
		if got := d.Get(name).(string); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
}

// TestResourceAccessKeyReadResultNilCreateDate verifies that a nil CreateDate clears the create_date field.
func TestResourceAccessKeyReadResultNilCreateDate(t *testing.T) {
	t.Parallel()

	d := schema.TestResourceDataRaw(t, tfiam.ResourceAccessKey().Schema, map[string]any{
		"create_date": time.Now().Format(time.RFC3339),
	})

	tfiam.AccessKeyReadResult(d, &awstypes.AccessKeyMetadata{
		AccessKeyId: aws.String("AKIATEST"),
		Status:      awstypes.StatusTypeActive,
		UserName:    aws.String("test-user"),
	})

	if got := d.Get("create_date"); got != "" {
		t.Fatalf("create_date = %#v, want empty value", got)
	}
}

// TestResourceAccessKeyLastUsedReadResult verifies that last-used information is correctly written to Terraform state; this unit test covers only local state population, not the AK-06 end-to-end readback scenario.
func TestResourceAccessKeyLastUsedReadResult(t *testing.T) {
	t.Parallel()

	lastUsedDate := time.Date(2026, 6, 24, 10, 45, 0, 0, time.UTC)
	d := schema.TestResourceDataRaw(t, tfiam.ResourceAccessKey().Schema, map[string]any{})

	tfiam.AccessKeyLastUsedReadResult(d, &iam.GetAccessKeyLastUsedOutput{
		AccessKeyLastUsed: &awstypes.AccessKeyLastUsed{
			LastUsedDate: aws.Time(lastUsedDate),
			ServiceName:  aws.String("s3"),
		},
	})

	for name, want := range map[string]string{
		"last_used_date": lastUsedDate.Format(time.RFC3339),
		"service_name":   "s3",
	} {
		if got := d.Get(name).(string); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
}

// TestResourceAccessKeyLastUsedReadResultEmpty verifies that empty last-used information clears the related fields.
func TestResourceAccessKeyLastUsedReadResultEmpty(t *testing.T) {
	t.Parallel()

	d := schema.TestResourceDataRaw(t, tfiam.ResourceAccessKey().Schema, map[string]any{
		"last_used_date": time.Now().Format(time.RFC3339),
		"service_name":   "iam",
	})

	tfiam.AccessKeyLastUsedReadResult(d, nil)

	for _, name := range []string{"last_used_date", "service_name"} {
		if got := d.Get(name); got != "" {
			t.Fatalf("%s = %#v, want empty value", name, got)
		}
	}
}

// TestAccessKeysMutex verifies that the access-key mutex serializes concurrent operations for one user while allowing different users to proceed independently.
func TestAccessKeysMutex(t *testing.T) {
	username := "TestUser"

	if got, want := tfiam.AccessKeysMutexKey(username), "oos-iam-access-keys/testuser"; got != want {
		t.Fatalf("access key mutex key = %q, want %q", got, want)
	}

	unlock := tfiam.LockAccessKeys(username)
	lockedCh := make(chan func(), 1)

	go func() {
		lockedCh <- tfiam.LockAccessKeys(strings.ToLower(username))
	}()

	select {
	case secondUnlock := <-lockedCh:
		secondUnlock()
		unlock()
		t.Fatal("second lock for same user was acquired before first lock was released")
	case <-time.After(50 * time.Millisecond):
		// pass
	}

	unlock()

	select {
	case secondUnlock := <-lockedCh:
		secondUnlock()
	case <-time.After(time.Second):
		t.Fatal("second lock for same user did not acquire after first lock was released")
	}

	unlock = tfiam.LockAccessKeys(username)
	differentUserLockedCh := make(chan func(), 1)

	go func() {
		differentUserLockedCh <- tfiam.LockAccessKeys("OtherUser")
	}()

	select {
	case differentUserUnlock := <-differentUserLockedCh:
		differentUserUnlock()
	case <-time.After(50 * time.Millisecond):
		unlock()
		t.Fatal("lock for different user blocked on access key mutex")
	}

	unlock()
}

func TestAccessKeyPGPKeyPair(t *testing.T) {
	t.Parallel()

	publicKey, privateKey := testAccPGPKeyPair(t)
	_, encryptedValues, err := pgpkeys.EncryptShares([][]byte{[]byte("test-secret-access-key")}, []string{publicKey})
	if err != nil {
		t.Fatalf("encrypting with generated PGP public key: %s", err)
	}

	decryptedValue, err := pgpkeys.DecryptBytes(base64.StdEncoding.EncodeToString(encryptedValues[0]), privateKey)
	if err != nil {
		t.Fatalf("decrypting with generated PGP private key: %s", err)
	}
	if got, want := decryptedValue.String(), "test-secret-access-key"; got != want {
		t.Fatalf("decrypted value = %q, want %q", got, want)
	}
}

func testAccCheckAccessKeyDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_iam_access_key" {
				continue
			}

			_, err := tfiam.FindAccessKeyByTwoPartKey(ctx, conn, rs.Primary.Attributes["user"], rs.Primary.ID)

			if retry.NotFound(err) {
				continue
			}

			if err != nil {
				return err
			}

			return fmt.Errorf("IAM Access Key (%s) still exists", rs.Primary.ID)
		}

		return nil
	}
}

func testAccCheckAccessKeyIDsDistinct(resourceNames []string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ids := make(map[string]string, len(resourceNames))

		for _, resourceName := range resourceNames {
			rs, ok := s.RootModule().Resources[resourceName]
			if !ok {
				return fmt.Errorf("not found: %s", resourceName)
			}

			if rs.Primary.ID == "" {
				return fmt.Errorf("IAM Access Key (%s) has an empty ID", resourceName)
			}
			if previousResourceName, ok := ids[rs.Primary.ID]; ok {
				return fmt.Errorf("IAM Access Key ID %s is shared by %s and %s", rs.Primary.ID, previousResourceName, resourceName)
			}

			ids[rs.Primary.ID] = resourceName
		}

		return nil
	}
}

func testAccCheckAccessKeyExists(ctx context.Context, t *testing.T, n string, v *awstypes.AccessKeyMetadata) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		output, err := tfiam.FindAccessKeyByTwoPartKey(ctx, conn, rs.Primary.Attributes["user"], rs.Primary.ID)

		if err != nil {
			return err
		}

		*v = *output

		return nil
	}
}

func testAccCheckAccessKeyReplacedAndDeleted(ctx context.Context, t *testing.T, resourceName, oldUserName string, oldAccessKey *awstypes.AccessKeyMetadata) resource.TestCheckFunc {
	t.Helper()

	return func(s *terraform.State) error {
		if oldAccessKey == nil {
			return fmt.Errorf("old IAM Access Key metadata is nil")
		}

		oldAccessKeyID := aws.ToString(oldAccessKey.AccessKeyId)
		if oldAccessKeyID == "" {
			return fmt.Errorf("old IAM Access Key ID is empty")
		}

		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}
		if rs.Primary.ID == oldAccessKeyID {
			return fmt.Errorf("IAM Access Key replacement kept old ID %s", oldAccessKeyID)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		if _, err := tfresource.RetryUntilNotFound(ctx, time.Minute, func(ctx context.Context) (any, error) {
			return tfiam.FindAccessKeyByTwoPartKey(ctx, conn, oldUserName, oldAccessKeyID)
		}); err != nil {
			return fmt.Errorf("waiting for replaced IAM Access Key (%s) deletion: %w", oldAccessKeyID, err)
		}

		return nil
	}
}

func testAccCheckAccessKeyEncryptedSecretDecrypts(resourceName, pgpPrivateKey string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		encryptedSecret := rs.Primary.Attributes["encrypted_secret"]
		if encryptedSecret == "" {
			return fmt.Errorf("IAM Access Key (%s) encrypted_secret is empty", rs.Primary.ID)
		}

		secretAccessKey, err := pgpkeys.DecryptBytes(encryptedSecret, pgpPrivateKey)
		if err != nil {
			return fmt.Errorf("decrypting IAM Access Key (%s) encrypted_secret: %w", rs.Primary.ID, err)
		}
		if secretAccessKey.Len() == 0 {
			return fmt.Errorf("decrypted IAM Access Key (%s) SecretAccessKey is empty", rs.Primary.ID)
		}

		return nil
	}
}

func testAccCheckAccessKeyFingerprint(resourceName string, fingerprint *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		got := rs.Primary.Attributes["key_fingerprint"]
		if got == "" {
			return fmt.Errorf("IAM Access Key (%s) key_fingerprint is empty", rs.Primary.ID)
		}

		*fingerprint = got
		return nil
	}
}

func testAccCheckAccessKeyFingerprintChanged(resourceName string, previousFingerprint *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if *previousFingerprint == "" {
			return fmt.Errorf("previous IAM Access Key key_fingerprint is empty")
		}

		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		if got := rs.Primary.Attributes["key_fingerprint"]; got == "" {
			return fmt.Errorf("replacement IAM Access Key (%s) key_fingerprint is empty", rs.Primary.ID)
		} else if got == *previousFingerprint {
			return fmt.Errorf("replacement IAM Access Key (%s) key_fingerprint did not change", rs.Primary.ID)
		}

		return nil
	}
}

// testAccCheckAccessKeyDisappearsExternally removes the key through IAM APIs,
// deliberately avoiding the Terraform resource Delete implementation.
func testAccCheckAccessKeyDisappearsExternally(ctx context.Context, t *testing.T, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		username := rs.Primary.Attributes["user"]
		accessKeyID := rs.Primary.ID
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		_, err := conn.UpdateAccessKey(ctx, &iam.UpdateAccessKeyInput{
			AccessKeyId: aws.String(accessKeyID),
			Status:      awstypes.StatusTypeInactive,
			UserName:    aws.String(username),
		})
		if err != nil {
			return fmt.Errorf("externally deactivating IAM Access Key (%s): %w", accessKeyID, err)
		}

		_, err = conn.DeleteAccessKey(ctx, &iam.DeleteAccessKeyInput{
			AccessKeyId: aws.String(accessKeyID),
			UserName:    aws.String(username),
		})
		if err != nil {
			return fmt.Errorf("externally deleting IAM Access Key (%s): %w", accessKeyID, err)
		}

		return nil
	}
}

func testAccCheckAccessKeyRemovedFromState(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if _, ok := s.RootModule().Resources[resourceName]; ok {
			return fmt.Errorf("IAM Access Key resource (%s) is still present in state", resourceName)
		}

		return nil
	}
}

func testAccCheckAccessKeyState(resourceName string, accessKeyMetadata *awstypes.AccessKeyMetadata, wantUser string, wantStatus string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		if got := aws.ToString(accessKeyMetadata.UserName); got != wantUser {
			return fmt.Errorf("bad remote user: got %s, want %s", got, wantUser)
		}

		if got := string(accessKeyMetadata.Status); got != wantStatus {
			return fmt.Errorf("bad remote status: got %s, want %s", got, wantStatus)
		}

		if got := rs.Primary.ID; got != aws.ToString(accessKeyMetadata.AccessKeyId) {
			return fmt.Errorf("bad state id: got %s, want %s", got, aws.ToString(accessKeyMetadata.AccessKeyId))
		}

		if got := rs.Primary.Attributes["user"]; got != wantUser {
			return fmt.Errorf("bad state user: got %s, want %s", got, wantUser)
		}

		if got := rs.Primary.Attributes[names.AttrStatus]; got != wantStatus {
			return fmt.Errorf("bad state status: got %s, want %s", got, wantStatus)
		}

		if got := rs.Primary.Attributes["create_date"]; got == "" {
			return fmt.Errorf("create_date is empty")
		}

		return nil
	}
}

func testAccCheckAccessKeyLastUsedEmpty(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		for _, name := range []string{"last_used_date", "service_name"} {
			if got := rs.Primary.Attributes[name]; got != "" {
				return fmt.Errorf("expected %s to be empty, got %q", name, got)
			}
		}

		return nil
	}
}

type accessKeyImportRef struct {
	ID           string
	Secret       string
	UserName     string
	LastUsedDate string
	ServiceName  string
}

type accessKeyImportFixture struct {
	accessKey      *accessKeyImportRef
	policyARN      string
	policyAttached bool
	policyCreated  bool
	userCreated    bool
	userName       string
}

func testAccCreateAccessKeyImportFixture(ctx context.Context, t *testing.T, conn *iam.Client, userName, policyName string) *accessKeyImportFixture {
	t.Helper()

	fixture := &accessKeyImportFixture{userName: userName}
	t.Cleanup(func() {
		if err := testAccDestroyAccessKeyImportFixture(context.Background(), conn, fixture); err != nil {
			t.Errorf("destroying IAM Access Key import fixture: %s", err)
		}
	})

	_, err := conn.CreateUser(ctx, &iam.CreateUserInput{UserName: aws.String(fixture.userName)})
	if err != nil {
		t.Fatalf("creating IAM User (%s) for access key import: %s", fixture.userName, err)
	}
	fixture.userCreated = true

	policyOutput, err := conn.CreatePolicy(ctx, &iam.CreatePolicyInput{
		Description: aws.String("Allow GetUser for access key import testing"),
		PolicyDocument: aws.String(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": ["iam:GetUser"],
      "Resource": "*",
      "Effect": "Allow"
    }
  ]
}`),
		PolicyName: aws.String(policyName),
	})
	if err != nil {
		t.Fatalf("creating IAM Policy (%s) for access key import: %s", policyName, err)
	}
	if policyOutput == nil || policyOutput.Policy == nil || aws.ToString(policyOutput.Policy.Arn) == "" {
		t.Fatalf("creating IAM Policy (%s) for access key import: empty response", policyName)
	}
	fixture.policyARN = aws.ToString(policyOutput.Policy.Arn)
	fixture.policyCreated = true

	_, err = conn.AttachUserPolicy(ctx, &iam.AttachUserPolicyInput{
		PolicyArn: aws.String(fixture.policyARN),
		UserName:  aws.String(fixture.userName),
	})
	if err != nil {
		t.Fatalf("attaching IAM Policy (%s) to User (%s): %s", fixture.policyARN, fixture.userName, err)
	}
	fixture.policyAttached = true

	accessKeyOutput, err := conn.CreateAccessKey(ctx, &iam.CreateAccessKeyInput{UserName: aws.String(fixture.userName)})
	if err != nil {
		t.Fatalf("creating external IAM Access Key for User (%s): %s", fixture.userName, err)
	}
	if accessKeyOutput == nil || accessKeyOutput.AccessKey == nil || accessKeyOutput.AccessKey.AccessKeyId == nil || accessKeyOutput.AccessKey.SecretAccessKey == nil {
		t.Fatalf("creating external IAM Access Key for User (%s): empty response", fixture.userName)
	}

	lastUsedOutput, err := testAccUseAccessKeyAndWaitForLastUsed(ctx, t, aws.ToString(accessKeyOutput.AccessKey.AccessKeyId), aws.ToString(accessKeyOutput.AccessKey.SecretAccessKey))
	if err != nil {
		t.Fatalf("using external IAM Access Key for User (%s): %s", fixture.userName, err)
	}

	fixture.accessKey = &accessKeyImportRef{
		ID:           aws.ToString(accessKeyOutput.AccessKey.AccessKeyId),
		Secret:       aws.ToString(accessKeyOutput.AccessKey.SecretAccessKey),
		UserName:     fixture.userName,
		LastUsedDate: aws.ToTime(lastUsedOutput.AccessKeyLastUsed.LastUsedDate).Format(time.RFC3339),
		ServiceName:  aws.ToString(lastUsedOutput.AccessKeyLastUsed.ServiceName),
	}

	return fixture
}

func testAccDestroyAccessKeyImportFixture(ctx context.Context, conn *iam.Client, fixture *accessKeyImportFixture) error {
	if fixture == nil {
		return nil
	}

	if fixture.userCreated && fixture.accessKey != nil && fixture.accessKey.ID != "" {
		_, err := tfiam.FindAccessKeyByTwoPartKey(ctx, conn, fixture.userName, fixture.accessKey.ID)
		if err == nil {
			_, err = conn.UpdateAccessKey(ctx, &iam.UpdateAccessKeyInput{
				AccessKeyId: aws.String(fixture.accessKey.ID),
				Status:      awstypes.StatusTypeInactive,
				UserName:    aws.String(fixture.userName),
			})
			if err != nil {
				return fmt.Errorf("deactivating external IAM Access Key (%s): %w", fixture.accessKey.ID, err)
			}

			_, err = conn.DeleteAccessKey(ctx, &iam.DeleteAccessKeyInput{
				AccessKeyId: aws.String(fixture.accessKey.ID),
				UserName:    aws.String(fixture.userName),
			})
			if err != nil {
				return fmt.Errorf("deleting external IAM Access Key (%s): %w", fixture.accessKey.ID, err)
			}
		} else if !retry.NotFound(err) {
			return fmt.Errorf("reading external IAM Access Key (%s): %w", fixture.accessKey.ID, err)
		}
	}

	if fixture.policyAttached {
		_, err := conn.DetachUserPolicy(ctx, &iam.DetachUserPolicyInput{
			PolicyArn: aws.String(fixture.policyARN),
			UserName:  aws.String(fixture.userName),
		})
		if err != nil {
			return fmt.Errorf("detaching IAM Policy (%s) from User (%s): %w", fixture.policyARN, fixture.userName, err)
		}
	}

	if fixture.policyCreated {
		_, err := conn.DeletePolicy(ctx, &iam.DeletePolicyInput{PolicyArn: aws.String(fixture.policyARN)})
		if err != nil {
			return fmt.Errorf("deleting IAM Policy (%s): %w", fixture.policyARN, err)
		}
	}

	if fixture.userCreated {
		_, err := conn.DeleteUser(ctx, &iam.DeleteUserInput{UserName: aws.String(fixture.userName)})
		if err != nil {
			return fmt.Errorf("deleting IAM User (%s): %w", fixture.userName, err)
		}
	}

	return nil
}

func testAccCheckAccessKeyImportState(accessKey *accessKeyImportRef) resource.ImportStateCheckFunc {
	return func(states []*terraform.InstanceState) error {
		if accessKey == nil || accessKey.ID == "" {
			return fmt.Errorf("externally created IAM Access Key is not available")
		}
		if len(states) != 1 {
			return fmt.Errorf("expected 1 imported state, got %d", len(states))
		}

		state := states[0]
		if state.ID != accessKey.ID {
			return fmt.Errorf("imported access key ID = %q, want %q", state.ID, accessKey.ID)
		}

		for name, want := range map[string]string{
			"user":           accessKey.UserName,
			"last_used_date": accessKey.LastUsedDate,
			"service_name":   accessKey.ServiceName,
			names.AttrStatus: string(awstypes.StatusTypeActive),
		} {
			if got := state.Attributes[name]; got != want {
				return fmt.Errorf("imported %s = %q, want %q", name, got, want)
			}
		}

		return nil
	}
}

func testAccCheckAccessKeyUseAndWaitForLastUsed(ctx context.Context, t *testing.T, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		accessKeyID := rs.Primary.ID
		secretAccessKey := rs.Primary.Attributes["secret"]
		if accessKeyID == "" || secretAccessKey == "" {
			return fmt.Errorf("IAM Access Key %s is missing id or secret in state", resourceName)
		}

		_, err := testAccUseAccessKeyAndWaitForLastUsed(ctx, t, accessKeyID, secretAccessKey)
		return err
	}
}

func testAccCheckAccessKeyDecryptAndUse(ctx context.Context, t *testing.T, resourceName, pgpPrivateKey string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		accessKeyID := rs.Primary.ID
		encryptedSecret := rs.Primary.Attributes["encrypted_secret"]
		if accessKeyID == "" || encryptedSecret == "" {
			return fmt.Errorf("IAM Access Key %s is missing id or encrypted_secret in state", resourceName)
		}

		secretAccessKey, err := pgpkeys.DecryptBytes(encryptedSecret, pgpPrivateKey)
		if err != nil {
			return fmt.Errorf("decrypting IAM Access Key (%s) encrypted_secret: %w", accessKeyID, err)
		}
		if secretAccessKey.Len() == 0 {
			return fmt.Errorf("decrypted IAM Access Key (%s) SecretAccessKey is empty", accessKeyID)
		}

		_, err = testAccUseAccessKeyAndWaitForLastUsed(ctx, t, accessKeyID, secretAccessKey.String())
		return err
	}
}

func testAccUseAccessKeyAndWaitForLastUsed(ctx context.Context, t *testing.T, accessKeyID, secretAccessKey string) (*iam.GetAccessKeyLastUsedOutput, error) {
	endpoint := os.Getenv("CTYUNOOS_ENDPOINT_IAM")
	if endpoint == "" {
		return nil, fmt.Errorf("CTYUNOOS_ENDPOINT_IAM must be set to verify IAM Access Key last-used data")
	}

	meta := acctest.ProviderMeta(ctx, t)
	if err := sdkretry.RetryContext(ctx, time.Minute, func() *sdkretry.RetryError {
		err := tfiam.TestProbeGetUserWithStaticCredentials(ctx, meta, endpoint, accessKeyID, secretAccessKey)
		if tfawserr.ErrCodeEquals(err, "AccessDenied") {
			return sdkretry.RetryableError(fmt.Errorf("waiting for IAM Access Key (%s) GetUser permission propagation: %w", accessKeyID, err))
		}
		if err != nil {
			return sdkretry.NonRetryableError(fmt.Errorf("using IAM Access Key (%s) to get itself: %w", accessKeyID, err))
		}

		return nil
	}); err != nil {
		return nil, err
	}

	var lastUsedOutput *iam.GetAccessKeyLastUsedOutput
	err := sdkretry.RetryContext(ctx, time.Minute, func() *sdkretry.RetryError {
		output, err := meta.IAMClient(ctx).GetAccessKeyLastUsed(ctx, &iam.GetAccessKeyLastUsedInput{
			AccessKeyId: aws.String(accessKeyID),
		})
		if err != nil {
			return sdkretry.RetryableError(fmt.Errorf("reading IAM Access Key (%s) last used after GetUser: %w", accessKeyID, err))
		}

		if output == nil || output.AccessKeyLastUsed == nil || output.AccessKeyLastUsed.LastUsedDate == nil {
			return sdkretry.RetryableError(fmt.Errorf("IAM Access Key (%s) last-used data has not propagated", accessKeyID))
		}
		if serviceName := aws.ToString(output.AccessKeyLastUsed.ServiceName); serviceName != "iam" {
			return sdkretry.NonRetryableError(fmt.Errorf("IAM Access Key (%s) service name = %q, want %q after GetUser", accessKeyID, serviceName, "iam"))
		}

		lastUsedOutput = output
		return nil
	})

	return lastUsedOutput, err
}

func testAccCheckAccessKeyLastUsedPopulated(ctx context.Context, t *testing.T, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		accessKeyID := rs.Primary.ID
		output, err := acctest.ProviderMeta(ctx, t).IAMClient(ctx).GetAccessKeyLastUsed(ctx, &iam.GetAccessKeyLastUsedInput{
			AccessKeyId: aws.String(accessKeyID),
		})
		if err != nil {
			return fmt.Errorf("reading IAM Access Key (%s) last used: %w", accessKeyID, err)
		}
		if output == nil || output.AccessKeyLastUsed == nil || output.AccessKeyLastUsed.LastUsedDate == nil {
			return fmt.Errorf("IAM Access Key (%s) last-used data is empty after refresh", accessKeyID)
		}

		if got, want := rs.Primary.Attributes["last_used_date"], aws.ToTime(output.AccessKeyLastUsed.LastUsedDate).Format(time.RFC3339); got != want {
			return fmt.Errorf("last_used_date = %q, want %q", got, want)
		}
		if got, want := rs.Primary.Attributes["service_name"], aws.ToString(output.AccessKeyLastUsed.ServiceName); got != want {
			return fmt.Errorf("service_name = %q, want %q", got, want)
		}
		if got, want := aws.ToString(output.UserName), rs.Primary.Attributes["user"]; got != want {
			return fmt.Errorf("last-used user = %q, want %q", got, want)
		}

		return nil
	}
}

func testAccAccessKeyConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name = %[1]q
}

resource "ctyun_iam_access_key" "test" {
  user = ctyun_iam_user.test.name
}
`, rName)
}

func testAccAccessKeyConfig_concurrentCreate(userName string, accessKeyCount int) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name = %[1]q
}

resource "ctyun_iam_access_key" "test" {
  count = %[2]d
  user  = ctyun_iam_user.test.name
}
`, userName, accessKeyCount)
}

func testAccAccessKeyConfig_accessDeniedImport(userName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_access_key" "test" {
  user = %[1]q
}
`, userName)
}

func testAccAccessKeyConfig_importByID(userName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_access_key" "test" {
  user = %[1]q
}
`, userName)
}

func testAccAccessKeyConfig_status(rName string, status string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name = %[1]q
}

resource "ctyun_iam_access_key" "test" {
  user   = ctyun_iam_user.test.name
  status = %[2]q
}
`, rName, status)
}

func testAccAccessKeyConfig_pgpKey(userName, policyName, pgpKey string) string {
	return testAccAccessKeyConfig_lastUsedPrerequisites(userName, policyName) + fmt.Sprintf(`
resource "ctyun_iam_access_key" "test" {
  user       = ctyun_iam_user.test.name
  pgp_key    = %[1]q
  depends_on = [ctyun_iam_user_policy_attachment.test]
}
`, pgpKey)
}

func testAccAccessKeyConfig_userReplace(initialUserName, replacementUserName string, useReplacementUser bool) string {
	accessKeyUser := "initial"
	if useReplacementUser {
		accessKeyUser = "replacement"
	}

	return fmt.Sprintf(`
resource "ctyun_iam_user" "initial" {
  name = %[1]q
}

resource "ctyun_iam_user" "replacement" {
  name = %[2]q
}

resource "ctyun_iam_access_key" "test" {
  user = ctyun_iam_user.%[3]s.name
}
`, initialUserName, replacementUserName, accessKeyUser)
}

func testAccPGPKeyPair(t *testing.T) (string, string) {
	t.Helper()

	entity, err := openpgp.NewEntity("Terraform OOS Access Key Test", "", "", &packet.Config{RSABits: 1024})
	if err != nil {
		t.Fatalf("creating PGP test key pair: %s", err)
	}

	publicKey := bytes.NewBuffer(nil)
	if err := entity.Serialize(publicKey); err != nil {
		t.Fatalf("serializing PGP test public key: %s", err)
	}

	privateKey := bytes.NewBuffer(nil)
	if err := entity.SerializePrivate(privateKey, nil); err != nil {
		t.Fatalf("serializing PGP test private key: %s", err)
	}

	return base64.StdEncoding.EncodeToString(publicKey.Bytes()), base64.StdEncoding.EncodeToString(privateKey.Bytes())
}

func testAccAccessKeyConfig_lastUsedPrerequisites(userName, policyName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name          = %[1]q
  force_destroy = true
}

resource "ctyun_iam_policy" "test" {
  name        = %[2]q
  description = "Allow GetUser for access key last-used testing"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": ["iam:GetUser"],
      "Resource": "*",
      "Effect": "Allow"
    }
  ]
}
EOF
}

resource "ctyun_iam_user_policy_attachment" "test" {
  user       = ctyun_iam_user.test.name
  policy_arn = ctyun_iam_policy.test.arn
}

`, userName, policyName)
}

func testAccAccessKeyConfig_lastUsedPopulated(userName, policyName string) string {
	return testAccAccessKeyConfig_lastUsedPrerequisites(userName, policyName) + `
resource "ctyun_iam_access_key" "test" {
  user       = ctyun_iam_user.test.name
  depends_on = [ctyun_iam_user_policy_attachment.test]
}
`
}
