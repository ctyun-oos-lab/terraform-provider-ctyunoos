// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/acctest"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	tfiam "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/service/iam"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/pquerna/otp/totp"
)

func testAccVirtualMFADeviceARNPattern(deviceName string) *regexp.Regexp {
	return regexp.MustCompile(fmt.Sprintf(`^arn:ctyun:iam::[0-9A-Za-z]+:mfa/%s$`, regexp.QuoteMeta(deviceName)))
}

func testAccCheckResourceAttrOOSIAMMFADeviceARN(resourceName, attrName, deviceName string) resource.TestCheckFunc {
	return resource.TestMatchResourceAttr(resourceName, attrName, testAccVirtualMFADeviceARNPattern(deviceName))
}

func TestUnitIAMVirtualMFADevice_seedStateEncoding(t *testing.T) {
	const wireSeed = "TB3WJRQBFWGZYTPRYHLFMQRQPGN6NTNZKRNJDRJDRO7MPONUDEZYXC5KY4IFP5DZ"

	decodedSeed, err := base64.StdEncoding.DecodeString(wireSeed)
	if err != nil {
		t.Fatalf("decoding IAM wire seed: %s", err)
	}

	stateSeed := base64.StdEncoding.EncodeToString(decodedSeed)
	if stateSeed != wireSeed {
		t.Fatalf("state seed = %q, want %q", stateSeed, wireSeed)
	}

	secret, err := normalizeVirtualMFADeviceSeed([]byte(stateSeed))
	if err != nil {
		t.Fatalf("normalizing state seed: %s", err)
	}
	if secret != wireSeed {
		t.Fatalf("TOTP secret = %q, want %q", secret, wireSeed)
	}
}

func timeValueString(v *time.Time) string {
	if v == nil {
		return ""
	}

	return v.Format(time.RFC3339)
}

func enableVirtualMFADeviceForUser(ctx context.Context, t *testing.T, userName, serialNumber string, seed []byte) error {
	secret, err := normalizeVirtualMFADeviceSeed(seed)
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

	conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
	_, err = conn.EnableMFADevice(ctx, &iam.EnableMFADeviceInput{
		AuthenticationCode1: aws.String(code1),
		AuthenticationCode2: aws.String(code2),
		SerialNumber:        aws.String(serialNumber),
		UserName:            aws.String(userName),
	})
	if err != nil {
		return fmt.Errorf("enabling IAM Virtual MFA Device (%s) for IAM User (%s): %w", serialNumber, userName, err)
	}

	verification, err := conn.ListMFADevices(ctx, &iam.ListMFADevicesInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		return fmt.Errorf("listing MFA devices for IAM User (%s): %w", userName, err)
	}

	for _, device := range verification.MFADevices {
		if aws.ToString(device.SerialNumber) == serialNumber {
			return nil
		}
	}

	return fmt.Errorf("IAM Virtual MFA Device (%s) not attached to IAM User (%s)", serialNumber, userName)
}

// TC-IAM-MFA-01-Basic verifies the basic CRUD flow of a virtual MFA device: create the device, read key attributes, import into state, and confirm the resource is destroyed after the test.
func TestAccIAMVirtualMFADevice_basic(t *testing.T) {
	ctx := acctest.Context(t)
	var conf awstypes.VirtualMFADevice
	resourceName := "ctyun_iam_virtual_mfa_device.test"

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckVirtualMFADeviceDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualMFADeviceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckVirtualMFADeviceExists(ctx, t, resourceName, &conf),
					testAccCheckResourceAttrOOSIAMMFADeviceARN(resourceName, names.AttrARN, rName),
					testAccCheckResourceAttrOOSIAMMFADeviceARN(resourceName, "serial_number", rName),
					testAccCheckVirtualMFADeviceStateSeed(resourceName),
					testAccCheckVirtualMFADeviceUnbound(resourceName),
					testAccCheckVirtualMFADeviceStateQRCodePNG(resourceName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateCheck: acctest.ComposeAggregateImportStateCheckFunc(
					acctest.ImportMatchResourceAttr(names.AttrARN, testAccVirtualMFADeviceARNPattern(rName)),
					acctest.ImportMatchResourceAttr("serial_number", testAccVirtualMFADeviceARNPattern(rName)),
					acctest.ImportMatchResourceAttr(names.AttrID, testAccVirtualMFADeviceARNPattern(rName)),
					acctest.ImportCheckResourceAttr(names.AttrUserName, ""),
					acctest.ImportCheckResourceAttr("enable_date", ""),
					acctest.ImportCheckResourceAttr("virtual_mfa_device_name", rName),
				),
				ImportStateVerifyIgnore: []string{
					"base_32_string_seed",
					"qr_code_png",
				},
			},
		},
	})
}

