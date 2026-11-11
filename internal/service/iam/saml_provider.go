// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package iam

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awstypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/conns"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/errs/sdkdiag"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/retry"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/internal/tfresource"
	"github.com/ctyun-oos-lab/terraform-provider-ctyunoos/names"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// @SDKResource("ctyun_iam_saml_provider", name="SAML Provider")
// @Testing(tagsTest=false)
// @ArnIdentity
// @Testing(existsType="github.com/aws/aws-sdk-go-v2/service/iam;iam.GetSAMLProviderOutput", hasNoPreExistingResource=true)
func resourceSAMLProvider() *schema.Resource {
	const (
		nameMaxLength        = 128
		metadataDocMinLength = 1000
		metadataDocMaxLength = 10000000
	)
	return &schema.Resource{
		CreateWithoutTimeout: resourceSAMLProviderCreate,
		ReadWithoutTimeout:   resourceSAMLProviderRead,
		UpdateWithoutTimeout: resourceSAMLProviderUpdate,
		DeleteWithoutTimeout: resourceSAMLProviderDelete,

		Schema: map[string]*schema.Schema{
			names.AttrARN: {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrName: {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validResourceName(nameMaxLength),
			},
			"saml_metadata_document": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringLenBetween(metadataDocMinLength, metadataDocMaxLength),
			},
			"saml_provider_uuid": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"valid_until": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceSAMLProviderCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics

	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	name := d.Get(names.AttrName).(string)
	input := &iam.CreateSAMLProviderInput{
		Name:                 aws.String(name),
		SAMLMetadataDocument: aws.String(d.Get("saml_metadata_document").(string)),
	}

	output, err := conn.CreateSAMLProvider(ctx, input)
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating IAM SAML Provider (%s): %s", name, err)
	}

	d.SetId(aws.ToString(output.SAMLProviderArn))

	return append(diags, resourceSAMLProviderRead(ctx, d, meta)...)
}

func resourceSAMLProviderRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics

	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	output, err := findSAMLProviderByARN(ctx, conn, d.Id())

	if !d.IsNewResource() && retry.NotFound(err) {
		log.Printf("[WARN] IAM SAML Provider %s not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM SAML Provider (%s): %s", d.Id(), err)
	}

	name, err := nameFromSAMLProviderARN(d.Id())

	if err != nil {
		return sdkdiag.AppendFromErr(diags, err)
	}

	d.Set(names.AttrARN, d.Id())
	d.Set(names.AttrName, name)
	d.Set("saml_metadata_document", output.SAMLMetadataDocument)
	d.Set("saml_provider_uuid", output.SAMLProviderUUID)
	if output.ValidUntil != nil {
		d.Set("valid_until", aws.ToTime(output.ValidUntil).Format(time.RFC3339))
	} else {
		d.Set("valid_until", nil)
	}

	return diags
}

func resourceSAMLProviderUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics

	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	input := &iam.UpdateSAMLProviderInput{
		SAMLProviderArn:      aws.String(d.Id()),
		SAMLMetadataDocument: aws.String(d.Get("saml_metadata_document").(string)),
	}

	_, err := conn.UpdateSAMLProvider(ctx, input)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "updating IAM SAML Provider (%s): %s", d.Id(), err)
	}

	return append(diags, resourceSAMLProviderRead(ctx, d, meta)...)
}

func resourceSAMLProviderDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics

	conn := meta.(*conns.AWSClient).IAMClient(ctx)

	log.Printf("[DEBUG] Deleting IAM SAML Provider: %s", d.Id())
	_, err := conn.DeleteSAMLProvider(ctx, &iam.DeleteSAMLProviderInput{
		SAMLProviderArn: aws.String(d.Id()),
	})

	if err != nil && (errs.IsA[*awstypes.NoSuchEntityException](err) || strings.Contains(err.Error(), "StatusCode: 404")) {
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting IAM SAML Provider (%s): %s", d.Id(), err)
	}

	return diags
}

func findSAMLProviderByARN(ctx context.Context, conn *iam.Client, arn string) (*iam.GetSAMLProviderOutput, error) {
	input := &iam.GetSAMLProviderInput{
		SAMLProviderArn: aws.String(arn),
	}

	output, err := conn.GetSAMLProvider(ctx, input)

	if err != nil && (errs.IsA[*awstypes.NoSuchEntityException](err) || strings.Contains(err.Error(), "StatusCode: 404")) {
		return nil, &retry.NotFoundError{
			LastError: err,
		}
	}

	if err != nil {
		return nil, err
	}

	if output == nil {
		return nil, tfresource.NewEmptyResultError()
	}

	return output, nil
}

func nameFromSAMLProviderARN(v string) (string, error) {
	arn, err := arn.Parse(v)

	if err != nil {
		return "", fmt.Errorf("parsing IAM SAML Provider ARN (%s): %w", v, err)
	}

	return strings.TrimPrefix(arn.Resource, "saml-provider/"), nil
}
