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

// IsTestFile checks if a file path is a test file or test fixture.
func IsTestFile(filename string) bool {
	if strings.HasSuffix(filename, "_test.go") {
		return true
	}
	if strings.Contains(filename, "/testdata/") || strings.HasPrefix(filename, "testdata/") {
		return true
	}
	return false
}

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
	return IsTestFile(filename)
}

// ClassifyPR checks the modified files in a PR and classifies the changes.
// Returns:
// - isDisqualified: true if the PR modifies any non-test/non-doc files (e.g. code changes).
// - isCandidate: true if the PR modifies only tests/docs AND modifies at least one test file.
func ClassifyPR(files []string, verbose bool, prHTMLURL string) (isDisqualified, isCandidate bool) {
	if len(files) == 0 {
		return false, false
	}

	hasCode := false
	hasTest := false

	for _, file := range files {
		if !IsTestOrDocFile(file) {
			if verbose {
				fmt.Fprintf(os.Stderr, "[Verbose] PR %s contains non-test/non-doc file: %s\n", prHTMLURL, file)
			}
			hasCode = true
		}
		if IsTestFile(file) {
			hasTest = true
		}
	}

	if hasCode {
		return true, false
	}
	if hasTest {
		return false, true
	}

	// Docs-only change: not disqualified, but not a candidate
	if verbose {
		fmt.Fprintf(os.Stderr, "[Verbose] PR %s only modifies documentation/changelog (no test changes).\n", prHTMLURL)
	}
	return false, false
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
