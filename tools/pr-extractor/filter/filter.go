package filter

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	// Regex to match the Magic Modules PR URL from the body.
	derivedFromRegex = regexp.MustCompile(`(?i)derived\s+from[^a-zA-Z0-9]*(https://github\.com/GoogleCloudPlatform/magic-modules/pull/\d+)`)
)

// IsTestOrDocFile checks if a file path qualifies as tests or documentation.
func IsTestOrDocFile(filename string) bool {
	// Documentation folders/files
	if strings.HasPrefix(filename, "website/") || strings.HasPrefix(filename, "docs/") {
		return true
	}
	if strings.HasSuffix(filename, ".md") || strings.HasSuffix(filename, ".markdown") {
		return true
	}
	// Changelog files
	if strings.HasPrefix(filename, ".changelog/") {
		return true
	}
	// Go test files
	if strings.HasSuffix(filename, "_test.go") {
		return true
	}
	// Testdata files/fixtures
	if strings.Contains(filename, "/testdata/") || strings.HasPrefix(filename, "testdata/") {
		return true
	}
	return false
}

// OnlyModifiesTestsOrDocs checks if all modified files in a PR are test or doc files.
func OnlyModifiesTestsOrDocs(files []string, verbose bool, prHTMLURL string) bool {
	if len(files) == 0 {
		return false
	}
	for _, file := range files {
		if !IsTestOrDocFile(file) {
			if verbose {
				fmt.Fprintf(os.Stderr, "[Verbose] PR %s rejected due to non-test/non-doc file: %s\n", prHTMLURL, file)
			}
			return false
		}
	}
	return true
}

// HasMMPRLink returns true if the body contains a Magic Modules PR link.
func HasMMPRLink(body string) bool {
	return derivedFromRegex.MatchString(body)
}

// ExtractMMPRs extracts all Magic Modules PR URLs from the body.
func ExtractMMPRs(body string) []string {
	matches := derivedFromRegex.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	var urls []string
	for _, match := range matches {
		if len(match) > 1 {
			urls = append(urls, match[1])
		}
	}
	return urls
}
