// Copyright IBM Corp. 2014, 2026
// Modifications Copyright (C) 2026 China Telecom Cloud Technology Co., Ltd.
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestUnitIAMVirtualMFADevicesDataSource_schema verifies the schema shape of the ctyun_iam_virtual_mfa_devices
// data source: no Required inputs; it contains id / virtual_mfa_devices (Computed).
func TestUnitIAMVirtualMFADevicesDataSource_schema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ds, err := tfiam.NewVirtualMFADevicesDataSource(ctx)
	if err != nil {
		t.Fatalf("new data source: %s", err)
	}

	var resp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}

	for _, name := range []string{names.AttrID, "virtual_mfa_devices"} {
		if _, ok := resp.Schema.Attributes[name]; !ok {
			t.Fatalf("expected %s schema", name)
		}
	}

	devicesAttr, ok := resp.Schema.Attributes["virtual_mfa_devices"].(schema.ListAttribute)
	if !ok {
		t.Fatalf("expected virtual_mfa_devices ListAttribute, got %T", resp.Schema.Attributes["virtual_mfa_devices"])
	}
	if !devicesAttr.IsComputed() {
		t.Fatal("expected virtual_mfa_devices Computed")
	}
}

// TestUnitIAMVirtualMFADevicesDataSource_flatten verifies the flatten mapping for bound / unbound devices:
// a bound device contains enable_date/serial_number and the nested user fields; an unbound device has empty enable_date and user fields.
func TestUnitIAMVirtualMFADevicesDataSource_flatten(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	enableDate := time.Date(2026, 6, 19, 8, 30, 10, 0, time.UTC)
	createDate := time.Date(2026, 5, 9, 8, 30, 10, 0, time.UTC)
	passwordLastUsed := time.Date(2026, 6, 19, 8, 30, 10, 0, time.UTC)
	devices := []awstypes.VirtualMFADevice{
		{
			EnableDate:   aws.Time(enableDate),
			SerialNumber: aws.String("arn:ctyun:iam::10rc2arpn6306:mfa/mfa1"),
			User: &awstypes.User{
				Arn:              aws.String("arn:ctyun:iam::10rc2arpn6306:user/mfa1"),
				CreateDate:       aws.Time(createDate),
				PasswordLastUsed: aws.Time(passwordLastUsed),
				UserId:           aws.String("AIDA6DQOBIADX4MRQEY7W"),
				UserName:         aws.String("mfa1"),
			},
		},
		{
			SerialNumber: aws.String("arn:ctyun:iam::10rc2arpn6306:mfa/mfa2"),
		},
	}

	output := tfiam.FlattenVirtualMFADevices(ctx, devices)

	if got, want := len(output), 2; got != want {
		t.Fatalf("expected %d devices, got %d", want, got)
	}

	if got, want := output[0].EnableDate.ValueString(), "2026-06-19T08:30:10Z"; got != want {
		t.Fatalf("expected enable_date %q, got %q", want, got)
	}

	if got, want := output[0].SerialNumber.ValueString(), "arn:ctyun:iam::10rc2arpn6306:mfa/mfa1"; got != want {
		t.Fatalf("expected serial_number %q, got %q", want, got)
	}

	user, diags := output[0].User.ToPtr(ctx)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if got, want := user.ARN.ValueString(), "arn:ctyun:iam::10rc2arpn6306:user/mfa1"; got != want {
		t.Fatalf("expected user arn %q, got %q", want, got)
	}

	if got, want := user.CreateDate.ValueString(), "2026-05-09T08:30:10Z"; got != want {
		t.Fatalf("expected user create_date %q, got %q", want, got)
	}

	if got, want := user.PasswordLastUsed.ValueString(), "2026-06-19T08:30:10Z"; got != want {
		t.Fatalf("expected user password_last_used %q, got %q", want, got)
	}

	if got, want := user.UserID.ValueString(), "AIDA6DQOBIADX4MRQEY7W"; got != want {
		t.Fatalf("expected user_id %q, got %q", want, got)
	}

	if got, want := user.UserName.ValueString(), "mfa1"; got != want {
		t.Fatalf("expected user_name %q, got %q", want, got)
	}

	unassignedUser, diags := output[1].User.ToPtr(ctx)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if got, want := output[1].EnableDate.ValueString(), ""; got != want {
		t.Fatalf("expected unassigned enable_date %q, got %q", want, got)
	}

	if got, want := unassignedUser.ARN.ValueString(), ""; got != want {
		t.Fatalf("expected unassigned user arn %q, got %q", want, got)
	}
	if got, want := unassignedUser.CreateDate.ValueString(), ""; got != want {
		t.Fatalf("expected unassigned user create_date %q, got %q", want, got)
	}
	if got, want := unassignedUser.PasswordLastUsed.ValueString(), ""; got != want {
		t.Fatalf("expected unassigned user password_last_used %q, got %q", want, got)
	}
	if got, want := unassignedUser.UserID.ValueString(), ""; got != want {
		t.Fatalf("expected unassigned user_id %q, got %q", want, got)
	}
	if got, want := unassignedUser.UserName.ValueString(), ""; got != want {
		t.Fatalf("expected unassigned user_name %q, got %q", want, got)
	}
}

