package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// Config holds the configuration for the tool execution.
type Config struct {
	Token    string
	Since    string
	Verbose  bool
	DelayMs  int
	Repos    []string
}

// GitHubClient handles requests to the GitHub API.
type GitHubClient struct {
	token      string
	httpClient *http.Client
	verbose    bool
	delay      time.Duration
}

func NewGitHubClient(token string, verbose bool, delayMs int) *GitHubClient {
	return &GitHubClient{
		token: token,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		verbose: verbose,
		delay:   time.Duration(delayMs) * time.Millisecond,
	}
}

// SearchIssuesResponse matches the structure of GitHub Search Issues API response.
type SearchIssuesResponse struct {
	TotalCount        int  `json:"total_count"`
	IncompleteResults bool `json:"incomplete_results"`
	Items             []PR `json:"items"`
}

type PR struct {
	Number   int    `json:"number"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	HTMLURL  string `json:"html_url"`
}

// PullRequestFile matches the structure of GitHub Pull Requests Files API response.
type PullRequestFile struct {
	Filename string `json:"filename"`
}

var (
	// Regex to match the Magic Modules PR URL from the body.
	// Matches variations like:
	// - Derived from https://github.com/GoogleCloudPlatform/magic-modules/pull/1234
	// - [Derived from](https://github.com/GoogleCloudPlatform/magic-modules/pull/1234)
	// - Derived from: https://github.com/GoogleCloudPlatform/magic-modules/pull/1234
	derivedFromRegex = regexp.MustCompile(`(?i)derived\s+from[^a-zA-Z0-9]*(https://github\.com/GoogleCloudPlatform/magic-modules/pull/\d+)`)
)

func (c *GitHubClient) doRequest(req *http.Request) ([]byte, error) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-API-Version", "2022-11-28")
	req.Header.Set("User-Agent", "magic-modules-pr-extractor")

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	// Wait to respect secondary rate limits
	if c.delay > 0 {
		time.Sleep(c.delay)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		// Check for rate limiting
		if resp.StatusCode == http.StatusForbidden && strings.Contains(string(body), "rate limit") {
			resetHeader := resp.Header.Get("X-RateLimit-Reset")
			if resetHeader != "" {
				return nil, fmt.Errorf("GitHub API rate limit exceeded. Reset time: %s", resetHeader)
			}
			return nil, fmt.Errorf("GitHub API rate limit exceeded: %s", string(body))
		}
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// SearchMergedPRs searches for merged PRs in a repository since a specific date.
func (c *GitHubClient) SearchMergedPRs(repo, since string) ([]PR, error) {
	var prs []PR
	page := 1

	for {
		// Construct the search query
		query := fmt.Sprintf("repo:%s is:pr is:merged merged:>=%s", repo, since)
		escapedQuery := url.QueryEscape(query)
		apiURL := fmt.Sprintf("https://api.github.com/search/issues?q=%s&per_page=100&page=%d", escapedQuery, page)

		if c.verbose {
			fmt.Fprintf(os.Stderr, "Searching PRs in %s (page %d)... URL: %s\n", repo, page, apiURL)
		}

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return nil, err
		}

		respBody, err := c.doRequest(req)
		if err != nil {
			return nil, err
		}

		var searchResp SearchIssuesResponse
		if err := json.Unmarshal(respBody, &searchResp); err != nil {
			return nil, err
		}

		if page == 1 && searchResp.TotalCount > 1000 {
			fmt.Fprintf(os.Stderr, "WARNING: Total merged PRs for %s since %s is %d, but GitHub Search API caps results at 1000. Some PRs may be omitted.\n", repo, since, searchResp.TotalCount)
		}

		if len(searchResp.Items) == 0 {
			break
		}

		prs = append(prs, searchResp.Items...)
		if len(searchResp.Items) < 100 {
			break
		}
		page++
	}

	return prs, nil
}

// GetPRFiles fetches all files modified by a pull request.
func (c *GitHubClient) GetPRFiles(repo string, prNumber int) ([]string, error) {
	var files []string
	page := 1

	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository format: %s", repo)
	}
	owner, repoName := parts[0], parts[1]

	for {
		apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/files?per_page=100&page=%d", owner, repoName, prNumber, page)

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return nil, err
		}

		respBody, err := c.doRequest(req)
		if err != nil {
			return nil, err
		}

		var prFiles []PullRequestFile
		if err := json.Unmarshal(respBody, &prFiles); err != nil {
			return nil, err
		}

		if len(prFiles) == 0 {
			break
		}

		for _, f := range prFiles {
			files = append(files, f.Filename)
		}

		if len(prFiles) < 100 {
			break
		}
		page++
	}

	return files, nil
}

