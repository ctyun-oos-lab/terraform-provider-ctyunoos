// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	sdkretry "github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/pquerna/otp/totp"
)

// TestResourceUserTagValidation verifies the tag validation behavior of the final assembled ctyun_iam_user, ensuring that ResourceUser has the user-specific tags validator attached and correctly rejects count, length, and character-set boundary violations.
func TestUnitIAMUser_resourceUserTagValidation(t *testing.T) {
	t.Parallel()

	resource := tfiam.ResourceUser()
	validate := resource.Schema[names.AttrTags].ValidateDiagFunc
	if validate == nil {
		t.Fatal("expected tags validation")
	}

	maxTags := map[string]any{}
	for i := range 10 {
		maxTags[fmt.Sprintf("key%d", i)] = "value"
	}

	tooManyTags := map[string]any{}
	for i := range 11 {
		tooManyTags[fmt.Sprintf("key%d", i)] = "value"
	}

	testCases := []struct {
		name    string
		value   map[string]any
		wantErr bool
	}{
		{name: "valid", value: map[string]any{"Tag Key_.:/=+-@123": "Tag Value_.:/=+-@123"}, wantErr: false},
		{name: "max_tags", value: maxTags, wantErr: false},
		{name: "empty_key", value: map[string]any{"": "value"}, wantErr: true},
		{name: "key_too_long", value: map[string]any{strings.Repeat("a", 129): "value"}, wantErr: true},
		{name: "key_invalid_character", value: map[string]any{"bad!key": "value"}, wantErr: true},
		{name: "value_too_long", value: map[string]any{"key": strings.Repeat("a", 257)}, wantErr: true},
		{name: "value_invalid_character", value: map[string]any{"key": "bad!value"}, wantErr: true},
		{name: "too_many_tags", value: tooManyTags, wantErr: true},
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

// TC-IAM-USER-01-Basic verifies the basic CRUD flow of an IAM User: create the user, read key attributes, import into state, and confirm the resource is destroyed after the test.
func TestAccIAMUser_basic(t *testing.T) {
	ctx := acctest.Context(t)
	var user awstypes.User
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_user.user"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
					testAccCheckUserAttributes(&user, rName),
					resource.TestCheckResourceAttr(resourceName, names.AttrID, rName),
					resource.TestCheckResourceAttr(resourceName, names.AttrName, rName),
					testAccCheckUserARN(&user),
					testAccCheckUserUniqueID(&user),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{names.AttrForceDestroy},
			},
		},
	})
}

// TC-IAM-USER-01-Basic verifies the valid-special-character user name scenario for an IAM User: a name containing -, +, _, ., and @ can be created, read, and destroyed successfully.
func TestAccIAMUser_nameSpecialCharacters(t *testing.T) {
	ctx := acctest.Context(t)
	var user awstypes.User
	resourceName := "ctyun_iam_user.user"
	rName := acctest.RandomWithPrefix(t, "tf-acc-test-+_.@")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
					testAccCheckUserAttributes(&user, rName),
					resource.TestCheckResourceAttr(resourceName, names.AttrName, rName),
				),
			},
		},
	})
}

// TC-IAM-USER-01-Basic verifies the minimum user name length boundary for an IAM User: a name of length 1 can be created, read, and destroyed successfully.
func TestAccIAMUser_nameLengthValidation_min(t *testing.T) {
	ctx := acctest.Context(t)
	var user awstypes.User
	resourceName := "ctyun_iam_user.user"
	nameMinLength := "a"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_basic(nameMinLength),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
					testAccCheckUserAttributes(&user, nameMinLength),
					resource.TestCheckResourceAttr(resourceName, names.AttrName, nameMinLength),
				),
			},
		},
	})
}

// TC-IAM-USER-01-Basic verifies the maximum user name length boundary for an IAM User: a name of length 64 can be created, read, and destroyed successfully.
func TestAccIAMUser_nameLengthValidation_max(t *testing.T) {
	ctx := acctest.Context(t)
	var user awstypes.User
	resourceName := "ctyun_iam_user.user"
	nameMaxLength := strings.Repeat("a", 64)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_basic(nameMaxLength),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
					testAccCheckUserAttributes(&user, nameMaxLength),
					resource.TestCheckResourceAttr(resourceName, names.AttrName, nameMaxLength),
				),
			},
		},
	})
}

// TC-IAM-USER-01-Basic verifies the invalid empty user name scenario for an IAM User: a name of length 0 fails validation in ACC.
func TestAccIAMUser_nameLengthValidation_empty(t *testing.T) {
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserConfig_basic(""),
				ExpectError: regexache.MustCompile(`expected length of name to be in the range \(1 - 64\), got`),
			},
		},
	})
}

// TC-IAM-USER-01-Basic verifies the invalid over-length user name scenario for an IAM User: a name of length 65 fails validation in ACC.
func TestAccIAMUser_nameLengthValidation_tooLong(t *testing.T) {
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserConfig_basic(strings.Repeat("a", 65)),
				ExpectError: regexache.MustCompile(`expected length of name to be in the range \(1 - 64\), got`),
			},
		},
	})
}

// TC-IAM-USER-01-Basic verifies the invalid user name scenario containing a space for an IAM User: a name containing a space fails validation in ACC.
func TestAccIAMUser_nameInvalidCharacters_space(t *testing.T) {
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserConfig_basic("invalid user"),
				ExpectError: regexache.MustCompile(`must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs`),
			},
		},
	})
}

// TC-IAM-USER-01-Basic verifies the invalid user name scenario containing illegal characters for an IAM User: a name containing illegal characters fails validation in ACC.
func TestAccIAMUser_nameInvalidCharacters_invalidCharacter(t *testing.T) {
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserConfig_basic("invalid_user!"),
				ExpectError: regexache.MustCompile(`must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs`),
			},
		},
	})
}