// TC-IAM-DVMFA-01-Basic List verifies reading after creating at least one virtual MFA device:
// id is the Region; the DS state matches the cloud ListVirtualMFADevices result (paginated aggregation); and the created
// serial_number is confirmed to appear in the list.
// Serial: without filtering, ExactMatch compares against the whole account and would race with fixtures created in parallel by pagination etc.
func TestAccIAMVirtualMFADevicesDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_virtual_mfa_devices.test"
	resourceName := "ctyun_iam_virtual_mfa_device.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckVirtualMFADeviceDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualMFADevicesDataSourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckVirtualMFADevicesDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckTypeSetElemAttrPair(dataSourceName, "virtual_mfa_devices.*.serial_number", resourceName, "serial_number"),
				),
			},
		},
	})
}

// TC-IAM-DVMFA-02-Nested User Readback verifies the DS nested user read-back after binding a user:
// step1 creates user+VMFA+DS and completes the binding via the resource-side EnableExternal (internally enableVirtualMFADeviceForUser);
// step2 applies again to re-read the DS, matches the cloud, and asserts that the target device's enable_date / user.* are back-filled.
// Serial: step2 ExactMatch compares against the whole account.
func TestAccIAMVirtualMFADevicesDataSource_nestedUser(t *testing.T) {
	ctx := acctest.Context(t)
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	deviceName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_virtual_mfa_devices.test"
	resourceName := "ctyun_iam_virtual_mfa_device.test"
	userResourceName := "ctyun_iam_user.test"
	var user awstypes.User

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckVirtualMFADeviceDestroy(ctx, t),
			testAccCheckUserDestroy(ctx, t),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualMFADevicesDataSourceConfig_nestedUser(userName, deviceName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, userResourceName, &user),
					testAccCheckVirtualMFADeviceEnableExternal(ctx, t, resourceName, &user),
				),
			},
			{
				Config: testAccVirtualMFADevicesDataSourceConfig_nestedUser(userName, deviceName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckVirtualMFADevicesDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					resource.TestCheckTypeSetElemAttrPair(dataSourceName, "virtual_mfa_devices.*.serial_number", resourceName, "serial_number"),
					resource.TestCheckTypeSetElemAttrPair(dataSourceName, "virtual_mfa_devices.*.user.user_name", userResourceName, "name"),
					testAccCheckVirtualMFADevicesDataSourceAssigned(dataSourceName, resourceName, userName),
				),
			},
		},
	})
}

// TC-IAM-DVMFA-03-Unassigned Device verifies a VMFA not bound to any user: matches the cloud,
// and the target device's enable_date / user nested fields are empty strings.
// Serial: ExactMatch compares against the whole account.
func TestAccIAMVirtualMFADevicesDataSource_unassigned(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_virtual_mfa_devices.test"
	resourceName := "ctyun_iam_virtual_mfa_device.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckVirtualMFADeviceDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualMFADevicesDataSourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckVirtualMFADevicesDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					testAccCheckVirtualMFADevicesDataSourceUnassigned(dataSourceName, resourceName),
				),
			},
		},
	})
}

