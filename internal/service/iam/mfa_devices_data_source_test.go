// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUnitIAMMFADevicesDataSource_schema verifies the schema shape of ctyun_iam_mfa_devices:
// mfa_devices is a Computed TypeList whose elements contain enable_date / serial_number / user_name;
// the user_name input is Optional; id is not in the Schema (written by Read via SetId, consistent with other SDKv2 DSs).
func TestUnitIAMMFADevicesDataSource_schema(t *testing.T) {
	t.Parallel()

	dataSource := tfiam.DataSourceMFADevices()

	for _, name := range []string{"mfa_devices", names.AttrUserName} {
		if _, ok := dataSource.Schema[name]; !ok {
			t.Fatalf("expected %s schema", name)
		}
	}

	devices := dataSource.Schema["mfa_devices"]
	if devices.Type != schema.TypeList {
		t.Fatalf("expected mfa_devices TypeList, got %v", devices.Type)
	}
	if !devices.Computed {
		t.Fatal("expected mfa_devices Computed")
	}
	elem, ok := devices.Elem.(*schema.Resource)
	if !ok {
		t.Fatalf("expected mfa_devices Elem *schema.Resource, got %T", devices.Elem)
	}
	for _, name := range []string{"enable_date", "serial_number", names.AttrUserName} {
		attr, ok := elem.Schema[name]
		if !ok {
			t.Fatalf("expected mfa_devices nested %s", name)
		}
		if !attr.Computed {
			t.Fatalf("expected mfa_devices.%s Computed", name)
		}
	}

	userName := dataSource.Schema[names.AttrUserName]
	if !userName.Optional {
		t.Fatal("expected user_name Optional")
	}
	if userName.ValidateFunc == nil {
		t.Fatal("expected user_name ValidateFunc")
	}
}

// TestUnitIAMMFADevicesDataSource_nameValidation verifies the ValidateFunc of the user_name input
// (covers the former TC-IAM-DMFA-03 User Validation Matrix):
// valid special characters and the upper length bound of 64 pass; empty values, 65 characters, and
// illegal characters such as space / ! / & / * fail at the schema stage.
func TestUnitIAMMFADevicesDataSource_nameValidation(t *testing.T) {
	t.Parallel()

	validateFunc := tfiam.DataSourceMFADevices().Schema[names.AttrUserName].ValidateFunc
	if validateFunc == nil {
		t.Fatal("expected user_name ValidateFunc")
	}

	testCases := []struct {
		name        string
		value       string
		errContains []string
	}{
		{name: "valid_special_chars", value: "User_-,.+@=123", errContains: nil},
		{name: "valid_special_chars_alt", value: "test-1+2_.@hello", errContains: nil},
		{name: "valid_max_length", value: strings.Repeat("a", 64), errContains: nil},
		{name: "empty", value: "", errContains: []string{
			"expected length of user_name to be in the range (1 - 64)",
		}},
		{name: "too_long", value: strings.Repeat("a", 65), errContains: []string{
			"expected length of user_name to be in the range (1 - 64)",
		}},
		{name: "invalid_space", value: "t est", errContains: []string{
			"must only contain alphanumeric characters",
		}},
		{name: "invalid_exclamation", value: "invalid user!", errContains: []string{
			"must only contain alphanumeric characters",
		}},
		{name: "invalid_ampersand_asterisk", value: "testa&b.*+", errContains: []string{
			"must only contain alphanumeric characters",
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := validateFunc(tc.value, names.AttrUserName)
			errText := fmt.Sprintf("%v", errs)

			if len(tc.errContains) == 0 {
				if len(errs) != 0 {
					t.Fatalf("expected no validation error for user_name=%q, got: %s", tc.value, errText)
				}
				return
			}

			if len(errs) == 0 {
				t.Fatalf("expected validation error for user_name=%q containing %v, got none", tc.value, tc.errContains)
			}

			for _, want := range tc.errContains {
				if !strings.Contains(errText, want) {
					t.Fatalf("validation error for user_name missing %q\nfull errors: %s", want, errText)
				}
			}
		})
	}
}

