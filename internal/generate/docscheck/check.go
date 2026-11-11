// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// docKind classifies the pages that the OOS template covers.
type docKind int

const (
	kindResource docKind = iota
	kindDataSource
	kindProvider
)

// finding is a single documentation problem, anchored to a file and line.
type finding struct {
	file string
	line int
	msg  string
}

func (f finding) String() string {
	if f.line > 0 {
		return fmt.Sprintf("%s:%d: %s", f.file, f.line, f.msg)
	}
	return fmt.Sprintf("%s: %s", f.file, f.msg)
}

// parsedDoc holds the parts of a documentation file the rules care about.
type parsedDoc struct {
	frontMatter    map[string]string
	frontMatterEnd int      // 1-based line of the closing "---"; 0 if unterminated
	h1             string   // text after "# "
	h1Line         int      // 1-based line of the H1
	sections       []string // H2 headings in order of appearance
	terraformTypes []string // resource/data type names declared in terraform fences
	openFence      int      // 1-based line of an unterminated fence; 0 if all closed
}

const (
	// Buffer sizes for reading documentation files
	scannerInitialBufferSize = 64 * 1024
	scannerMaxBufferSize     = 1024 * 1024
)

var (
	frontMatterKeyRE = regexp.MustCompile(`^([a-z_]+):\s*(.*)$`)
	tfDeclRE         = regexp.MustCompile(`^\s*(?:resource|data)\s+"([a-z0-9_]+)"`)
)

