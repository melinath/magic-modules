# PR Extractor

`pr-extractor` is a Go command-line tool that scans merged PRs from the Google Cloud Platform Terraform providers (`terraform-provider-google` and `terraform-provider-google-beta`), identifies PRs that **only** contain documentation or test changes, and extracts the unique upstream Magic Modules PR URLs from their descriptions.

This tool is useful for identifying test and documentation fixes that were synced downstream and tracing them back to their origin in Magic Modules.

## Features

- **Double Filtering**: Uses a quick, cheap description match to identify PRs that might be derived from Magic Modules before fetching full file lists, saving API requests.
- **Strict File Categorization**: Only matches PRs where all modified files match tests (`*_test.go`, `testdata/*`), documentation (`website/*`, `docs/*`, `.md`, `.markdown`), or changelog templates (`.changelog/*`).
- **No Dependencies**: Built entirely using Go's standard library (`net/http`, `encoding/json`, etc.) for portability, lightweight footprint, and simple builds.
- **Customizable**: Allows scanning custom repositories, setting custom start dates, and adding throttling delays.

## Installation / Build

Since this tool resides in a workspace containing other packages, build it with Go workspace mode disabled (`GOWORK=off`) to avoid module compatibility checks with the generator:

```bash
GOWORK=off go build -o pr-extractor main.go
```

## Running Unit Tests

You can run the full suite of unit tests for the parsing and API client code:

```bash
GOWORK=off go test -v ./...
```

## Usage

### Prerequisites
To run the tool for PRs merged since early 2026, the script will query pages of issues and PR files, consuming a moderate number of API requests. You will quickly hit unauthenticated rate limits. You should generate a GitHub Personal Access Token (PAT) and pass it via the environment variable `GITHUB_TOKEN`.

### Quick Start

To extract Magic Modules PRs merged since **January 1, 2026**:

```bash
export GITHUB_TOKEN="your-github-token-here"
GOWORK=off ./pr-extractor -since 2026-01-01T00:00:00Z
```

### Command-line Options

```
Usage of ./pr-extractor:
  -delay int
    	Delay in milliseconds between API requests to prevent rate limiting (default 100)
  -repos string
    	Comma-separated list of repos to scan (default "hashicorp/terraform-provider-google,hashicorp/terraform-provider-google-beta")
  -since string
    	Search for PRs merged since this ISO 8601 date (default "2026-01-01T00:00:00Z")
  -token string
    	GitHub Personal Access Token (defaults to GITHUB_TOKEN env var)
  -verbose
    	Print verbose debug output to stderr
```

## Example Output

```
Scanning repository: hashicorp/terraform-provider-google...
Found 150 merged PRs in hashicorp/terraform-provider-google since 2026-01-01T00:00:00Z.
Scanning repository: hashicorp/terraform-provider-google-beta...
Found 140 merged PRs in hashicorp/terraform-provider-google-beta since 2026-01-01T00:00:00Z.

Unique Magic Modules PR URLs:
https://github.com/GoogleCloudPlatform/magic-modules/pull/17893
https://github.com/GoogleCloudPlatform/magic-modules/pull/17889
https://github.com/GoogleCloudPlatform/magic-modules/pull/17934
```
