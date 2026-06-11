package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Client handles requests to the GitHub API.
type Client struct {
	BaseURL    string
	token      string
	httpClient *http.Client
	verbose    bool
	delay      time.Duration
}

func NewClient(token string, verbose bool, delayMs int) *Client {
	return &Client{
		BaseURL: "https://api.github.com",
		token:   token,
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
	Number  int    `json:"number"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
}

// PullRequestFile matches the structure of GitHub Pull Requests Files API response.
type PullRequestFile struct {
	Filename string `json:"filename"`
}

func (c *Client) doRequest(req *http.Request) ([]byte, error) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-API-Version", "2022-11-28")
	req.Header.Set("User-Agent", "magic-modules-pr-extractor")

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	// Wait to respect rate limits
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
func (c *Client) SearchMergedPRs(repo, since string) ([]PR, error) {
	var prs []PR
	page := 1

	for {
		query := fmt.Sprintf("repo:%s is:pr is:merged merged:>=%s", repo, since)
		escapedQuery := url.QueryEscape(query)
		apiURL := fmt.Sprintf("%s/search/issues?q=%s&per_page=100&page=%d", c.BaseURL, escapedQuery, page)

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
func (c *Client) GetPRFiles(repo string, prNumber int) ([]string, error) {
	var files []string
	page := 1

	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository format: %s", repo)
	}
	owner, repoName := parts[0], parts[1]

	for {
		apiURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/files?per_page=100&page=%d", c.BaseURL, owner, repoName, prNumber, page)

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