// isTestOrDocFile checks if a file path qualifies as tests or documentation.
func isTestOrDocFile(filename string) bool {
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

// onlyModifiesTestsOrDocs checks if all modified files in a PR are test or doc files.
func onlyModifiesTestsOrDocs(files []string, verbose bool, prHTMLURL string) bool {
	if len(files) == 0 {
		return false
	}
	for _, file := range files {
		if !isTestOrDocFile(file) {
			if verbose {
				fmt.Fprintf(os.Stderr, "[Verbose] PR %s rejected due to non-test/non-doc file: %s\n", prHTMLURL, file)
			}
			return false
		}
	}
	return true
}

func main() {
	var config Config
	var reposFlag string

	flag.StringVar(&config.Token, "token", "", "GitHub Personal Access Token (defaults to GITHUB_TOKEN env var)")
	flag.StringVar(&config.Since, "since", "2026-01-01T00:00:00Z", "Search for PRs merged since this ISO 8601 date")
	flag.StringVar(&reposFlag, "repos", "hashicorp/terraform-provider-google,hashicorp/terraform-provider-google-beta", "Comma-separated list of repos to scan")
	flag.BoolVar(&config.Verbose, "verbose", false, "Print verbose debug output to stderr")
	flag.IntVar(&config.DelayMs, "delay", 100, "Delay in milliseconds between API requests to prevent rate limiting")

	flag.Parse()

	// Fallback to environment variable for token
	if config.Token == "" {
		config.Token = os.Getenv("GITHUB_TOKEN")
		if config.Token == "" {
			config.Token = os.Getenv("GH_TOKEN")
		}
	}

	if config.Token == "" {
		fmt.Fprintln(os.Stderr, "WARNING: No GitHub token provided. You may hit rate limits quickly.")
		fmt.Fprintln(os.Stderr, "Use the -token flag or set the GITHUB_TOKEN environment variable.")
	}

	config.Repos = strings.Split(reposFlag, ",")
	for i, r := range config.Repos {
		config.Repos[i] = strings.TrimSpace(r)
	}

	client := NewGitHubClient(config.Token, config.Verbose, config.DelayMs)

	// Map to collect and deduplicate Magic Modules PR URLs.
	uniqueMMPRs := make(map[string]bool)

	for _, repo := range config.Repos {
		if repo == "" {
			continue
		}

		fmt.Fprintf(os.Stderr, "Scanning repository: %s...\n", repo)

		prs, err := client.SearchMergedPRs(repo, config.Since)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error searching PRs for %s: %v\n", repo, err)
			os.Exit(1)
		}

		fmt.Fprintf(os.Stderr, "Found %d merged PRs in %s since %s.\n", len(prs), repo, config.Since)

		for _, pr := range prs {
			// First, perform a cheap check: does the PR body contain references to Magic Modules PRs?
			matches := derivedFromRegex.FindAllStringSubmatch(pr.Body, -1)
			if len(matches) == 0 {
				if config.Verbose {
					fmt.Fprintf(os.Stderr, "[Verbose] PR %s skipped (no Magic Modules PR link found in description).\n", pr.HTMLURL)
				}
				continue
			}

			// If the PR body has references, fetch the modified files list to verify if it's test/doc-only.
			files, err := client.GetPRFiles(repo, pr.Number)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting files for PR %s: %v\n", pr.HTMLURL, err)
				continue
			}

			if !onlyModifiesTestsOrDocs(files, config.Verbose, pr.HTMLURL) {
				continue
			}

			// All files are test/doc, and we have derived matches. Extract them.
			if config.Verbose {
				fmt.Fprintf(os.Stderr, "[Verbose] PR %s matches test/doc-only filter. Extracting links.\n", pr.HTMLURL)
			}

			for _, match := range matches {
				if len(match) > 1 {
					mmURL := match[1]
					uniqueMMPRs[mmURL] = true
				}
			}
		}
	}

	// Output the deduplicated Magic Modules PR URLs.
	fmt.Println("\nUnique Magic Modules PR URLs:")
	for mmPR := range uniqueMMPRs {
		fmt.Println(mmPR)
	}
}