// TC-IAM-DVMFA-04-Read Stability verifies two consecutive applies of the same configuration: each read-back matches the cloud,
// the target device remains unbound, and its serial_number is found; the second apply shows no diff (PreApply / PostApplyPostRefresh).
// Serial: ExactMatch compares against the whole account.
func TestAccIAMVirtualMFADevicesDataSource_readStability(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	dataSourceName := "data.ctyun_iam_virtual_mfa_devices.test"
	resourceName := "ctyun_iam_virtual_mfa_device.test"

	checks := resource.ComposeAggregateTestCheckFunc(
		testAccCheckVirtualMFADevicesDataSourceStateMatchesCloud(ctx, t, dataSourceName),
		resource.TestCheckTypeSetElemAttrPair(dataSourceName, "virtual_mfa_devices.*.serial_number", resourceName, "serial_number"),
		testAccCheckVirtualMFADevicesDataSourceUnassigned(dataSourceName, resourceName),
	)

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckVirtualMFADeviceDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualMFADevicesDataSourceConfig_basic(rName),
				Check:  checks,
			},
			{
				Config: testAccVirtualMFADevicesDataSourceConfig_basic(rName),
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

// The ListVirtualMFADevices default MaxItems is 100, consistent with AWS IAM / OOS; the DS Read uses an empty Input + Paginator
// and cannot change MaxItems, so this case creates more devices than the default page size to force the real Read through multi-page aggregation.
// For non-whitelisted OOS accounts, the VMFA creation quota is aligned with UsersQuota (per GetAccountSummary).
const testAccIAMListVirtualMFADevicesDefaultMaxItems = 100

// TC-IAM-DVMFA-05-Pagination verifies that when the total number of virtual MFA devices in the account exceeds the
// ListVirtualMFADevices default MaxItems, the data source Read (empty ListVirtualMFADevicesInput → default pagination) still
// aggregates and returns all devices, matching the cloud.
// Serial: creates 101 devices and ExactMatch-compares against the whole account; running in parallel would pollute other DS cases.
func TestAccIAMVirtualMFADevicesDataSource_pagination(t *testing.T) {
	ctx := acctest.Context(t)
	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	deviceCount := testAccIAMListVirtualMFADevicesDefaultMaxItems + 1
	dataSourceName := "data.ctyun_iam_virtual_mfa_devices.test"

	acctest.Test(ctx, t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			testAccPreCheckVirtualMFADevicesPaginationQuota(ctx, t, deviceCount)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckVirtualMFADeviceDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualMFADevicesDataSourceConfig_pagination(rName, deviceCount),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckVirtualMFADevicesListDefaultPageTruncated(ctx, t),
					testAccCheckVirtualMFADevicesDataSourceStateMatchesCloud(ctx, t, dataSourceName),
					testAccCheckVirtualMFADevicesDataSourceContainsCreated(dataSourceName, "ctyun_iam_virtual_mfa_device.test", deviceCount),
				),
			},
		},
	})
}