// TestUnitIAMMFADevicesDataSource_flatten verifies flattening a single device: enable_date / serial_number / user_name.
func TestUnitIAMMFADevicesDataSource_flatten(t *testing.T) {
	t.Parallel()

	enableDate := time.Date(2026, 6, 19, 8, 30, 10, 0, time.UTC)
	device := &awstypes.MFADevice{
		EnableDate:   aws.Time(enableDate),
		SerialNumber: aws.String("arn:ctyun:iam::10rc2arpn6306:mfa/mfa1"),
		UserName:     aws.String("mfa1"),
	}

	output := tfiam.FlattenMFADeviceForDataSource(device)

	if got, want := output["enable_date"], "2026-06-19T08:30:10Z"; got != want {
		t.Fatalf("expected enable_date %q, got %q", want, got)
	}
	if got, want := output["serial_number"], "arn:ctyun:iam::10rc2arpn6306:mfa/mfa1"; got != want {
		t.Fatalf("expected serial_number %q, got %q", want, got)
	}
	if got, want := output[names.AttrUserName], "mfa1"; got != want {
		t.Fatalf("expected user_name %q, got %q", want, got)
	}
}

// TestUnitIAMMFADevicesDataSource_flatten_nil verifies that flattening a nil device yields an empty map.
func TestUnitIAMMFADevicesDataSource_flatten_nil(t *testing.T) {
	t.Parallel()

	output := tfiam.FlattenMFADeviceForDataSource(nil)
	if got, want := len(output), 0; got != want {
		t.Fatalf("expected %d attributes, got %d", want, got)
	}
}

// TestUnitIAMMFADevicesDataSource_flattenList verifies flattening a list of multiple devices.
func TestUnitIAMMFADevicesDataSource_flattenList(t *testing.T) {
	t.Parallel()

	enableDate := time.Date(2026, 6, 19, 8, 30, 10, 0, time.UTC)
	devices := []awstypes.MFADevice{
		{
			EnableDate:   aws.Time(enableDate),
			SerialNumber: aws.String("arn:ctyun:iam::10rc2arpn6306:mfa/mfa1"),
			UserName:     aws.String("mfa1"),
		},
		{
			SerialNumber: aws.String("arn:ctyun:iam::10rc2arpn6306:mfa/mfa2"),
			UserName:     aws.String("mfa2"),
		},
	}

	output := tfiam.FlattenMFADevicesForDataSource(devices)
	if got, want := len(output), 2; got != want {
		t.Fatalf("expected %d devices, got %d", want, got)
	}
	if got, want := output[0]["serial_number"], "arn:ctyun:iam::10rc2arpn6306:mfa/mfa1"; got != want {
		t.Fatalf("expected serial_number %q, got %q", want, got)
	}
	if got, want := output[1][names.AttrUserName], "mfa2"; got != want {
		t.Fatalf("expected user_name %q, got %q", want, got)
	}
}

// TC-IAM-DMFA-01-Lookup By User verifies reading bound MFA devices by user_name:
// step1 creates user+VMFA and binds it with EnableExternal; step2 applies again to read the DS, compared against the cloud ListMFADevices result.
func TestAccIAMMFADevicesDataSource_lookupByUser(t *testing.T) {
	ctx := acctest.Context(t)
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	deviceName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_mfa_devices.test"
	resourceName := "ctyun_iam_virtual_mfa_device.test"
	userResourceName := "ctyun_iam_user.test"
	var user awstypes.User

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckVirtualMFADeviceDestroy(ctx, t),
			testAccCheckUserDestroy(ctx, t),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccMFADevicesDataSourceConfig_lookupByUser(userName, deviceName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, userResourceName, &user),
					testAccCheckVirtualMFADeviceEnableExternal(ctx, t, resourceName, &user),
				),
			},
			{
				Config: testAccMFADevicesDataSourceConfig_lookupByUser(userName, deviceName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckMFADevicesDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrUserName, userName),
					resource.TestCheckResourceAttr(dataSourceName, "mfa_devices.#", "1"),
					resource.TestCheckResourceAttrPair(dataSourceName, "mfa_devices.0.serial_number", resourceName, "serial_number"),
					resource.TestCheckResourceAttr(dataSourceName, "mfa_devices.0.user_name", userName),
					resource.TestCheckResourceAttrSet(dataSourceName, "mfa_devices.0.enable_date"),
				),
			},
		},
	})
}