// TC-IAM-USER-02-Rename verifies the basic rename scenario of an IAM User: after the name changes, the remote read-back stays consistent with the name in state.
func TestAccIAMUser_nameChange_basic(t *testing.T) {
	ctx := acctest.Context(t)
	var user awstypes.User
	name1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	name2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_user.user"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_basic(name1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
					testAccCheckUserAttributes(&user, name1),
					resource.TestCheckResourceAttr(resourceName, names.AttrName, name1),
				),
			},
			{
				Config: testAccUserConfig_basic(name2),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
					testAccCheckUserAttributes(&user, name2),
					resource.TestCheckResourceAttr(resourceName, names.AttrName, name2),
				),
			},
		},
	})
}

// TC-IAM-USER-02-Rename verifies the case-only rename scenario of an IAM User: after changing only letter casing, the remote read-back stays consistent with the name in state.
func TestAccIAMUser_nameChange_caseDifference(t *testing.T) {
	ctx := acctest.Context(t)
	var user awstypes.User
	name1 := acctest.RandomWithPrefix(t, "tf-acc-test-user")
	name2 := strings.ToUpper(name1)
	resourceName := "ctyun_iam_user.user"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_basic(name1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
					testAccCheckUserAttributes(&user, name1),
					resource.TestCheckResourceAttr(resourceName, names.AttrName, name1),
				),
			},
			{
				Config: testAccUserConfig_basic(name2),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
					testAccCheckUserAttributes(&user, name2),
					resource.TestCheckResourceAttr(resourceName, names.AttrName, name2),
				),
			},
		},
	})
}

// TC-IAM-USER-02-Rename verifies the special-character rename scenario of an IAM User: after renaming to a valid name containing special characters, the remote read-back stays consistent with the name in state.
func TestAccIAMUser_nameChange_specialCharacterDifference(t *testing.T) {
	ctx := acctest.Context(t)
	var user awstypes.User
	name1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	name2 := acctest.RandomWithPrefix(t, "tf-acc-test-+_.@")
	resourceName := "ctyun_iam_user.user"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_basic(name1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
					testAccCheckUserAttributes(&user, name1),
					resource.TestCheckResourceAttr(resourceName, names.AttrName, name1),
				),
			},
			{
				Config: testAccUserConfig_basic(name2),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
					testAccCheckUserAttributes(&user, name2),
					resource.TestCheckResourceAttr(resourceName, names.AttrName, name2),
				),
			},
		},
	})
}

// TC-IAM-USER-02-Rename verifies the duplicate-name conflict scenario of an IAM User: renaming to a name already taken by another user fails in ACC.
func TestAccIAMUser_nameChange_duplicateName(t *testing.T) {
	ctx := acctest.Context(t)
	rName1 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	rName2 := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_user.user"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_duplicateNameBase(rName1, rName2),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, new(awstypes.User)),
					resource.TestCheckResourceAttr(resourceName, names.AttrName, rName1),
				),
			},
			{
				Config:      testAccUserConfig_duplicateNameConflict(rName1, rName2),
				ExpectError: regexache.MustCompile(`(?i)(EntityAlreadyExists|already exists|duplicate)`),
			},
		},
	})
}

