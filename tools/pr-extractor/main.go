package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/GoogleCloudPlatform/magic-modules/tools/pr-extractor/filter"
	"github.com/GoogleCloudPlatform/magic-modules/tools/pr-extractor/github"
)

// Config holds the configuration for the tool execution.
type Config struct {
	Token    string
	Since    string
	Verbose  bool
	DelayMs  int
	Repos    []string
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

	client := github.NewClient(config.Token, config.Verbose, config.DelayMs)

	// Map to collect and deduplicate Magic Modules PR URLs.
	// Maps to collect candidates and disqualified Magic Modules PR URLs.
	candidates := make(map[string]bool)
	disqualified := make(map[string]bool)

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
			// Pre-filter: does the PR body contain references to Magic Modules?
			if !filter.HasMMPRLink(pr.Body) {
				if config.Verbose {
					fmt.Fprintf(os.Stderr, "[Verbose] PR %s skipped (no Magic Modules PR link found in description).\n", pr.HTMLURL)
				}
				continue
			}

			mmURL, err := filter.ExtractMMPR(pr.Body)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error extracting Magic Modules link from PR %s: %v\n", pr.HTMLURL, err)
				continue
			}
			if mmURL == "" {
				continue
			}

			// Fetch modified files
			files, err := client.GetPRFiles(repo, pr.Number)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting files for PR %s: %v. Disqualifying %s.\n", pr.HTMLURL, err, mmURL)
				disqualified[mmURL] = true
				continue
			}

			isDisqualified, isCandidate := filter.ClassifyPR(files, config.Verbose, pr.HTMLURL)
			if isDisqualified {
				disqualified[mmURL] = true
			} else if isCandidate {
				candidates[mmURL] = true
			}
		}
	}

	// Output the valid Magic Modules PR URLs.
	fmt.Println("\nUnique Magic Modules PR URLs (Test/Doc changes only):")
	for mmPR := range candidates {
		if !disqualified[mmPR] {
			fmt.Println(mmPR)
		}
	}
}