// testAccCheckVirtualMFADevicesDataSourceStateMatchesCloud checks the DS state:
//   - id equals the current IAM EffectiveRealRegion;
//   - matches the cloud ListVirtualMFADevices result (paginated aggregation) without relying on managed resource state:
//     compares the virtual_mfa_devices set (keyed by serial_number) and the per-field enable_date / user.* values.
func testAccCheckVirtualMFADevicesDataSourceStateMatchesCloud(ctx context.Context, t *testing.T, dataSourceName string) resource.TestCheckFunc {
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

		conn := meta.IAMClient(ctx)
		cloudDevices, err := tfiam.FindVirtualMFADevicesForDataSource(ctx, conn, &iam.ListVirtualMFADevicesInput{})
		if err != nil {
			return fmt.Errorf("listing virtual MFA devices: %w", err)
		}

		attrs := rs.Primary.Attributes
		stateCount, err := strconv.Atoi(attrs["virtual_mfa_devices.#"])
		if err != nil {
			return fmt.Errorf("parsing virtual_mfa_devices.#: %w", err)
		}
		if stateCount != len(cloudDevices) {
			return fmt.Errorf("virtual_mfa_devices count mismatch: state=%d listVirtualMFADevices=%d", stateCount, len(cloudDevices))
		}

		cloudBySerial := make(map[string]awstypes.VirtualMFADevice, len(cloudDevices))
		for _, device := range cloudDevices {
			cloudBySerial[aws.ToString(device.SerialNumber)] = device
		}

		seen := make(map[string]struct{}, stateCount)
		for i := 0; i < stateCount; i++ {
			prefix := fmt.Sprintf("virtual_mfa_devices.%d.", i)
			serial := attrs[prefix+"serial_number"]
			if serial == "" {
				return fmt.Errorf("empty virtual_mfa_devices.%d.serial_number", i)
			}
			if _, dup := seen[serial]; dup {
				return fmt.Errorf("duplicate virtual_mfa_devices.%d.serial_number %q in state", i, serial)
			}
			seen[serial] = struct{}{}

			cloud, ok := cloudBySerial[serial]
			if !ok {
				return fmt.Errorf("serial_number %q not found in ListVirtualMFADevices", serial)
			}

			wantEnableDate := ""
			if cloud.EnableDate != nil {
				wantEnableDate = aws.ToTime(cloud.EnableDate).Format(time.RFC3339)
			}
			if got := attrs[prefix+"enable_date"]; got != wantEnableDate {
				return fmt.Errorf("bad virtual_mfa_devices.%d.enable_date: got %q, want %q", i, got, wantEnableDate)
			}

			wantUserARN, wantUserCreateDate, wantPasswordLastUsed, wantUserID, wantUserName := "", "", "", "", ""
			if cloud.User != nil {
				wantUserARN = aws.ToString(cloud.User.Arn)
				if cloud.User.CreateDate != nil {
					wantUserCreateDate = aws.ToTime(cloud.User.CreateDate).Format(time.RFC3339)
				}
				if cloud.User.PasswordLastUsed != nil {
					wantPasswordLastUsed = aws.ToTime(cloud.User.PasswordLastUsed).Format(time.RFC3339)
				}
				wantUserID = aws.ToString(cloud.User.UserId)
				wantUserName = aws.ToString(cloud.User.UserName)
			}
			if got := attrs[prefix+"user.arn"]; got != wantUserARN {
				return fmt.Errorf("bad virtual_mfa_devices.%d.user.arn: got %q, want %q", i, got, wantUserARN)
			}
			if got := attrs[prefix+"user.create_date"]; got != wantUserCreateDate {
				return fmt.Errorf("bad virtual_mfa_devices.%d.user.create_date: got %q, want %q", i, got, wantUserCreateDate)
			}
			if got := attrs[prefix+"user.password_last_used"]; got != wantPasswordLastUsed {
				return fmt.Errorf("bad virtual_mfa_devices.%d.user.password_last_used: got %q, want %q", i, got, wantPasswordLastUsed)
			}
			if got := attrs[prefix+"user.user_id"]; got != wantUserID {
				return fmt.Errorf("bad virtual_mfa_devices.%d.user.user_id: got %q, want %q", i, got, wantUserID)
			}
			if got := attrs[prefix+"user.user_name"]; got != wantUserName {
				return fmt.Errorf("bad virtual_mfa_devices.%d.user.user_name: got %q, want %q", i, got, wantUserName)
			}
		}

		if len(seen) != len(cloudBySerial) {
			return fmt.Errorf("virtual_mfa_devices serial set size mismatch after field checks: state=%d listVirtualMFADevices=%d", len(seen), len(cloudBySerial))
		}

		return nil
	}
}