// TC-IAM-USER-03 Tags verifies IAM User tag creation, add/remove/update synchronization, and the idempotence of re-planning the same configuration.
func TestAccIAMUser_tagsLifecycle(t *testing.T) {
	ctx := acctest.Context(t)
	var user awstypes.User
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_user.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_1_0),
		},
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				ConfigDirectory: config.StaticDirectory("testdata/User/tags/"),
				ConfigVariables: config.Variables{
					acctest.CtRName: config.StringVariable(rName),
					acctest.CtResourceTags: config.MapVariable(map[string]config.Variable{
						acctest.CtKey1: config.StringVariable(acctest.CtValue1),
					}),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(resourceName, tfjsonpath.New(names.AttrTags), knownvalue.MapExact(map[string]knownvalue.Check{
						acctest.CtKey1: knownvalue.StringExact(acctest.CtValue1),
					})),
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
						plancheck.ExpectKnownValue(resourceName, tfjsonpath.New(names.AttrTags), knownvalue.MapExact(map[string]knownvalue.Check{
							acctest.CtKey1: knownvalue.StringExact(acctest.CtValue1),
						})),
					},
				},
			},
			{
				ConfigDirectory: config.StaticDirectory("testdata/User/tags/"),
				ConfigVariables: config.Variables{
					acctest.CtRName: config.StringVariable(rName),
					acctest.CtResourceTags: config.MapVariable(map[string]config.Variable{
						acctest.CtKey1: config.StringVariable(acctest.CtValue1Updated),
						acctest.CtKey2: config.StringVariable(acctest.CtValue2),
					}),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(resourceName, tfjsonpath.New(names.AttrTags), knownvalue.MapExact(map[string]knownvalue.Check{
						acctest.CtKey1: knownvalue.StringExact(acctest.CtValue1Updated),
						acctest.CtKey2: knownvalue.StringExact(acctest.CtValue2),
					})),
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue(resourceName, tfjsonpath.New(names.AttrTags), knownvalue.MapExact(map[string]knownvalue.Check{
							acctest.CtKey1: knownvalue.StringExact(acctest.CtValue1Updated),
							acctest.CtKey2: knownvalue.StringExact(acctest.CtValue2),
						})),
					},
				},
			},
			{
				ConfigDirectory: config.StaticDirectory("testdata/User/tags/"),
				ConfigVariables: config.Variables{
					acctest.CtRName: config.StringVariable(rName),
					acctest.CtResourceTags: config.MapVariable(map[string]config.Variable{
						acctest.CtKey2: config.StringVariable(acctest.CtValue2),
					}),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(resourceName, tfjsonpath.New(names.AttrTags), knownvalue.MapExact(map[string]knownvalue.Check{
						acctest.CtKey2: knownvalue.StringExact(acctest.CtValue2),
					})),
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue(resourceName, tfjsonpath.New(names.AttrTags), knownvalue.MapExact(map[string]knownvalue.Check{
							acctest.CtKey2: knownvalue.StringExact(acctest.CtValue2),
						})),
					},
				},
			},
			{
				ConfigDirectory: config.StaticDirectory("testdata/User/tags/"),
				ConfigVariables: config.Variables{
					acctest.CtRName: config.StringVariable(rName),
					acctest.CtResourceTags: config.MapVariable(map[string]config.Variable{
						acctest.CtKey2: config.StringVariable(acctest.CtValue2),
					}),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(resourceName, tfjsonpath.New(names.AttrTags), knownvalue.MapExact(map[string]knownvalue.Check{
						acctest.CtKey2: knownvalue.StringExact(acctest.CtValue2),
					})),
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionNoop),
						plancheck.ExpectKnownValue(resourceName, tfjsonpath.New(names.AttrTags), knownvalue.MapExact(map[string]knownvalue.Check{
							acctest.CtKey2: knownvalue.StringExact(acctest.CtValue2),
						})),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

// TC-IAM-USER-03 Tags verifies that after tags are modified directly on the remote side, Terraform detects the drift and
// converges the tags back to the declared configuration on the next apply.
// This covers the back-fill scenario where the remote already has 10 tags while Terraform declares only 1 tag; after apply,
// only the 1 tag from the configuration is expected to remain.
func TestAccIAMUser_tagsExternalDriftReconcile(t *testing.T) {
	ctx := acctest.Context(t)
	var user awstypes.User
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_user.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_1_0),
		},
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				ConfigDirectory: config.StaticDirectory("testdata/User/tags/"),
				ConfigVariables: config.Variables{
					acctest.CtRName: config.StringVariable(rName),
					acctest.CtResourceTags: config.MapVariable(map[string]config.Variable{
						acctest.CtKey1: config.StringVariable(acctest.CtValue1),
					}),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
					testAccCheckUserTagExternalDriftToMax(ctx, t, &user),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				ConfigDirectory: config.StaticDirectory("testdata/User/tags/"),
				ConfigVariables: config.Variables{
					acctest.CtRName: config.StringVariable(rName),
					acctest.CtResourceTags: config.MapVariable(map[string]config.Variable{
						acctest.CtKey1: config.StringVariable(acctest.CtValue1),
					}),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(resourceName, tfjsonpath.New(names.AttrTags), knownvalue.MapExact(map[string]knownvalue.Check{
						acctest.CtKey1: knownvalue.StringExact(acctest.CtValue1),
					})),
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionNoop),
					},
				},
			},
			{
				ConfigDirectory: config.StaticDirectory("testdata/User/tags/"),
				ConfigVariables: config.Variables{
					acctest.CtRName: config.StringVariable(rName),
					acctest.CtResourceTags: config.MapVariable(map[string]config.Variable{
						acctest.CtKey1: config.StringVariable(acctest.CtValue1),
					}),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
					testAccCheckUserTagExternalDriftToMax(ctx, t, &user),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				ConfigDirectory: config.StaticDirectory("testdata/User/tags/"),
				ConfigVariables: config.Variables{
					acctest.CtRName: config.StringVariable(rName),
					acctest.CtResourceTags: config.MapVariable(map[string]config.Variable{
						acctest.CtKey1: config.StringVariable(acctest.CtValue1),
					}),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(resourceName, tfjsonpath.New(names.AttrTags), knownvalue.MapExact(map[string]knownvalue.Check{
						acctest.CtKey1: knownvalue.StringExact(acctest.CtValue1),
					})),
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

// TC-IAM-USER-04 ForceDestroy AccessKey verifies that an IAM User with force_destroy enabled cleans up externally created
// access keys and completes user destruction.
// After destruction, GetUser is probed with the static AK/SK and is expected to return InvalidAccessKeyId, indicating the original access key is no longer valid.
func TestAccIAMUser_ForceDestroy_accessKey(t *testing.T) {
	ctx := acctest.Context(t)
	var user awstypes.User
	accessKey := &userAccessKey{}
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_user.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroyWithInvalidAccessKey(ctx, t, accessKey),
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_forceDestroy(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
					testAccCheckUserCreatesAccessKey(ctx, t, &user, accessKey),
				),
			},
		},
	})
}

// TC-IAM-USER-05 ForceDestroy LoginProfile verifies that an IAM User with force_destroy enabled cleans up an externally
// created login profile and completes user destruction.
// After destruction, GetLoginProfile is probed and is expected to return NoSuchEntity or 404, indicating the original login profile is no longer valid.
func TestAccIAMUser_ForceDestroy_loginProfile(t *testing.T) {
	ctx := acctest.Context(t)
	var user awstypes.User
	loginProfile := &userLoginProfileRef{}
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_user.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroyWithLoginProfile(ctx, t, loginProfile),
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_forceDestroy(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
					testAccCheckUserCreatesLoginProfile(ctx, t, &user, loginProfile),
				),
			},
		},
	})
}

// TC-IAM-USER-06 ForceDestroy MFA verifies that an IAM User with force_destroy enabled cleans up an externally created
// virtual MFA device bound to the user.
// After destruction, ListMFADevices and FindVirtualMFADeviceBySerialNumber are probed together, expecting both the binding and the device entity to be cleaned up.
func TestAccIAMUser_ForceDestroy_mfa(t *testing.T) {
	ctx := acctest.Context(t)
	var user awstypes.User
	mfaDevice := &userMFADeviceRef{}
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_user.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroyWithMFA(ctx, t, mfaDevice),
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_forceDestroy(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
					testAccCheckUserCreatesAndEnablesVirtualMFA(ctx, t, &user, mfaDevice),
				),
			},
		},
	})
}

