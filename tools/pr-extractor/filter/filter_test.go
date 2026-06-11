package filter

import (
	"testing"
)

func TestIsTestOrDocFile(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		// Valid cases: Docs
		{"website/docs/r/compute.html.markdown", true},
		{"docs/index.md", true},
		{"README.md", true},
		{"CONTRIBUTING.markdown", true},
		// Valid cases: Tests
		{"google/services/compute/resource_compute_instance_test.go", true},
		{"google/testdata/payload.json", true},
		{"testdata/config.tf", true},
		// Valid cases: Changelog
		{".changelog/12345.txt", true},
		{".changelog/fix_bug.txt", true},

		// Invalid cases
		{"google/services/compute/resource_compute_instance.go", false},
		{"go.mod", false},
		{"go.sum", false},
		{"GNUmakefile", false},
		{".github/workflows/main.yml", false},
		{"tools/diff-processor/main.go", false},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			actual := IsTestOrDocFile(tc.filename)
			if actual != tc.expected {
				t.Errorf("IsTestOrDocFile(%q) = %v; want %v", tc.filename, actual, tc.expected)
			}
		})
	}
}

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"google/services/compute/resource_compute_instance_test.go", true},
		{"google/testdata/payload.json", true},
		{"testdata/config.tf", true},
		{"google/services/compute/resource_compute_instance.go", false},
		{"website/docs/r/compute.html.markdown", false},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			actual := IsTestFile(tc.filename)
			if actual != tc.expected {
				t.Errorf("IsTestFile(%q) = %v; want %v", tc.filename, actual, tc.expected)
			}
		})
	}
}

func TestClassifyPR(t *testing.T) {
	tests := []struct {
		name                 string
		files                []string
		expectedDisqualified bool
		expectedCandidate    bool
	}{
		{
			name: "test and doc files (candidate)",
			files: []string{
				"website/docs/r/compute.html.markdown",
				"google/services/compute/resource_compute_instance_test.go",
				".changelog/12345.txt",
			},
			expectedDisqualified: false,
			expectedCandidate:    true,
		},
		{
			name: "contains a code file (disqualified)",
			files: []string{
				"website/docs/r/compute.html.markdown",
				"google/services/compute/resource_compute_instance.go",
				".changelog/12345.txt",
			},
			expectedDisqualified: true,
			expectedCandidate:    false,
		},
		{
			name: "docs only files (ignored, not disqualified)",
			files: []string{
				"website/docs/r/compute.html.markdown",
				".changelog/12345.txt",
			},
			expectedDisqualified: false,
			expectedCandidate:    false,
		},
		{
			name:                 "empty file list",
			files:                []string{},
			expectedDisqualified: false,
			expectedCandidate:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isDisqualified, isCandidate := ClassifyPR(tc.files, false, "https://github.com/pr/1")
			if isDisqualified != tc.expectedDisqualified {
				t.Errorf("ClassifyPR(%v) isDisqualified = %v; want %v", tc.files, isDisqualified, tc.expectedDisqualified)
			}
			if isCandidate != tc.expectedCandidate {
				t.Errorf("ClassifyPR(%v) isCandidate = %v; want %v", tc.files, isCandidate, tc.expectedCandidate)
			}
		})
	}
}

func TestExtractMMPR(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		expectedUrl string
		expectErr   bool
	}{
		{
			name:        "standard markdown link description",
			body:        "Derived from https://github.com/GoogleCloudPlatform/magic-modules/pull/17934",
			expectedUrl: "https://github.com/GoogleCloudPlatform/magic-modules/pull/17934",
			expectErr:   false,
		},
		{
			name:        "markdown syntax with brackets",
			body:        "[Derived from](https://github.com/GoogleCloudPlatform/magic-modules/pull/17934)",
			expectedUrl: "https://github.com/GoogleCloudPlatform/magic-modules/pull/17934",
			expectErr:   false,
		},
		{
			name:        "colon formatted",
			body:        "Derived from: https://github.com/GoogleCloudPlatform/magic-modules/pull/17934",
			expectedUrl: "https://github.com/GoogleCloudPlatform/magic-modules/pull/17934",
			expectErr:   false,
		},
		{
			name:        "colon and hyphen formatted",
			body:        "derived from - https://github.com/GoogleCloudPlatform/magic-modules/pull/17934",
			expectedUrl: "https://github.com/GoogleCloudPlatform/magic-modules/pull/17934",
			expectErr:   false,
		},
		{
			name:        "multiple links in body - returns error",
			body: `Some description.
Derived from https://github.com/GoogleCloudPlatform/magic-modules/pull/17934
Also derived from: https://github.com/GoogleCloudPlatform/magic-modules/pull/17935
End of description.`,
			expectedUrl: "",
			expectErr:   true,
		},
		{
			name:        "no matching links",
			body:        "This PR was written manually by the team.",
			expectedUrl: "",
			expectErr:   false,
		},
		{
			name:        "partial link matches",
			body:        "Derived from https://github.com/other-org/other-repo/pull/123",
			expectedUrl: "",
			expectErr:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actualUrl, err := ExtractMMPR(tc.body)
			if (err != nil) != tc.expectErr {
				t.Fatalf("ExtractMMPR(%q) returned error %v; expected error presence: %v", tc.body, err, tc.expectErr)
			}
			if actualUrl != tc.expectedUrl {
				t.Errorf("ExtractMMPR(%q) = %q; want %q", tc.body, actualUrl, tc.expectedUrl)
			}
		})
	}
}

func TestHasMMPRLink(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected bool
	}{
		{
			name:     "has link",
			body:     "Derived from https://github.com/GoogleCloudPlatform/magic-modules/pull/123",
			expected: true,
		},
		{
			name:     "does not have link",
			body:     "Simple description.",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := HasMMPRLink(tc.body)
			if actual != tc.expected {
				t.Errorf("HasMMPRLink(%q) = %v; want %v", tc.body, actual, tc.expected)
			}
		})
	}
}
