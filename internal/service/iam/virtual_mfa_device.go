// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	tfslices "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/slices"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	inttypes "github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/aws-sdk-go-base/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// @SDKResource("ctyun_iam_virtual_mfa_device", name="Virtual MFA Device")
// @Testing(existsType="github.com/aws/aws-sdk-go-v2/service/iam/types;types.VirtualMFADevice", importIgnore="base_32_string_seed;qr_code_png")
func resourceVirtualMFADevice() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceVirtualMFADeviceCreate,
		ReadWithoutTimeout:   resourceVirtualMFADeviceRead,
		DeleteWithoutTimeout: resourceVirtualMFADeviceDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			names.AttrARN: {
				Type:     schema.TypeString,
				Computed: true,
			},
			"base_32_string_seed": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"enable_date": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"qr_code_png": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"serial_number": {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrUserName: {
				Type:     schema.TypeString,
				Computed: true,
			},
			"virtual_mfa_device_name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(1, 128),
					validation.StringMatch(
						regexache.MustCompile(`^[0-9A-Za-z=,.@\-_+]+$`),
						"must only contain alphanumeric characters, hyphens, underscores, commas, periods, @ symbols, plus and equals signs",
					),
				),
			},
		},
	}
}

func resourceVirtualMFADeviceCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	name := d.Get("virtual_mfa_device_name").(string)
	input := iam.CreateVirtualMFADeviceInput{
		VirtualMFADeviceName: aws.String(name),
	}

	output, err := conn.CreateVirtualMFADevice(ctx, &input)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating IAM Virtual MFA Device (%s): %s", name, err)
	}

	vMFA := output.VirtualMFADevice
	d.SetId(aws.ToString(vMFA.SerialNumber))

	seed, err := virtualMFADeviceBase32SeedForState(vMFA.Base32StringSeed)
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "encoding IAM Virtual MFA Device (%s) base_32_string_seed: %s", d.Id(), err)
	}

	// State keeps the Base32 secret, while the PNG requires Base64 to remain text-safe.
	d.Set("base_32_string_seed", seed)
	d.Set("qr_code_png", base64.StdEncoding.EncodeToString(vMFA.QRCodePNG))

	return append(diags, resourceVirtualMFADeviceRead(ctx, d, meta)...)
}

// virtualMFADeviceBase32SeedForState preserves the OOS API contract: state stores the Base32 MFA secret.
// OOS returns that text in a field modeled as a Base64 blob by the AWS SDK, so the SDK may expose decoded bytes.
func virtualMFADeviceBase32SeedForState(seed []byte) (string, error) {
	for _, candidate := range []string{
		string(seed),
		base64.StdEncoding.EncodeToString(seed),
	} {
		candidate = strings.ToUpper(strings.TrimRight(strings.TrimSpace(candidate), "="))
		if candidate == "" {
			continue
		}

		if _, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("value is not Base32 encoded")
}

func resourceVirtualMFADeviceRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	vMFA, err := findVirtualMFADeviceBySerialNumber(ctx, conn, d.Id())

	if !d.IsNewResource() && retry.NotFound(err) {
		log.Printf("[WARN] IAM Virtual MFA Device (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Virtual MFA Device (%s): %s", d.Id(), err)
	}

	d.Set(names.AttrARN, vMFA.SerialNumber)
	d.Set("serial_number", vMFA.SerialNumber)

	_, name, err := parseVirtualMFADeviceARN(aws.ToString(vMFA.SerialNumber))
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Virtual MFA Device (%s): %s", d.Id(), err)
	}

	d.Set("virtual_mfa_device_name", name)

	if v := vMFA.EnableDate; v != nil {
		d.Set("enable_date", aws.ToTime(v).Format(time.RFC3339))
	} else {
		d.Set("enable_date", nil)
	}

	if u := vMFA.User; u != nil {
		d.Set(names.AttrUserName, u.UserName)
	} else {
		d.Set(names.AttrUserName, nil)
	}

	return diags
}

func resourceVirtualMFADeviceDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	if v := d.Get(names.AttrUserName); v != "" {
		input := iam.DeactivateMFADeviceInput{
			SerialNumber: aws.String(d.Id()),
			UserName:     aws.String(v.(string)),
		}
		_, err := conn.DeactivateMFADevice(ctx, &input)

		if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
			return diags
		}

		if err != nil {
			return sdkdiag.AppendErrorf(diags, "deactivating IAM Virtual MFA Device (%s): %s", d.Id(), err)
		}
	}

	log.Printf("[INFO] Deleting IAM Virtual MFA Device: %s", d.Id())
	input := iam.DeleteVirtualMFADeviceInput{
		SerialNumber: aws.String(d.Id()),
	}
	_, err := conn.DeleteVirtualMFADevice(ctx, &input)

	if errs.IsA[*awstypes.NoSuchEntityException](err) || tfawserr.ErrHTTPStatusCodeEquals(err, http.StatusNotFound) {
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting IAM Virtual MFA Device (%s): %s", d.Id(), err)
	}

	return diags
}

func findVirtualMFADeviceBySerialNumber(ctx context.Context, conn *iam.Client, serialNumber string) (*awstypes.VirtualMFADevice, error) {
	var input iam.ListVirtualMFADevicesInput

	return findVirtualMFADevice(ctx, conn, &input, func(v *awstypes.VirtualMFADevice) bool {
		return aws.ToString(v.SerialNumber) == serialNumber
	})
}

func findVirtualMFADevice(ctx context.Context, conn *iam.Client, input *iam.ListVirtualMFADevicesInput, filter tfslices.Predicate[*awstypes.VirtualMFADevice]) (*awstypes.VirtualMFADevice, error) {
	output, err := findVirtualMFADevices(ctx, conn, input, filter)

	if err != nil {
		return nil, err
	}

	return tfresource.AssertSingleValueResult(output)
}

func findVirtualMFADevices(ctx context.Context, conn *iam.Client, input *iam.ListVirtualMFADevicesInput, filter tfslices.Predicate[*awstypes.VirtualMFADevice]) ([]awstypes.VirtualMFADevice, error) {
	var output []awstypes.VirtualMFADevice

	pages := iam.NewListVirtualMFADevicesPaginator(conn, input)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)

		if err != nil {
			return nil, err
		}

		for _, v := range page.VirtualMFADevices {
			if p := &v; !inttypes.IsZero(p) && filter(p) {
				output = append(output, v)
			}
		}
	}

	return output, nil
}

func parseVirtualMFADeviceARN(s string) (path, name string, err error) {
	arn, err := arn.Parse(s)
	if err != nil {
		return "", "", err
	}

	re := regexache.MustCompile(`^mfa(/|/[\x{0021}-\x{007E}]+/)([0-9A-Za-z_+=,.@-]+)$`)
	matches := re.FindStringSubmatch(arn.Resource)
	if len(matches) != 3 {
		return "", "", fmt.Errorf("IAM Virtual MFA Device ARN: invalid resource section (%s)", arn.Resource)
	}

	return matches[1], matches[2], nil
}