func testAccCheckVirtualMFADevicesDataSourceUnassigned(dataSourceName, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		serial := rs.Primary.Attributes["serial_number"]
		if serial == "" {
			return fmt.Errorf("No serial_number set on %s", resourceName)
		}

		ds, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}

		count, err := strconv.Atoi(ds.Primary.Attributes["virtual_mfa_devices.#"])
		if err != nil {
			return fmt.Errorf("invalid virtual_mfa_devices.#: %w", err)
		}

		for i := 0; i < count; i++ {
			prefix := fmt.Sprintf("virtual_mfa_devices.%d", i)
			if ds.Primary.Attributes[prefix+".serial_number"] != serial {
				continue
			}

			for _, attr := range []string{
				"enable_date",
				"user.arn",
				"user.create_date",
				"user.password_last_used",
				"user.user_id",
				"user.user_name",
			} {
				if got := ds.Primary.Attributes[prefix+"."+attr]; got != "" {
					return fmt.Errorf("expected %s.%s to be empty for unassigned device %s, got %q", prefix, attr, serial, got)
				}
			}

			return nil
		}

		return fmt.Errorf("virtual MFA device with serial_number %q not found in %s", serial, dataSourceName)
	}
}

// testAccCheckVirtualMFADevicesDataSourceAssigned locates the DS entry by the managed resource's serial_number,
// asserts that enable_date is set and user.user_name equals the bound user, and that the other key user identifier fields are non-empty.
func testAccCheckVirtualMFADevicesDataSourceAssigned(dataSourceName, resourceName, userName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		serial := rs.Primary.Attributes["serial_number"]
		if serial == "" {
			return fmt.Errorf("No serial_number set on %s", resourceName)
		}

		ds, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}

		count, err := strconv.Atoi(ds.Primary.Attributes["virtual_mfa_devices.#"])
		if err != nil {
			return fmt.Errorf("invalid virtual_mfa_devices.#: %w", err)
		}

		for i := 0; i < count; i++ {
			prefix := fmt.Sprintf("virtual_mfa_devices.%d", i)
			if ds.Primary.Attributes[prefix+".serial_number"] != serial {
				continue
			}

			if got := ds.Primary.Attributes[prefix+".enable_date"]; got == "" {
				return fmt.Errorf("expected %s.enable_date to be set for assigned device %s", prefix, serial)
			}
			if got := ds.Primary.Attributes[prefix+".user.user_name"]; got != userName {
				return fmt.Errorf("bad %s.user.user_name: got %q, want %q", prefix, got, userName)
			}
			for _, attr := range []string{"user.arn", "user.user_id", "user.create_date"} {
				if got := ds.Primary.Attributes[prefix+"."+attr]; got == "" {
					return fmt.Errorf("expected %s.%s to be set for assigned device %s", prefix, attr, serial)
				}
			}

			return nil
		}

		return fmt.Errorf("virtual MFA device with serial_number %q not found in %s", serial, dataSourceName)
	}
}

func testAccVirtualMFADevicesDataSourceConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_virtual_mfa_device" "test" {
  virtual_mfa_device_name = %[1]q
}

data "ctyun_iam_virtual_mfa_devices" "test" {
  depends_on = [ctyun_iam_virtual_mfa_device.test]
}
`, rName)
}

func testAccVirtualMFADevicesDataSourceConfig_nestedUser(userName, deviceName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name = %[1]q
}

resource "ctyun_iam_virtual_mfa_device" "test" {
  virtual_mfa_device_name = %[2]q
  depends_on              = [ctyun_iam_user.test]
}

data "ctyun_iam_virtual_mfa_devices" "test" {
  depends_on = [ctyun_iam_virtual_mfa_device.test]
}
`, userName, deviceName)
}

func testAccVirtualMFADevicesDataSourceConfig_pagination(rName string, deviceCount int) string {
	return fmt.Sprintf(`
resource "ctyun_iam_virtual_mfa_device" "test" {
  count                   = %[2]d
  virtual_mfa_device_name = "%[1]s-${count.index}"
}

data "ctyun_iam_virtual_mfa_devices" "test" {
  depends_on = [ctyun_iam_virtual_mfa_device.test]
}
`, rName, deviceCount)
}