// TC-IAM-MFA-02-NameBoundary verifies the local validation rules of virtual_mfa_device_name on ctyun_iam_virtual_mfa_device: valid special characters pass; empty values, over-length values, and illegal characters are rejected.
func TestUnitIAMVirtualMFADevice_nameValidation(t *testing.T) {
	t.Parallel()

	nameSchema, ok := tfiam.ResourceVirtualMFADevice().Schema["virtual_mfa_device_name"]
	if !ok || nameSchema == nil {
		t.Fatal("expected virtual_mfa_device_name schema")
	}
	if !nameSchema.Required {
		t.Fatal("expected virtual_mfa_device_name to be required")
	}
	if nameSchema.Optional {
		t.Fatal("expected virtual_mfa_device_name not to be optional")
	}
	if nameSchema.Computed {
		t.Fatal("expected virtual_mfa_device_name not to be computed")
	}
	if !nameSchema.ForceNew {
		t.Fatal("expected virtual_mfa_device_name changes to force a new device")
	}

	validate := nameSchema.ValidateFunc
	if validate == nil {
		t.Fatal("expected virtual_mfa_device_name validation")
	}

	for _, tc := range []struct {
		name    string
		wantErr bool
	}{
		{name: "example_01-foo,bar.baz+qux=zed@example", wantErr: false},
		{name: "", wantErr: true},
		{name: strings.Repeat("a", 129), wantErr: true},
		{name: "example/name", wantErr: true},
		{name: "example name", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, errs := validate(tc.name, "virtual_mfa_device_name")
			if gotErr := len(errs) > 0; gotErr != tc.wantErr {
				t.Fatalf("ValidateFunc(%q) error = %t, want %t: %v", tc.name, gotErr, tc.wantErr, errs)
			}
		})
	}
}

// TC-IAM-MFA-02-NameBoundary verifies the valid-special-character name scenario for a virtual MFA device: a name containing =, ,, ., @, -, _, and + can be created, read, and destroyed successfully.
func TestAccIAMVirtualMFADevice_nameSpecialCharacters(t *testing.T) {
	ctx := acctest.Context(t)
	var conf awstypes.VirtualMFADevice
	resourceName := "ctyun_iam_virtual_mfa_device.test"

	rName := acctest.RandomWithPrefix(t, "tf-acc-test-=,.@_+")

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckVirtualMFADeviceDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualMFADeviceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckVirtualMFADeviceExists(ctx, t, resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "virtual_mfa_device_name", rName),
				),
			},
		},
	})
}

// TC-IAM-MFA-02-NameBoundary verifies the minimum name length boundary for a virtual MFA device: a name of length 1 can be created, read, and destroyed successfully.
func TestAccIAMVirtualMFADevice_nameLengthValidation_min(t *testing.T) {
	ctx := acctest.Context(t)
	var conf awstypes.VirtualMFADevice
	resourceName := "ctyun_iam_virtual_mfa_device.test"
	nameMinLength := "a"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckVirtualMFADeviceDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualMFADeviceConfig_basic(nameMinLength),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckVirtualMFADeviceExists(ctx, t, resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "virtual_mfa_device_name", nameMinLength),
				),
			},
		},
	})
}

// TC-IAM-MFA-02-NameBoundary verifies the maximum name length boundary for a virtual MFA device: a name of length 128 can be created, read, and destroyed successfully.
func TestAccIAMVirtualMFADevice_nameLengthValidation_max(t *testing.T) {
	ctx := acctest.Context(t)
	var conf awstypes.VirtualMFADevice
	resourceName := "ctyun_iam_virtual_mfa_device.test"
	nameMaxLength := strings.Repeat("a", 128)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckVirtualMFADeviceDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualMFADeviceConfig_basic(nameMaxLength),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckVirtualMFADeviceExists(ctx, t, resourceName, &conf),
					resource.TestCheckResourceAttr(resourceName, "virtual_mfa_device_name", nameMaxLength),
				),
			},
		},
	})
}