// TC-IAM-USER-07 ForceDestroy Attachments verifies that an IAM User with force_destroy enabled cleans up externally created
// policy attachments and completes user destruction.
// After destruction, ListAttachedUserPolicies is probed, expecting the original attachments to be gone.
func TestAccIAMUser_ForceDestroy_attachments(t *testing.T) {
	ctx := acctest.Context(t)
	var user awstypes.User
	attachment := &userPolicyAttachment{}
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_user.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroyWithAttachment(ctx, t, attachment),
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_forceDestroy(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
					testAccCheckUserCreatesPolicyAttachment(ctx, t, &user, attachment),
				),
			},
		},
	})
}

// TC-IAM-USER-08 ForceDestroy GroupMembership verifies that an IAM User with force_destroy enabled cleans up externally
// created group memberships and completes user destruction.
// After destruction, ListGroupsForUser is probed, expecting the original group relationships to be gone.
func TestAccIAMUser_ForceDestroy_groupMembership(t *testing.T) {
	ctx := acctest.Context(t)
	var user awstypes.User
	membership := &userGroupMembershipRef{}
	rUserName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_user.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroyWithMembership(ctx, t, membership),
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_forceDestroy(rUserName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
					testAccCheckUserCreatesGroupMembership(ctx, t, &user, membership),
				),
			},
		},
	})
}

// TC-IAM-USER-09 Disappears deletes the IAM User outside Terraform via the IAM SDK, then verifies that refresh removes it from state and produces a Create plan.
func TestAccIAMUser_disappears(t *testing.T) {
	ctx := acctest.Context(t)
	var user awstypes.User
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_user.user"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
					testAccCheckUserDisappearsExternally(ctx, t, resourceName),
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
					testAccCheckUserRemovedFromState(resourceName),
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

func testAccCheckUserDisappearsExternally(ctx context.Context, t *testing.T, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		userName := rs.Primary.ID
		if userName == "" {
			return fmt.Errorf("IAM User (%s) has no name", resourceName)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		_, err := conn.DeleteUser(ctx, &iam.DeleteUserInput{UserName: aws.String(userName)})
		if err != nil {
			return fmt.Errorf("externally deleting IAM User (%s): %w", userName, err)
		}

		_, err = tfresource.RetryUntilNotFound(ctx, time.Minute, func(ctx context.Context) (any, error) {
			_, err := tfiam.FindUserByName(ctx, conn, userName)
			if retry.NotFound(err) {
				return nil, &retry.NotFoundError{LastError: err}
			}

			return nil, err
		})
		if err != nil {
			return fmt.Errorf("waiting for IAM User (%s) to disappear: %w", userName, err)
		}

		return nil
	}
}

func testAccCheckUserRemovedFromState(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if _, ok := s.RootModule().Resources[resourceName]; ok {
			return fmt.Errorf("IAM User resource (%s) is still present in state", resourceName)
		}

		return nil
	}
}

// TC-IAM-USER-10 Destroy Order Regression verifies that when an IAM User has both group memberships and policy attachments,
// the destroy order is stable with no residue.
// After destruction, ListGroupsForUser and ListAttachedUserPolicies are checked to confirm that both the memberships and attachments of the current UserName have been cleaned up.
func TestAccIAMUser_destroyOrder_membershipAndAttachment(t *testing.T) {
	ctx := acctest.Context(t)
	var user awstypes.User
	membership := &userGroupMembershipRef{}
	attachment := &userPolicyAttachment{}
	rUserName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	rGroupName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	rPolicyName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	resourceName := "ctyun_iam_user.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroyWithMembershipAndAttachment(ctx, t, membership, attachment),
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_destroyOrderMembershipAndAttachment(rUserName, rGroupName, rPolicyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserExists(ctx, t, resourceName, &user),
					testAccCheckUserGroupMembershipRecorded(ctx, t, rUserName, rGroupName, membership),
					testAccCheckUserPolicyAttachmentRecorded(ctx, t, rUserName, attachment),
				),
			},
		},
	})
}

func testAccCheckUserDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_iam_user" {
				continue
			}

			_, err := tfiam.FindUserByName(ctx, conn, rs.Primary.ID)
			if retry.NotFound(err) {
				continue
			}
			if err != nil {
				return err
			}

			return fmt.Errorf("IAM User %s still exists", rs.Primary.ID)
		}

		return nil
	}
}

func testAccCheckUserDestroyWithInvalidAccessKey(ctx context.Context, t *testing.T, accessKey *userAccessKey) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if err := testAccCheckUserDestroy(ctx, t)(s); err != nil {
			return err
		}

		if accessKey == nil || accessKey.ID == "" || accessKey.Secret == "" {
			return nil
		}

		meta := acctest.ProviderMeta(ctx, t)
		endpoint := os.Getenv("CTYUNOOS_ENDPOINT_IAM")
		if endpoint == "" {
			return fmt.Errorf("CTYUNOOS_ENDPOINT_IAM must be set to verify invalid access key after force destroy")
		}

		err := tfiam.TestProbeGetUserWithStaticCredentials(ctx, meta, endpoint, accessKey.ID, accessKey.Secret)
		if err == nil {
			return fmt.Errorf("expected access key to be invalid after force destroy")
		}
		if !tfawserr.ErrCodeEquals(err, "InvalidAccessKeyId") {
			return fmt.Errorf("expected InvalidAccessKeyId after force destroy, got %w", err)
		}

		return nil
	}
}