// testAccPreCheckVirtualMFADevicesPaginationQuota confirms that the account's remaining MFA quota is sufficient to create need virtual MFA devices.
// For the non-whitelisted OOS path, UsersQuota serves as the MFADevices limit and MFADevices is the used count.
func testAccPreCheckVirtualMFADevicesPaginationQuota(ctx context.Context, t *testing.T, need int) {
	t.Helper()

	conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
	out, err := conn.GetAccountSummary(ctx, &iam.GetAccountSummaryInput{})
	if err != nil {
		t.Fatalf("GetAccountSummary: %s", err)
	}

	quota, ok := out.SummaryMap[string(awstypes.SummaryKeyTypeUsersQuota)]
	if !ok {
		t.Fatalf("UsersQuota missing in SummaryMap: %#v", out.SummaryMap)
	}
	used := out.SummaryMap[string(awstypes.SummaryKeyTypeMFADevices)]
	remaining := int(quota) - int(used)
	if remaining < need {
		t.Skipf("insufficient MFADevices remaining=%d need=%d (UsersQuota=%d MFADevices=%d)", remaining, need, quota, used)
	}
}

// testAccCheckVirtualMFADevicesListDefaultPageTruncated fetches one page (without paging further) using the same empty
// ListVirtualMFADevicesInput as the DS Read, confirming the account has more than the default MaxItems with IsTruncated=true
// (otherwise this case has not actually exercised the DS pagination path).
func testAccCheckVirtualMFADevicesListDefaultPageTruncated(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		page, err := conn.ListVirtualMFADevices(ctx, &iam.ListVirtualMFADevicesInput{})
		if err != nil {
			return fmt.Errorf("ListVirtualMFADevices(default MaxItems) single page: %w", err)
		}
		if !page.IsTruncated {
			return fmt.Errorf("expected IsTruncated=true for default ListVirtualMFADevices page (MaxItems default %d); first page returned %d devices — DS Read would not exercise pagination", testAccIAMListVirtualMFADevicesDefaultMaxItems, len(page.VirtualMFADevices))
		}
		if aws.ToString(page.Marker) == "" {
			return fmt.Errorf("expected non-empty Marker when IsTruncated=true")
		}
		return nil
	}
}

// testAccCheckVirtualMFADevicesDataSourceContainsCreated confirms that the serial_numbers of all VMFAs created by this case
// via count appear in the DS list (the account may contain other devices, so the total is not hard-coded).
func testAccCheckVirtualMFADevicesDataSourceContainsCreated(dataSourceName, resourceAddress string, deviceCount int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ds, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}

		dsCount, err := strconv.Atoi(ds.Primary.Attributes["virtual_mfa_devices.#"])
		if err != nil {
			return fmt.Errorf("invalid virtual_mfa_devices.#: %w", err)
		}

		serials := make(map[string]struct{}, dsCount)
		for i := 0; i < dsCount; i++ {
			serial := ds.Primary.Attributes[fmt.Sprintf("virtual_mfa_devices.%d.serial_number", i)]
			if serial == "" {
				return fmt.Errorf("empty virtual_mfa_devices.%d.serial_number", i)
			}
			serials[serial] = struct{}{}
		}

		for i := 0; i < deviceCount; i++ {
			addr := fmt.Sprintf("%s.%d", resourceAddress, i)
			rs, ok := s.RootModule().Resources[addr]
			if !ok {
				return fmt.Errorf("Not found: %s", addr)
			}
			serial := rs.Primary.Attributes["serial_number"]
			if serial == "" {
				return fmt.Errorf("No serial_number set on %s", addr)
			}
			if _, ok := serials[serial]; !ok {
				return fmt.Errorf("created device serial_number %q (%s) not found in %s", serial, addr, dataSourceName)
			}
		}

		return nil
	}
}