// TC-IAM-MFA-02-NameBoundary verifies that an empty-string virtual MFA device name is rejected by the provider schema in ACC.
func TestAccIAMVirtualMFADevice_nameLengthValidation_empty(t *testing.T) {
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccVirtualMFADeviceConfig_basic(""),
				ExpectError: regexache.MustCompile(`expected length of virtual_mfa_device_name to be in the range \(1 - 128\), got`),
			},
		},
	})
}

// TC-IAM-MFA-02-NameBoundary verifies that a virtual MFA device name of length 129 is rejected by the provider schema in ACC.
func TestAccIAMVirtualMFADevice_nameLengthValidation_tooLong(t *testing.T) {
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccVirtualMFADeviceConfig_basic(strings.Repeat("a", 129)),
				ExpectError: regexache.MustCompile(`expected length of virtual_mfa_device_name to be in the range \(1 - 128\), got`),
			},
		},
	})
}

// TC-IAM-MFA-02-NameBoundary verifies that a virtual MFA device name containing a space is rejected by the provider schema in ACC.
func TestAccIAMVirtualMFADevice_nameInvalidCharacters_space(t *testing.T) {
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccVirtualMFADeviceConfig_basic("invalid name"),
				ExpectError: regexache.MustCompile(`must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs`),
			},
		},
	})
}

// TC-IAM-MFA-02-NameBoundary verifies that a virtual MFA device name containing a slash is rejected by the provider schema in ACC.
func TestAccIAMVirtualMFADevice_nameInvalidCharacters_invalidCharacter(t *testing.T) {
	ctx := acctest.Context(t)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccVirtualMFADeviceConfig_basic("invalid/name"),
				ExpectError: regexache.MustCompile(`must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs`),
			},
		},
	})
}

// TC-IAM-MFA-02-NameBoundary / TC-IAM-MFA-05-ImportOrDuplicate verifies the duplicate-name conflict scenario for a virtual MFA device: creating another device with a name already taken fails in ACC.
func TestAccIAMVirtualMFADevice_duplicateName(t *testing.T) {
	ctx := acctest.Context(t)
	var conf awstypes.VirtualMFADevice
	resourceName := "ctyun_iam_virtual_mfa_device.test"

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckVirtualMFADeviceDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualMFADeviceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckVirtualMFADeviceExists(ctx, t, resourceName, &conf),
				),
			},
			{
				Config:      testAccVirtualMFADeviceConfig_duplicateNameConflict(rName),
				ExpectError: regexache.MustCompile(`(?i)(EntityAlreadyExists|already exists|duplicate)`),
			},
		},
	})
}

// TC-IAM-MFA-03-ListVisibility verifies that a virtual MFA device is visible in the list read path after creation: it is read back via the ctyun_iam_virtual_mfa_devices data source, confirming the new device's serial_number appears in the list.
func TestAccIAMVirtualMFADevice_listVisibility(t *testing.T) {
	ctx := acctest.Context(t)
	var conf awstypes.VirtualMFADevice
	resourceName := "ctyun_iam_virtual_mfa_device.test"
	dataSourceName := "data.ctyun_iam_virtual_mfa_devices.test"

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckVirtualMFADeviceDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualMFADeviceConfig_listVisibility(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckVirtualMFADeviceExists(ctx, t, resourceName, &conf),
					testAccCheckVirtualMFADevicesDataSourceEqualsDevice(dataSourceName, &conf),
				),
			},
		},
	})
}