func testAccCheckUserDestroyWithLoginProfile(ctx context.Context, t *testing.T, loginProfile *userLoginProfileRef) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if err := testAccCheckUserDestroy(ctx, t)(s); err != nil {
			return err
		}

		if loginProfile == nil || loginProfile.UserName == "" {
			return nil
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		_, err := tfresource.RetryUntilNotFound(ctx, time.Minute, func(ctx context.Context) (any, error) {
			_, err := conn.GetLoginProfile(ctx, &iam.GetLoginProfileInput{
				UserName: aws.String(loginProfile.UserName),
			})

			if errs.IsA[*awstypes.NoSuchEntityException](err) ||
				tfawserr.ErrCodeEquals(err, (&awstypes.NoSuchEntityException{}).ErrorCode()) ||
				tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
				return nil, &retry.NotFoundError{LastError: err}
			}

			return nil, err
		})
		if err != nil {
			return fmt.Errorf("IAM User Login Profile (%s) still exists", loginProfile.UserName)
		}

		return nil
	}
}

func testAccCheckUserDestroyWithMFA(ctx context.Context, t *testing.T, mfaDevice *userMFADeviceRef) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if err := testAccCheckUserDestroy(ctx, t)(s); err != nil {
			return err
		}

		if mfaDevice == nil || mfaDevice.UserName == "" || mfaDevice.SerialNumber == "" {
			return nil
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		err := sdkretry.RetryContext(ctx, time.Minute, func() *sdkretry.RetryError {
			output, err := conn.ListMFADevices(ctx, &iam.ListMFADevicesInput{
				UserName: aws.String(mfaDevice.UserName),
			})
			if err != nil {
				if errs.IsA[*awstypes.NoSuchEntityException](err) ||
					tfawserr.ErrCodeEquals(err, (&awstypes.NoSuchEntityException{}).ErrorCode()) ||
					tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
					return nil
				}

				return sdkretry.RetryableError(fmt.Errorf("listing MFA devices for IAM User (%s): %w", mfaDevice.UserName, err))
			}

			for _, device := range output.MFADevices {
				if aws.ToString(device.SerialNumber) == mfaDevice.SerialNumber {
					return sdkretry.RetryableError(fmt.Errorf("MFA device %s still attached to IAM User %s", mfaDevice.SerialNumber, mfaDevice.UserName))
				}
			}

			return nil
		})
		if err != nil {
			return err
		}

		err = sdkretry.RetryContext(ctx, time.Minute, func() *sdkretry.RetryError {
			vMFA, err := tfiam.FindVirtualMFADeviceBySerialNumber(ctx, conn, mfaDevice.SerialNumber)
			if retry.NotFound(err) {
				return nil
			}
			if err != nil {
				return sdkretry.RetryableError(err)
			}

			if vMFA != nil && vMFA.User != nil && aws.ToString(vMFA.User.UserName) == mfaDevice.UserName {
				return sdkretry.RetryableError(fmt.Errorf("IAM Virtual MFA Device (%s) still attached to IAM User %s", mfaDevice.SerialNumber, mfaDevice.UserName))
			}

			return nil
		})
		if err != nil {
			return err
		}

		_, err = conn.DeleteVirtualMFADevice(ctx, &iam.DeleteVirtualMFADeviceInput{SerialNumber: aws.String(mfaDevice.SerialNumber)})
		if err != nil && !errs.IsA[*awstypes.NoSuchEntityException](err) && !tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
			return fmt.Errorf("deleting IAM Virtual MFA Device (%s): %w", mfaDevice.SerialNumber, err)
		}

		_, err = tfiam.FindVirtualMFADeviceBySerialNumber(ctx, conn, mfaDevice.SerialNumber)
		if retry.NotFound(err) || errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading IAM Virtual MFA Device (%s): %w", mfaDevice.SerialNumber, err)
		}

		return fmt.Errorf("IAM Virtual MFA Device %s still exists", mfaDevice.SerialNumber)
	}
}

func testAccCheckUserDestroyWithAttachment(ctx context.Context, t *testing.T, attachment *userPolicyAttachment) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if err := testAccCheckUserDestroy(ctx, t)(s); err != nil {
			return err
		}

		if attachment == nil || attachment.UserName == "" || attachment.PolicyARN == "" {
			return nil
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		input := &iam.ListAttachedUserPoliciesInput{UserName: aws.String(attachment.UserName)}
		policies, err := tfiam.FindAttachedUserPolicies(ctx, conn, input, func(policy awstypes.AttachedPolicy) bool {
			return aws.ToString(policy.PolicyArn) == attachment.PolicyARN
		})
		if err != nil && !retry.NotFound(err) && !errs.IsA[*awstypes.NoSuchEntityException](err) && !tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
			return fmt.Errorf("listing attached user policies for IAM User (%s): %w", attachment.UserName, err)
		}
		if len(policies) > 0 {
			return fmt.Errorf("policy attachment %s for IAM User %s still exists", attachment.PolicyARN, attachment.UserName)
		}

		_, err = conn.DeletePolicy(ctx, &iam.DeletePolicyInput{
			PolicyArn: aws.String(attachment.PolicyARN),
		})
		if err != nil && !errs.IsA[*awstypes.NoSuchEntityException](err) && !tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
			return fmt.Errorf("deleting IAM Policy (%s): %w", attachment.PolicyARN, err)
		}

		_, err = tfiam.FindPolicyByARN(ctx, conn, attachment.PolicyARN)
		if err != nil {
			if !retry.NotFound(err) && !errs.IsA[*awstypes.NoSuchEntityException](err) && !tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
				return fmt.Errorf("reading IAM Policy (%s): %w", attachment.PolicyARN, err)
			}
			return nil
		}

		return fmt.Errorf("IAM Policy %s still exists", attachment.PolicyARN)
	}
}