// TC-IAM-DMFA-02-No User Filter verifies omitting user_name: id is the Region, and mfa_devices matches the cloud
// ListMFADevices result (empty UserName). Under OOS a root call returns an empty list, so mfa_devices.#=0 is asserted explicitly.
func TestAccIAMMFADevicesDataSource_noUserFilter(t *testing.T) {
	ctx := acctest.Context(t)
	dataSourceName := "data.ctyun_iam_mfa_devices.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMFADevicesDataSourceConfig_noUserFilter,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckMFADevicesDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, "mfa_devices.#", "0"),
				),
			},
		},
	})
}

// TC-IAM-DMFA-04-Empty Result verifies a user that exists but has no MFA bound: matches the cloud and mfa_devices is empty.
func TestAccIAMMFADevicesDataSource_empty(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_mfa_devices.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccMFADevicesDataSourceConfig_empty(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckMFADevicesDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckResourceAttr(dataSourceName, names.AttrUserName, rName),
					resource.TestCheckResourceAttr(dataSourceName, "mfa_devices.#", "0"),
				),
			},
		},
	})
}

// TC-IAM-DMFA-05-Read Stability verifies two consecutive applies of the same configuration after binding MFA: at each step
// the DS state matches the cloud ListMFADevices result, cross-checking serial_number/user_name/enable_date;
// the second apply shows no diff (PreApply / PostApplyPostRefresh). CheckDestroy at the end.
func TestAccIAMMFADevicesDataSource_readStability(t *testing.T) {
	ctx := acctest.Context(t)
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	deviceName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_mfa_devices.test"
	resourceName := "ctyun_iam_virtual_mfa_device.test"
	userResourceName := "ctyun_iam_user.test"
	var user awstypes.User

	checks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckMFADevicesDataSourceStateMatchesCloud(ctx, t, dataSourceName),
		resource.TestCheckResourceAttr(dataSourceName, names.AttrUserName, userName),
		resource.TestCheckResourceAttr(dataSourceName, "mfa_devices.#", "1"),
		resource.TestCheckResourceAttrPair(dataSourceName, "mfa_devices.0.serial_number", resourceName, "serial_number"),
		resource.TestCheckResourceAttr(dataSourceName, "mfa_devices.0.user_name", userName),
		resource.TestCheckResourceAttrSet(dataSourceName, "mfa_devices.0.enable_date"),
	)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckVirtualMFADeviceDestroy(ctx, t),
			testAccCheckUserDestroy(ctx, t),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccMFADevicesDataSourceConfig_lookupByUser(userName, deviceName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, userResourceName, &user),
					testAccCheckVirtualMFADeviceEnableExternal(ctx, t, resourceName, &user),
				),
			},
			{
				Config: testAccMFADevicesDataSourceConfig_lookupByUser(userName, deviceName),
				Check:  checks,
			},
			{
				Config: testAccMFADevicesDataSourceConfig_lookupByUser(userName, deviceName),
				Check:  checks,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// testAccCheckMFADevicesDataSourceStateMatchesCloud checks the DS state:
// - id equals the current IAM EffectiveRealRegion;
// - the cloud list is fetched with the same ListMFADevices input as the DS (with UserName when present);
// - mfa_devices is compared as a set keyed by serial_number, with field-by-field comparison of enable_date / user_name.
func testAccCheckMFADevicesDataSourceStateMatchesCloud(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}

		meta := acctest.ProviderMeta(ctx, t)
		wantID := meta.EffectiveRealRegion(conns.NewResourceContext(ctx, names.IAM, "", "", ""))
		if wantID == "" {
			return fmt.Errorf("could not resolve IAM EffectiveRealRegion for id check")
		}
		if got := rs.Primary.ID; got != wantID {
			return fmt.Errorf("bad id: got %s, want %s (region)", got, wantID)
		}

		input := &iam.ListMFADevicesInput{}
		if userName := rs.Primary.Attributes[names.AttrUserName]; userName != "" {
			input.UserName = aws.String(userName)
		}

		conn := meta.IAMClient(ctx)
		cloudDevices, err := tfiam.FindMFADevicesForDataSource(ctx, conn, input)
		if err != nil {
			return fmt.Errorf("listing MFA devices: %w", err)
		}

		attrs := rs.Primary.Attributes
		stateCount, err := strconv.Atoi(attrs["mfa_devices.#"])
		if err != nil {
			return fmt.Errorf("parsing mfa_devices.#: %w", err)
		}
		if stateCount != len(cloudDevices) {
			return fmt.Errorf("mfa_devices count mismatch: state=%d listMFADevices=%d", stateCount, len(cloudDevices))
		}

		cloudBySerial := make(map[string]awstypes.MFADevice, len(cloudDevices))
		for _, device := range cloudDevices {
			cloudBySerial[aws.ToString(device.SerialNumber)] = device
		}

		seen := make(map[string]struct{}, stateCount)
		for i := 0; i < stateCount; i++ {
			prefix := fmt.Sprintf("mfa_devices.%d.", i)
			serial := attrs[prefix+"serial_number"]
			if serial == "" {
				return fmt.Errorf("empty mfa_devices.%d.serial_number", i)
			}
			if _, dup := seen[serial]; dup {
				return fmt.Errorf("duplicate mfa_devices.%d.serial_number %q in state", i, serial)
			}
			seen[serial] = struct{}{}

			cloud, ok := cloudBySerial[serial]
			if !ok {
				return fmt.Errorf("serial_number %q not found in ListMFADevices", serial)
			}

			wantEnableDate := ""
			if cloud.EnableDate != nil {
				wantEnableDate = aws.ToTime(cloud.EnableDate).Format(time.RFC3339)
			}
			if got := attrs[prefix+"enable_date"]; got != wantEnableDate {
				return fmt.Errorf("bad mfa_devices.%d.enable_date: got %q, want %q", i, got, wantEnableDate)
			}
			if got, want := attrs[prefix+names.AttrUserName], aws.ToString(cloud.UserName); got != want {
				return fmt.Errorf("bad mfa_devices.%d.user_name: got %q, want %q", i, got, want)
			}
		}

		if len(seen) != len(cloudBySerial) {
			return fmt.Errorf("mfa_devices serial set size mismatch after field checks: state=%d listMFADevices=%d", len(seen), len(cloudBySerial))
		}

		return nil
	}
}

func testAccMFADevicesDataSourceConfig_empty(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name = %[1]q
}

data "ctyun_iam_mfa_devices" "test" {
  user_name = ctyun_iam_user.test.name
}
`, rName)
}

func testAccMFADevicesDataSourceConfig_lookupByUser(userName, deviceName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name = %[1]q
}

resource "ctyun_iam_virtual_mfa_device" "test" {
  virtual_mfa_device_name = %[2]q
  depends_on              = [ctyun_iam_user.test]
}

data "ctyun_iam_mfa_devices" "test" {
  user_name  = ctyun_iam_user.test.name
  depends_on = [ctyun_iam_virtual_mfa_device.test]
}
`, userName, deviceName)
}

const testAccMFADevicesDataSourceConfig_noUserFilter = `
data "ctyun_iam_mfa_devices" "test" {}
`