// TC-IAM-MFA-04-Disappears deletes the virtual MFA device via the resource Delete implementation, then verifies that refresh detects the resource is gone and produces a non-empty plan for subsequent re-creation.
func TestAccIAMVirtualMFADevice_disappears(t *testing.T) {
	ctx := acctest.Context(t)
	var conf awstypes.VirtualMFADevice
	resourceName := "ctyun_iam_virtual_mfa_device.test"

	rName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckVirtualMFADeviceDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualMFADeviceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckVirtualMFADeviceExists(ctx, t, resourceName, &conf),
					acctest.CheckSDKResourceDisappears(ctx, t, tfiam.ResourceVirtualMFADevice(), resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TC-IAM-MFA-06-Enable/Deactivate Readback verifies that after Terraform creates a virtual MFA device, binding it to a user externally and refreshing back-fills enable_date/user_name; after an external unbind, another refresh keeps the historical enable_date and clears user_name.
func TestAccIAMVirtualMFADevice_enabledReadback(t *testing.T) {
	ctx := acctest.Context(t)
	resourceName := "ctyun_iam_virtual_mfa_device.test"
	userResourceName := "ctyun_iam_user.test"
	userName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	deviceName := acctest.RandomWithPrefix(t, acctest.ResourcePrefix)
	var user awstypes.User

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.IAMServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckVirtualMFADeviceDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualMFADeviceConfig_enabledReadback(userName, deviceName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckUserExists(ctx, t, userResourceName, &user),
					testAccCheckVirtualMFADeviceUnbound(resourceName),
					testAccCheckVirtualMFADeviceEnableExternal(ctx, t, resourceName, &user),
				),
			},
			{
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckVirtualMFADeviceEnabled(resourceName, userName),
				),
			},
			{
				Config: testAccVirtualMFADeviceConfig_enabledReadback(userName, deviceName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckVirtualMFADeviceDeactivateExternal(ctx, t, resourceName, &user),
				),
			},
			{
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckVirtualMFADeviceDeactivated(resourceName),
				),
			},
		},
	})
}

func testAccCheckVirtualMFADeviceDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ctyun_iam_virtual_mfa_device" {
				continue
			}

			output, err := tfiam.FindVirtualMFADeviceBySerialNumber(ctx, conn, rs.Primary.ID)

			if retry.NotFound(err) {
				continue
			}
			if err != nil {
				return fmt.Errorf("reading IAM Virtual MFA Device (%s) during destroy check: %w", rs.Primary.ID, err)
			}

			if output != nil {
				return fmt.Errorf("IAM Virtual MFA Device (%s) still exists", rs.Primary.ID)
			}
		}

		return nil
	}
}

func testAccCheckVirtualMFADeviceExists(ctx context.Context, t *testing.T, n string, v *awstypes.VirtualMFADevice) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return errors.New("No Virtual MFA Device ID is set")
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)

		output, err := tfiam.FindVirtualMFADeviceBySerialNumber(ctx, conn, rs.Primary.ID)

		if err != nil {
			return err
		}

		*v = *output

		return nil
	}
}

func testAccCheckVirtualMFADeviceUnbound(resourceName string) resource.TestCheckFunc {
	return resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(resourceName, "enable_date", ""),
		resource.TestCheckResourceAttr(resourceName, names.AttrUserName, ""),
	)
}

func testAccCheckVirtualMFADeviceEnabled(resourceName, userName string) resource.TestCheckFunc {
	return resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttrSet(resourceName, "enable_date"),
		resource.TestCheckResourceAttr(resourceName, names.AttrUserName, userName),
	)
}

func testAccCheckVirtualMFADeviceDeactivated(resourceName string) resource.TestCheckFunc {
	return resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttrSet(resourceName, "enable_date"),
		resource.TestCheckResourceAttr(resourceName, names.AttrUserName, ""),
	)
}

func testAccCheckVirtualMFADeviceStateSeed(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		seed := rs.Primary.Attributes["base_32_string_seed"]
		secret, err := normalizeVirtualMFADeviceSeed([]byte(seed))
		if err != nil {
			return fmt.Errorf("normalizing %s base_32_string_seed: %w", resourceName, err)
		}
		if seed != secret {
			return fmt.Errorf("%s base_32_string_seed is not canonical Base32", resourceName)
		}

		return nil
	}
}

func testAccCheckVirtualMFADeviceStateQRCodePNG(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		qrCodePNG, err := base64.StdEncoding.DecodeString(rs.Primary.Attributes["qr_code_png"])
		if err != nil {
			return fmt.Errorf("decoding %s qr_code_png: %w", resourceName, err)
		}
		if !bytes.HasPrefix(qrCodePNG, []byte{'\x89', 'P', 'N', 'G', '\r', '\n', '\x1a', '\n'}) {
			return fmt.Errorf("%s qr_code_png does not contain a PNG image", resourceName)
		}

		return nil
	}
}