func testAccCheckUserDestroyWithMembership(ctx context.Context, t *testing.T, membership *userGroupMembershipRef) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if err := testAccCheckUserDestroy(ctx, t)(s); err != nil {
			return err
		}

		if membership == nil || membership.UserName == "" || membership.GroupName == "" {
			return nil
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		input := &iam.ListGroupsForUserInput{UserName: aws.String(membership.UserName)}
		foundGroups := 0
		err := tfiam.ListGroupsForUserPages(ctx, conn, input, func(page *iam.ListGroupsForUserOutput, lastPage bool) bool {
			for _, group := range page.Groups {
				if aws.ToString(group.GroupName) == membership.GroupName {
					foundGroups++
				}
			}
			return !lastPage
		})
		if err != nil && !retry.NotFound(err) && !errs.IsA[*awstypes.NoSuchEntityException](err) && !tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
			return fmt.Errorf("listing groups for IAM User (%s): %w", membership.UserName, err)
		}
		if foundGroups > 0 {
			return fmt.Errorf("group membership %s for IAM User %s still exists", membership.GroupName, membership.UserName)
		}

		_, err = conn.DeleteGroup(ctx, &iam.DeleteGroupInput{GroupName: aws.String(membership.GroupName)})
		if err != nil && !errs.IsA[*awstypes.NoSuchEntityException](err) && !tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
			return fmt.Errorf("deleting IAM Group (%s): %w", membership.GroupName, err)
		}

		_, err = tfiam.FindGroupByName(ctx, conn, membership.GroupName)
		if retry.NotFound(err) || errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading IAM Group (%s): %w", membership.GroupName, err)
		}

		return fmt.Errorf("IAM Group %s still exists", membership.GroupName)
	}
}

func testAccCheckUserDestroyWithMembershipAndAttachment(ctx context.Context, t *testing.T, membership *userGroupMembershipRef, attachment *userPolicyAttachment) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		if err := testAccCheckUserDestroy(ctx, t)(s); err != nil {
			return err
		}

		if membership != nil && membership.UserName != "" {
			input := &iam.ListGroupsForUserInput{UserName: aws.String(membership.UserName)}
			foundGroups := 0
			err := tfiam.ListGroupsForUserPages(ctx, conn, input, func(page *iam.ListGroupsForUserOutput, lastPage bool) bool {
				foundGroups += len(page.Groups)
				return !lastPage
			})
			if err != nil && !retry.NotFound(err) && !errs.IsA[*awstypes.NoSuchEntityException](err) && !tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
				return fmt.Errorf("listing groups for IAM User (%s): %w", membership.UserName, err)
			}
			if foundGroups > 0 {
				return fmt.Errorf("group membership for IAM User %s still exists", membership.UserName)
			}
		}

		if attachment != nil && attachment.UserName != "" {
			input := &iam.ListAttachedUserPoliciesInput{UserName: aws.String(attachment.UserName)}
			policies, err := tfiam.FindAttachedUserPolicies(ctx, conn, input, func(policy awstypes.AttachedPolicy) bool {
				return aws.ToString(policy.PolicyArn) == attachment.PolicyARN
			})
			if err != nil {
				if !retry.NotFound(err) && !errs.IsA[*awstypes.NoSuchEntityException](err) && !tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
					return fmt.Errorf("listing attached user policies for IAM User (%s): %w", attachment.UserName, err)
				}
			} else if len(policies) > 0 {
				return fmt.Errorf("policy attachment %s for IAM User %s still exists", attachment.PolicyARN, attachment.UserName)
			}
		}

		return nil
	}
}

func testAccCheckUserExists(ctx context.Context, t *testing.T, n string, v *awstypes.User) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("no IAM User ID is set")
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		output, err := tfiam.FindUserByName(ctx, conn, rs.Primary.ID)
		if err != nil {
			return err
		}

		*v = *output
		return nil
	}
}

func testAccCheckUserAttributes(user *awstypes.User, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if aws.ToString(user.UserName) != name {
			return fmt.Errorf("bad remote name: %s", aws.ToString(user.UserName))
		}

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_iam_user" {
				continue
			}

			if rs.Primary.ID != name {
				return fmt.Errorf("bad state id: got %s, want %s", rs.Primary.ID, name)
			}

			stateName := rs.Primary.Attributes[names.AttrName]
			if stateName != name {
				return fmt.Errorf("bad state name: got %s, want %s", stateName, name)
			}

			return nil
		}

		return fmt.Errorf("ctyun_iam_user not found in state")
	}
}

func testAccCheckUserARN(user *awstypes.User) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_iam_user" {
				continue
			}

			got := rs.Primary.Attributes[names.AttrARN]
			want := aws.ToString(user.Arn)
			if got != want {
				return fmt.Errorf("bad arn: got %s, want %s", got, want)
			}
			return nil
		}

		return fmt.Errorf("ctyun_iam_user not found in state")
	}
}

func testAccCheckUserUniqueID(user *awstypes.User) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_iam_user" {
				continue
			}

			got := rs.Primary.Attributes["unique_id"]
			want := aws.ToString(user.UserId)
			if got != want {
				return fmt.Errorf("bad unique_id: got %s, want %s", got, want)
			}
			return nil
		}

		return fmt.Errorf("ctyun_iam_user not found in state")
	}
}

type userAccessKey struct {
	ID     string
	Secret string
}

type userLoginProfileRef struct {
	UserName string
}

type userMFADeviceRef struct {
	SerialNumber string
	UserName     string
}

type userGroupMembershipRef struct {
	UserName  string
	GroupName string
}

type userPolicyAttachment struct {
	PolicyARN  string
	UserName   string
	PolicyName string
}

func testAccCheckUserGroupMembershipRecorded(ctx context.Context, t *testing.T, userName, groupName string, membership *userGroupMembershipRef) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		_, err := tfiam.FindGroupForUserByTwoPartKey(ctx, conn, userName, groupName)
		if err != nil {
			return err
		}

		if membership != nil {
			membership.UserName = userName
			membership.GroupName = groupName
		}

		return nil
	}
}

