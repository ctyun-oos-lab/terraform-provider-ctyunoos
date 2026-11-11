// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeDoc creates dir/sub/name with the given content and returns the parsed doc.
func writeDoc(t *testing.T, content string) *parsedDoc {
	t.Helper()
	path := filepath.Join(t.TempDir(), "doc.html.markdown")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := parseDoc(path)
	if err != nil {
		t.Fatalf("parseDoc: %s", err)
	}
	return d
}

const validResource = `---
subcategory: "S3 (Simple Storage)"
page_title: "CTYUN OOS: ctyun_oos_bucket"
description: |-
  Provides an OOS storage bucket resource.
---

# Resource: ctyun_oos_bucket

## Example Usage

` + "```terraform" + `
resource "ctyun_oos_bucket" "example" {
  bucket = "example"
}
` + "```" + `

## Argument Reference

* ` + "`bucket`" + ` - (Optional) Name.

## Attribute Reference

* ` + "`arn`" + ` - ARN.

## Import

` + "```console" + `
terraform import ctyun_oos_bucket.example example
` + "```" + `
`

func TestValidResource(t *testing.T) {
	t.Parallel()
	d := writeDoc(t, validResource)
	if fs := checkFile("r/ctyun_oos_bucket.html.markdown", kindResource, d); len(fs) != 0 {
		t.Fatalf("expected no findings, got: %v", fs)
	}
}

func TestMissingFrontMatter(t *testing.T) {
	t.Parallel()
	d := writeDoc(t, "# Resource: ctyun_oos_bucket\n")
	fs := checkFile("r/x.html.markdown", kindResource, d)
	if len(fs) != 1 || !strings.Contains(fs[0].msg, "front matter") {
		t.Fatalf("expected front matter finding, got: %v", fs)
	}
}

func TestWrongH1Prefix(t *testing.T) {
	t.Parallel()
	doc := strings.Replace(validResource, "# Resource: ctyun_oos_bucket", "# Data Source: ctyun_oos_bucket", 1)
	d := writeDoc(t, doc)
	fs := checkFile("r/x.html.markdown", kindResource, d)
	if !hasMsg(fs, "must start with") {
		t.Fatalf("expected H1 prefix finding, got: %v", fs)
	}
}

func TestH1TypeMismatch(t *testing.T) {
	t.Parallel()
	doc := strings.Replace(validResource, "# Resource: ctyun_oos_bucket", "# Resource: ctyun_oos_other", 1)
	d := writeDoc(t, doc)
	fs := checkFile("r/x.html.markdown", kindResource, d)
	if !hasMsg(fs, "does not match page_title type") {
		t.Fatalf("expected H1/title mismatch finding, got: %v", fs)
	}
}

func TestMissingImportSection(t *testing.T) {
	t.Parallel()
	doc := strings.Replace(validResource, "## Import\n", "## NotImport\n", 1)
	d := writeDoc(t, doc)
	fs := checkFile("r/x.html.markdown", kindResource, d)
	if !hasMsg(fs, `missing required section "Import"`) {
		t.Fatalf("expected missing Import finding, got: %v", fs)
	}
}

func TestDataSourceNoImportRequired(t *testing.T) {
	t.Parallel()
	doc := strings.Replace(validResource, "# Resource: ctyun_oos_bucket", "# Data Source: ctyun_oos_bucket", 1)
	doc = strings.Replace(doc, "## Import\n"+"```console"+"\nterraform import ctyun_oos_bucket.example example\n"+"```"+"\n", "", 1)
	doc = strings.Replace(doc, `resource "ctyun_oos_bucket"`, `data "ctyun_oos_bucket"`, 1)
	d := writeDoc(t, doc)
	if fs := checkFile("d/x.html.markdown", kindDataSource, d); len(fs) != 0 {
		t.Fatalf("expected no findings for data source, got: %v", fs)
	}
}

func TestSectionsOutOfOrder(t *testing.T) {
	t.Parallel()
	doc := `---
page_title: "CTYUN OOS: ctyun_oos_bucket"
description: |-
  desc
subcategory: "S3 (Simple Storage)"
---

# Resource: ctyun_oos_bucket

## Argument Reference

## Example Usage

` + "```terraform" + `
resource "ctyun_oos_bucket" "x" {}
` + "```" + `

## Attribute Reference

## Import
`
	d := writeDoc(t, doc)
	fs := checkFile("r/x.html.markdown", kindResource, d)
	if !hasMsg(fs, "out of order") {
		t.Fatalf("expected out-of-order finding, got: %v", fs)
	}
}

func TestUnknownSubcategory(t *testing.T) {
	t.Parallel()
	doc := strings.Replace(validResource, `subcategory: "S3 (Simple Storage)"`, `subcategory: "Nope"`, 1)
	d := writeDoc(t, doc)
	fs := checkFile("r/x.html.markdown", kindResource, d)
	if !hasMsg(fs, "not one of the known service subcategories") {
		t.Fatalf("expected subcategory finding, got: %v", fs)
	}
}

func TestNoTerraformDeclaringOwnType(t *testing.T) {
	t.Parallel()
	doc := strings.Replace(validResource, `resource "ctyun_oos_bucket" "example"`, `resource "ctyun_oos_other" "example"`, 1)
	d := writeDoc(t, doc)
	fs := checkFile("r/x.html.markdown", kindResource, d)
	if !hasMsg(fs, "no terraform example declares") {
		t.Fatalf("expected missing-declaration finding, got: %v", fs)
	}
}

func TestUnterminatedFence(t *testing.T) {
	t.Parallel()
	doc := `---
page_title: "CTYUN OOS: ctyun_oos_bucket"
description: |-
  desc
subcategory: "S3 (Simple Storage)"
---

# Resource: ctyun_oos_bucket

## Example Usage

` + "```terraform" + `
resource "ctyun_oos_bucket" "x" {}
`
	d := writeDoc(t, doc)
	fs := checkFile("r/x.html.markdown", kindResource, d)
	if !hasMsg(fs, "unterminated code fence") {
		t.Fatalf("expected unterminated fence finding, got: %v", fs)
	}
}

func hasMsg(fs []finding, sub string) bool {
	for _, f := range fs {
		if strings.Contains(f.msg, sub) {
			return true
		}
	}
	return false
}
