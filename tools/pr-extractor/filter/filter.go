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

// ExtractMMPR extracts the single Magic Modules PR URL from the body.
// Returns an error if multiple Magic Modules PR URLs are found.
func ExtractMMPR(body string) (string, error) {
	matches := derivedFromRegex.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return "", nil
	}
	if len(matches) > 1 {
		var found []string
		for _, m := range matches {
			if len(m) > 1 {
				found = append(found, m[1])
			}
		}
		return "", fmt.Errorf("found multiple Magic Modules PR links in PR description: %s", strings.Join(found, ", "))
	}
	if len(matches[0]) > 1 {
		return matches[0][1], nil
	}
	return "", nil
}