func testAccCheckUserPolicyAttachmentRecorded(ctx context.Context, t *testing.T, userName string, attachment *userPolicyAttachment) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		input := &iam.ListAttachedUserPoliciesInput{
			UserName: aws.String(userName),
		}
		policies, err := tfiam.FindAttachedUserPolicies(ctx, conn, input, func(policy awstypes.AttachedPolicy) bool {
			return true
		})
		if err != nil {
			return err
		}
		if len(policies) == 0 {
			return fmt.Errorf("no attached policy found for IAM User %s", userName)
		}

		if attachment != nil {
			attachment.PolicyARN = aws.ToString(policies[0].PolicyArn)
			attachment.UserName = userName
		}

		return nil
	}
}

func testAccCheckUserTagExternalDriftToMax(ctx context.Context, t *testing.T, user *awstypes.User) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		_, err := conn.UntagUser(ctx, &iam.UntagUserInput{
			UserName: user.UserName,
			TagKeys:  []string{acctest.CtKey1},
		})
		if err != nil {
			return fmt.Errorf("externally untagging IAM User (%s): %w", aws.ToString(user.UserName), err)
		}

		tags := make([]awstypes.Tag, 0, 10)
		for i := range 10 {
			tags = append(tags, awstypes.Tag{
				Key:   aws.String(fmt.Sprintf("drift-key-%d", i)),
				Value: aws.String(fmt.Sprintf("drift-value-%d", i)),
			})
		}

		_, err = conn.TagUser(ctx, &iam.TagUserInput{
			UserName: user.UserName,
			Tags:     tags,
		})
		if err != nil {
			return fmt.Errorf("externally tagging IAM User (%s) to max tags: %w", aws.ToString(user.UserName), err)
		}

		return nil
	}
}

func testAccCheckUserCreatesAccessKey(ctx context.Context, t *testing.T, user *awstypes.User, accessKey *userAccessKey) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		output, err := conn.CreateAccessKey(ctx, &iam.CreateAccessKeyInput{
			UserName: user.UserName,
		})
		if err != nil {
			return fmt.Errorf("creating IAM User (%s) Access Key: %w", aws.ToString(user.UserName), err)
		}

		if accessKey != nil && output.AccessKey != nil {
			accessKey.ID = aws.ToString(output.AccessKey.AccessKeyId)
			accessKey.Secret = aws.ToString(output.AccessKey.SecretAccessKey)
		}

		return nil
	}
}

func testAccCheckUserCreatesLoginProfile(ctx context.Context, t *testing.T, user *awstypes.User, loginProfile *userLoginProfileRef) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		policy, err := tfiam.FindAccountPasswordPolicy(ctx, conn)
		if retry.NotFound(err) {
			policy = nil
		} else if err != nil {
			return fmt.Errorf("reading IAM Account Password Policy for User (%s) login profile: %w", aws.ToString(user.UserName), err)
		}

		password, err := tfiam.GeneratePasswordWithPolicy(policy)
		if err != nil {
			return fmt.Errorf("generating IAM User (%s) login profile password: %w", aws.ToString(user.UserName), err)
		}

		_, err = conn.CreateLoginProfile(ctx, &iam.CreateLoginProfileInput{
			Password:              aws.String(password),
			PasswordResetRequired: false,
			UserName:              user.UserName,
		})
		if err != nil {
			return fmt.Errorf("creating IAM User (%s) Login Profile: %w", aws.ToString(user.UserName), err)
		}

		if loginProfile != nil {
			loginProfile.UserName = aws.ToString(user.UserName)
		}

		return nil
	}
}

func testAccCheckUserCreatesGroupMembership(ctx context.Context, t *testing.T, user *awstypes.User, membership *userGroupMembershipRef) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		groupName := acctest.RandomWithPrefix(t, "tf-acc-test-group")

		groupOutput, err := conn.CreateGroup(ctx, &iam.CreateGroupInput{
			GroupName: aws.String(groupName),
		})
		if err != nil {
			return fmt.Errorf("creating IAM Group (%s): %w", groupName, err)
		}
		if groupOutput == nil || groupOutput.Group == nil || aws.ToString(groupOutput.Group.GroupName) == "" {
			return fmt.Errorf("creating IAM Group (%s): empty result", groupName)
		}

		_, err = conn.AddUserToGroup(ctx, &iam.AddUserToGroupInput{
			GroupName: aws.String(groupName),
			UserName:  user.UserName,
		})
		if err != nil {
			return fmt.Errorf("adding IAM User (%s) to Group (%s): %w", aws.ToString(user.UserName), groupName, err)
		}

		_, err = tfiam.FindGroupForUserByTwoPartKey(ctx, conn, aws.ToString(user.UserName), groupName)
		if err != nil {
			return fmt.Errorf("reading IAM User Group Membership (%s/%s): %w", aws.ToString(user.UserName), groupName, err)
		}

		if membership != nil {
			membership.UserName = aws.ToString(user.UserName)
			membership.GroupName = groupName
		}

		return nil
	}
}