// parseDoc reads a Markdown file and extracts the structural elements. It never
// fails on content; structural problems are reported by the rules instead.
func parseDoc(path string) (*parsedDoc, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

	d := &parsedDoc{frontMatter: map[string]string{}}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, scannerInitialBufferSize), scannerMaxBufferSize)

	var (
		lineNo        int
		inFrontMatter bool
		frontDone     bool
		inFence       bool
		fenceLang     string
		fenceStart    int
	)

	for scanner.Scan() {
		lineNo++
		line := scanner.Text()

		// Front matter is only recognized when it opens on the very first line.
		if lineNo == 1 && strings.TrimSpace(line) == "---" {
			inFrontMatter = true
			continue
		}
		if inFrontMatter {
			if strings.TrimSpace(line) == "---" {
				inFrontMatter = false
				frontDone = true
				d.frontMatterEnd = lineNo
				continue
			}
			if m := frontMatterKeyRE.FindStringSubmatch(line); m != nil {
				d.frontMatter[m[1]] = strings.TrimSpace(strings.Trim(m[2], `"`))
			}
			continue
		}
		_ = frontDone

		// Track fenced code blocks.
		if strings.HasPrefix(line, "```") {
			if inFence {
				inFence = false
				fenceLang = ""
			} else {
				inFence = true
				fenceLang = strings.TrimSpace(strings.TrimPrefix(line, "```"))
				fenceStart = lineNo
			}
			continue
		}
		if inFence {
			if fenceLang == "terraform" {
				if m := tfDeclRE.FindStringSubmatch(line); m != nil {
					d.terraformTypes = append(d.terraformTypes, m[1])
				}
			}
			continue
		}

		if strings.HasPrefix(line, "# ") && d.h1 == "" {
			d.h1 = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			d.h1Line = lineNo
			continue
		}
		if strings.HasPrefix(line, "## ") {
			d.sections = append(d.sections, strings.TrimSpace(strings.TrimPrefix(line, "## ")))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if inFence {
		d.openFence = fenceStart
	}
	return d, nil
}

// validSubcategories lists the subcategory values allowed in front matter. These
// mirror the human_friendly names in names/data/names_data.hcl.
var validSubcategories = map[string]bool{
	"IAM (Identity & Access Management)": true,
	"S3 (Simple Storage)":                true,
	"CloudTrail":                         true,
}

// requiredResourceSections and requiredDataSourceSections are the H2 headings
// that must appear, in this order, for each page kind.
var (
	requiredResourceSections   = []string{"Example Usage", "Argument Reference", "Attribute Reference", "Import"}
	requiredDataSourceSections = []string{"Example Usage", "Argument Reference", "Attribute Reference"}
)

// checkFile applies the template rules to a single parsed document.
func checkFile(rel string, kind docKind, d *parsedDoc) []finding {
	var fs []finding
	add := func(line int, format string, args ...any) {
		fs = append(fs, finding{file: rel, line: line, msg: fmt.Sprintf(format, args...)})
	}

	// Front matter.
	if d.frontMatterEnd == 0 {
		add(1, "front matter must open on line 1 and be closed with ---")
		return fs
	}
	if d.frontMatter["page_title"] == "" {
		add(d.frontMatterEnd, "front matter is missing a page_title")
	}
	if d.frontMatter["description"] == "" {
		add(d.frontMatterEnd, "front matter is missing a description")
	}

	// Validate front matter description format
	desc := d.frontMatter["description"]
	if desc != "" && !strings.HasPrefix(desc, "|-") {
		// Description should use literal block scalar style in YAML
		add(d.frontMatterEnd, "front matter description should use '|-' block scalar style")
	}

	// Unterminated fence makes the rest unreliable; report and stop.
	if d.openFence != 0 {
		add(d.openFence, "unterminated code fence")
		return fs
	}

	switch kind {
	case kindProvider:
		if d.h1 == "" {
			add(0, "provider page must have an H1 heading")
		}
	case kindResource, kindDataSource:
		checkEntity(add, rel, kind, d)
	}
	return fs
}

func checkEntity(add func(int, string, ...any), _ string, kind docKind, d *parsedDoc) {
	sub := d.frontMatter["subcategory"]
	if sub == "" {
		add(d.frontMatterEnd, "front matter is missing a subcategory")
	} else if !validSubcategories[sub] {
		add(d.frontMatterEnd, "subcategory %q is not one of the known service subcategories", sub)
	}

	// page_title entity name: "CTYUN OOS: <type>".
	title := d.frontMatter["page_title"]
	titleType := ""
	if i := strings.LastIndex(title, ": "); i != -1 {
		titleType = strings.TrimSpace(title[i+2:])
	}
	if !strings.HasPrefix(titleType, "ctyun_") {
		add(d.frontMatterEnd, "page_title %q must end with the ctyun_ type name", title)
	}

	// H1: "Resource: <type>" or "Data Source: <type>".
	wantPrefix := "Resource: "
	if kind == kindDataSource {
		wantPrefix = "Data Source: "
	}
	if d.h1 == "" {
		add(0, "missing H1 heading")
	} else if !strings.HasPrefix(d.h1, wantPrefix) {
		add(d.h1Line, "H1 %q must start with %q", d.h1, wantPrefix)
	} else {
		h1Type := strings.TrimSpace(strings.TrimPrefix(d.h1, wantPrefix))
		if titleType != "" && h1Type != titleType {
			add(d.h1Line, "H1 type %q does not match page_title type %q", h1Type, titleType)
		}
	}

	// Required sections in order.
	want := requiredResourceSections
	if kind == kindDataSource {
		want = requiredDataSourceSections
	}
	checkSectionOrder(add, d, want)

	// At least one terraform fence must declare this page's own type.
	if titleType != "" {
		found := false
		for _, t := range d.terraformTypes {
			if t == titleType {
				found = true
				break
			}
		}
		if !found {
			add(0, "no terraform example declares %q", titleType)
		}
	}
}

// checkSectionOrder verifies each required section is present exactly once and
// that they appear in the required relative order.
func checkSectionOrder(add func(int, string, ...any), d *parsedDoc, want []string) {
	pos := map[string]int{}
	for i, s := range d.sections {
		if _, seen := pos[s]; !seen {
			pos[s] = i
		}
	}
	last := -1
	for _, w := range want {
		i, ok := pos[w]
		if !ok {
			add(0, "missing required section %q", w)
			continue
		}
		if i < last {
			add(0, "section %q is out of order", w)
		}
		last = i
	}
}

// detailedCheck performs detailed line-by-line checks on the file content.
func detailedCheck(path string, rel string, _ docKind) ([]finding, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

	var fs []finding
	add := func(line int, format string, args ...any) {
		fs = append(fs, finding{file: rel, line: line, msg: fmt.Sprintf(format, args...)})
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, scannerInitialBufferSize), scannerMaxBufferSize)

	lineNo := 0
	inFence := false
	fenceLang := ""
	curBlock := ""

	for scanner.Scan() {
		lineNo++
		line := scanner.Text()

		// Track fence blocks
		if strings.HasPrefix(line, "```") {
			if !inFence {
				inFence = true
				fenceLang = strings.TrimSpace(strings.TrimPrefix(line, "```"))
				curBlock = ""
				// Validate fence language for examples
				if fenceLang != "" && fenceLang != "terraform" && fenceLang != "console" && fenceLang != "shell" && fenceLang != "hcl" {
					add(lineNo, "code fence should use 'terraform', 'console', 'shell', or 'hcl' as language, got %q", fenceLang)
				}
			} else {
				inFence = false
				// Closing fence should be bare ```
				if strings.TrimSpace(line) != "```" {
					add(lineNo, "closing code fence should be bare '```'")
				}
				fenceLang = ""
			}
			continue
		}

		if inFence {
			// Check for common issues in terraform blocks
			if fenceLang == "terraform" {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, `resource "`) {
					curBlock = "resource"
				} else if strings.HasPrefix(trimmed, `data "`) {
					curBlock = "data"
				}
				// Warn about 'test' in resource names
				if strings.Contains(trimmed, `"test`) || strings.Contains(trimmed, "test-") {
					if strings.Contains(trimmed, "=") {
						parts := strings.SplitN(trimmed, "=", 2)
						if len(parts) == 2 {
							key := strings.TrimSpace(parts[0])
							value := strings.TrimSpace(parts[1])
							// Check if this is a name-like field with 'test' in the value
							if (strings.HasSuffix(key, "name") || key == "bucket" || key == "id") &&
								(strings.Contains(value, `"test`) || strings.Contains(value, "test-")) {
								add(lineNo, "avoid using 'test' in example values for name fields; use more descriptive examples like 'example' or 'demo'")
							}
						}
					}
				}
				// Warn about depends_on in resource examples. Data sources that
				// list resources created earlier in the same example often require
				// an explicit depends_on because no argument reference is available.
				if curBlock == "resource" && strings.Contains(trimmed, "depends_on") && strings.Contains(trimmed, "=") {
					add(lineNo, "avoid using 'depends_on' in examples; implicit dependencies are preferred")
				}
			}
			continue
		}

		// Check for common markdown issues outside fences
		// Check argument/attribute list formatting
		if strings.HasPrefix(line, "* `") {
			// Extract the part before the closing backtick and dash
			if idx := strings.Index(line[3:], "`"); idx != -1 {
				afterField := line[3+idx+1:]
				// Check for proper spacing after field name
				if !strings.HasPrefix(afterField, " - ") {
					add(lineNo, "argument/attribute should follow format: * `field` - description")
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return fs, nil
}

// run walks the resources/, data-sources/ and index pages under dir and returns
// all findings, sorted by file then line for stable output.
func run(dir string) ([]finding, error) {
	var all []finding

	index := filepath.Join(dir, "index.md")
	if _, err := os.Stat(index); err == nil {
		d, err := parseDoc(index)
		if err != nil {
			return nil, err
		}
		all = append(all, checkFile("index.md", kindProvider, d)...)
		// Add detailed checks for index page
		if detailed, err := detailedCheck(index, "index.md", kindProvider); err == nil {
			all = append(all, detailed...)
		}
	}

	for _, sub := range []struct {
		dir  string
		kind docKind
	}{
		{"resources", kindResource},
		{"data-sources", kindDataSource},
	} {
		full := filepath.Join(dir, sub.dir)
		entries, err := os.ReadDir(full)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			rel := filepath.Join(sub.dir, e.Name())
			fullPath := filepath.Join(full, e.Name())
			d, err := parseDoc(fullPath)
			if err != nil {
				return nil, err
			}
			all = append(all, checkFile(rel, sub.kind, d)...)
			// Add detailed line-by-line checks
			if detailed, err := detailedCheck(fullPath, rel, sub.kind); err == nil {
				all = append(all, detailed...)
			}
		}
	}

	sort.SliceStable(all, func(i, j int) bool {
		if all[i].file != all[j].file {
			return all[i].file < all[j].file
		}
		return all[i].line < all[j].line
	})
	return all, nil
}
