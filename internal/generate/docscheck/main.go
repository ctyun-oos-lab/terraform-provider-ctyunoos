// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

// Command docscheck validates that the Registry documentation under docs
// follows the OOS documentation template. It is a pure validator: it reads the
// documentation on disk, reports every inconsistency it finds, and exits with a
// non-zero status if any were found. It performs no network access and does not
// depend on the current working directory beyond the -dir flag.
package main

import (
	"flag"
	"fmt"
	"os"
)

const (
	exitCodeError   = 2
	exitCodeFinding = 1
)

func main() {
	dir := flag.String("dir", "docs", "path to the provider documentation directory")
	flag.Parse()

	findings, err := run(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docscheck: %s\n", err)
		os.Exit(exitCodeError)
	}

	if len(findings) > 0 {
		for _, f := range findings {
			fmt.Fprintln(os.Stderr, f)
		}
		fmt.Fprintf(os.Stderr, "\ndocscheck: found %d documentation problem(s)\n", len(findings))
		os.Exit(exitCodeFinding)
	}

	fmt.Printf("docscheck: checked documentation under %s, 0 problems.\n", *dir)
}