func testAccCheckUserCreatesPolicyAttachment(ctx context.Context, t *testing.T, user *awstypes.User, attachment *userPolicyAttachment) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		policyName := acctest.RandomWithPrefix(t, "tf-acc-test-policy")

		output, err := conn.CreatePolicy(ctx, &iam.CreatePolicyInput{
			Description: aws.String("A test policy"),
			PolicyDocument: aws.String(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "iam:ChangePassword"
      ],
      "Resource": "*",
      "Effect": "Allow"
    }
  ]
}`),
			PolicyName: aws.String(policyName),
		})
		if err != nil {
			return fmt.Errorf("creating IAM Policy (%s): %w", policyName, err)
		}
		if output == nil || output.Policy == nil || aws.ToString(output.Policy.Arn) == "" {
			return fmt.Errorf("creating IAM Policy (%s): empty result", policyName)
		}

		policyARN := aws.ToString(output.Policy.Arn)
		_, err = conn.AttachUserPolicy(ctx, &iam.AttachUserPolicyInput{
			PolicyArn: aws.String(policyARN),
			UserName:  user.UserName,
		})
		if err != nil {
			return fmt.Errorf("attaching IAM Policy (%s) to IAM User (%s): %w", policyARN, aws.ToString(user.UserName), err)
		}

		_, err = tfiam.FindAttachedUserPolicyByTwoPartKey(ctx, conn, aws.ToString(user.UserName), policyARN)
		if err != nil {
			return fmt.Errorf("reading IAM User Policy Attachment (%s:%s): %w", aws.ToString(user.UserName), policyARN, err)
		}

		if attachment != nil {
			attachment.PolicyARN = policyARN
			attachment.UserName = aws.ToString(user.UserName)
			attachment.PolicyName = policyName
		}

		return nil
	}
}

func testAccCheckUserCreatesAndEnablesVirtualMFA(ctx context.Context, t *testing.T, user *awstypes.User, mfaDevice *userMFADeviceRef) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		deviceName := acctest.RandomWithPrefix(t, "tf-acc-test-mfa")

		output, err := conn.CreateVirtualMFADevice(ctx, &iam.CreateVirtualMFADeviceInput{
			VirtualMFADeviceName: aws.String(deviceName),
		})
		if err != nil {
			return fmt.Errorf("creating IAM Virtual MFA Device (%s): %w", deviceName, err)
		}
		if output == nil || output.VirtualMFADevice == nil {
			return fmt.Errorf("creating IAM Virtual MFA Device (%s): empty result", deviceName)
		}

		serialNumber := aws.ToString(output.VirtualMFADevice.SerialNumber)
		if serialNumber == "" || len(output.VirtualMFADevice.Base32StringSeed) == 0 {
			return fmt.Errorf("creating IAM Virtual MFA Device (%s): missing serial number or seed", deviceName)
		}

		secret, err := normalizeVirtualMFADeviceSeed(output.VirtualMFADevice.Base32StringSeed)
		if err != nil {
			return fmt.Errorf("normalizing IAM Virtual MFA Device (%s) base_32_string_seed: %w", serialNumber, err)
		}

		now := time.Now().UTC()
		firstWindow := now.Truncate(30 * time.Second).Add(30 * time.Second)
		time.Sleep(time.Until(firstWindow.Add(1 * time.Second)))

		firstObserved := time.Now().UTC()
		code1, err := totp.GenerateCode(secret, firstObserved)
		if err != nil {
			return fmt.Errorf("generating first TOTP for IAM Virtual MFA Device (%s): %w", serialNumber, err)
		}

		secondWindow := firstObserved.Truncate(30 * time.Second).Add(30 * time.Second)
		time.Sleep(time.Until(secondWindow.Add(1 * time.Second)))

		code2, err := totp.GenerateCode(secret, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("generating second TOTP for IAM Virtual MFA Device (%s): %w", serialNumber, err)
		}

		_, err = conn.EnableMFADevice(ctx, &iam.EnableMFADeviceInput{
			AuthenticationCode1: aws.String(code1),
			AuthenticationCode2: aws.String(code2),
			SerialNumber:        aws.String(serialNumber),
			UserName:            user.UserName,
		})
		if err != nil {
			return fmt.Errorf("enabling IAM Virtual MFA Device (%s) for IAM User (%s): %w", serialNumber, aws.ToString(user.UserName), err)
		}

		verification, err := conn.ListMFADevices(ctx, &iam.ListMFADevicesInput{
			UserName: user.UserName,
		})
		if err != nil {
			return fmt.Errorf("listing MFA devices for IAM User (%s): %w", aws.ToString(user.UserName), err)
		}

		found := false
		for _, device := range verification.MFADevices {
			if aws.ToString(device.SerialNumber) == serialNumber {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("IAM Virtual MFA Device (%s) not attached to IAM User (%s)", serialNumber, aws.ToString(user.UserName))
		}

		if mfaDevice != nil {
			mfaDevice.SerialNumber = serialNumber
			mfaDevice.UserName = aws.ToString(user.UserName)
		}

		return nil
	}
}

func normalizeVirtualMFADeviceSeed(seed []byte) (string, error) {
	if len(seed) == 0 {
		return "", fmt.Errorf("empty seed")
	}

	candidates := []string{
		string(seed),
		base64.StdEncoding.EncodeToString(seed),
		base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(seed),
	}

	for _, candidate := range candidates {
		candidate = strings.ToUpper(strings.TrimRight(strings.TrimSpace(candidate), "="))
		if candidate == "" {
			continue
		}
		if _, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("seed is not a supported Base32 representation")
}

func testAccUserConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "user" {
  name = %[1]q
}
`, rName)
}

func testAccUserConfig_forceDestroy(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  force_destroy = true
  name          = %[1]q
}
`, rName)
}

func testAccUserConfig_duplicateNameBase(name1, name2 string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "user" {
  name = %[1]q
}

resource "ctyun_iam_user" "other" {
  name = %[2]q
}
`, name1, name2)
}

func testAccUserConfig_duplicateNameConflict(name1, name2 string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "user" {
  name = %[2]q
}

resource "ctyun_iam_user" "other" {
  name = %[2]q
}
`, name1, name2)
}

func testAccUserConfig_destroyOrderMembershipAndAttachment(userName, groupName, policyName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name          = %[1]q
  force_destroy = true
}

resource "ctyun_iam_group" "test" {
  name = %[2]q
}

resource "ctyun_iam_user_group_membership" "test" {
  user  = ctyun_iam_user.test.name
  group = ctyun_iam_group.test.name
}

resource "ctyun_iam_policy" "test" {
  name        = %[3]q
  description = "A test policy"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "iam:ChangePassword"
      ],
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
`, userName, groupName, policyName)
}