func testAccCheckVirtualMFADeviceEnableExternal(ctx context.Context, t *testing.T, resourceName string, user *awstypes.User) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		serialNumber := rs.Primary.ID
		seed := []byte(rs.Primary.Attributes["base_32_string_seed"])
		if serialNumber == "" || len(seed) == 0 {
			return fmt.Errorf("IAM Virtual MFA Device (%s) serial number or seed not recorded in state", resourceName)
		}

		if user == nil {
			return fmt.Errorf("IAM User not recorded after GetUser")
		}

		userName := aws.ToString(user.UserName)
		if userName == "" {
			return fmt.Errorf("IAM User name not recorded after GetUser")
		}

		return enableVirtualMFADeviceForUser(ctx, t, userName, serialNumber, seed)
	}
}

func testAccCheckVirtualMFADeviceDeactivateExternal(ctx context.Context, t *testing.T, resourceName string, user *awstypes.User) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}
		if user == nil || aws.ToString(user.UserName) == "" {
			return fmt.Errorf("IAM User not recorded after GetUser")
		}

		serialNumber := rs.Primary.ID
		if serialNumber == "" {
			return fmt.Errorf("IAM Virtual MFA Device (%s) serial number not recorded in state", resourceName)
		}

		conn := acctest.ProviderMeta(ctx, t).IAMClient(ctx)
		_, err := conn.DeactivateMFADevice(ctx, &iam.DeactivateMFADeviceInput{
			SerialNumber: aws.String(serialNumber),
			UserName:     user.UserName,
		})
		if err != nil {
			return fmt.Errorf("deactivating IAM Virtual MFA Device (%s) for IAM User (%s): %w", serialNumber, aws.ToString(user.UserName), err)
		}

		return nil
	}
}

func testAccVirtualMFADeviceConfig_enabledReadback(userName, deviceName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_user" "test" {
  name = %[1]q
}

resource "ctyun_iam_virtual_mfa_device" "test" {
  virtual_mfa_device_name = %[2]q
  depends_on              = [ctyun_iam_user.test]
}
`, userName, deviceName)
}

func testAccCheckVirtualMFADevicesDataSourceEqualsDevice(dataSourceName string, device *awstypes.VirtualMFADevice) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ds, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not Found: %s", dataSourceName)
		}

		if device == nil || device.SerialNumber == nil {
			return fmt.Errorf("device serial number not available for comparison")
		}

		wantSerialNumber := aws.ToString(device.SerialNumber)
		wantEnableDate := timeValueString(device.EnableDate)
		wantUserName := ""

		if device.User != nil {
			wantUserName = aws.ToString(device.User.UserName)
		}

		n, err := strconv.Atoi(ds.Primary.Attributes["virtual_mfa_devices.#"])
		if err != nil {
			return err
		}

		for i := range n {
			prefix := fmt.Sprintf("virtual_mfa_devices.%d.", i)
			if ds.Primary.Attributes[prefix+"serial_number"] != wantSerialNumber {
				continue
			}
			if got := ds.Primary.Attributes[prefix+"enable_date"]; got != wantEnableDate {
				return fmt.Errorf("data source enable_date = %q, want %q", got, wantEnableDate)
			}
			if got := ds.Primary.Attributes[prefix+"user.user_name"]; got != wantUserName {
				return fmt.Errorf("data source user.user_name = %q, want %q", got, wantUserName)
			}
			return nil
		}

		return fmt.Errorf("IAM Virtual MFA Device %q not found in %s virtual_mfa_devices", wantSerialNumber, dataSourceName)
	}
}

func testAccVirtualMFADeviceConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_virtual_mfa_device" "test" {
  virtual_mfa_device_name = %[1]q
}
`, rName)
}

func testAccVirtualMFADeviceConfig_duplicateNameConflict(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_virtual_mfa_device" "test" {
  virtual_mfa_device_name = %[1]q
}

resource "ctyun_iam_virtual_mfa_device" "other" {
  virtual_mfa_device_name = %[1]q
}
`, rName)
}

func testAccVirtualMFADeviceConfig_listVisibility(rName string) string {
	return fmt.Sprintf(`
resource "ctyun_iam_virtual_mfa_device" "test" {
  virtual_mfa_device_name = %[1]q
}

data "ctyun_iam_virtual_mfa_devices" "test" {
  depends_on = [ctyun_iam_virtual_mfa_device.test]
}
`, rName)
}
