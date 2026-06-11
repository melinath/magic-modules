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
	startTime, err := time.Parse(time.RFC3339, since)
	if err != nil {
		startTime, err = time.Parse("2006-01-02", since)
		if err != nil {
			return nil, fmt.Errorf("invalid start date %q: must be in YYYY-MM-DD or RFC3339 format", since)
		}
	}
	endTime := time.Now()
	return c.searchMergedPRsRange(repo, startTime, endTime)
}

func (c *Client) searchMergedPRsRange(repo string, start, end time.Time) ([]PR, error) {
	startStr := start.Format(time.RFC3339)
	endStr := end.Format(time.RFC3339)
	query := fmt.Sprintf("repo:%s is:pr is:merged merged:%s..%s", repo, startStr, endStr)

	totalCount, err := c.getSearchTotalCount(query)
	if err != nil {
		return nil, err
	}

	if totalCount == 0 {
		return nil, nil
	}

	if totalCount <= 1000 {
		return c.retrievePRsForQuery(query)
	}

	diff := end.Sub(start)
	if diff <= 2*time.Second {
		fmt.Fprintf(os.Stderr, "WARNING: More than 1000 PRs merged within 2 seconds (%s to %s). Retrieving first 1000 only.\n", startStr, endStr)
		return c.retrievePRsForQuery(query)
	}

	mid := start.Add(diff / 2).Truncate(time.Second)

	if c.verbose {
		fmt.Fprintf(os.Stderr, "[Verbose] Range %s..%s has %d results (>1000 limit). Splitting at %s\n",
			startStr, endStr, totalCount, mid.Format(time.RFC3339))
	}

	leftPRs, err := c.searchMergedPRsRange(repo, start, mid)
	if err != nil {
		return nil, err
	}

	rightPRs, err := c.searchMergedPRsRange(repo, mid.Add(time.Second), end)
	if err != nil {
		return nil, err
	}

	return append(leftPRs, rightPRs...), nil
}

func (c *Client) getSearchTotalCount(query string) (int, error) {
	escapedQuery := url.QueryEscape(query)
	apiURL := fmt.Sprintf("%s/search/issues?q=%s&per_page=1&page=1", c.BaseURL, escapedQuery)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return 0, err
	}

	respBody, err := c.doRequest(req)
	if err != nil {
		return 0, err
	}

	var searchResp SearchIssuesResponse
	if err := json.Unmarshal(respBody, &searchResp); err != nil {
		return 0, err
	}

	return searchResp.TotalCount, nil
}

func (c *Client) retrievePRsForQuery(query string) ([]PR, error) {
	var prs []PR
	page := 1

	for {
		escapedQuery := url.QueryEscape(query)
		apiURL := fmt.Sprintf("%s/search/issues?q=%s&per_page=100&page=%d", c.BaseURL, escapedQuery, page)

		if c.verbose {
			fmt.Fprintf(os.Stderr, "Fetching page %d for query: %s\n", page, query)
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
