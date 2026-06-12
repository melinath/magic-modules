package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestSearchMergedPRs(t *testing.T) {
	mockResponse := SearchIssuesResponse{
		TotalCount:        2,
		IncompleteResults: false,
		Items: []PR{
			{Number: 1, Title: "PR 1", Body: "Body 1", HTMLURL: "url1"},
			{Number: 2, Title: "PR 2", Body: "Body 2", HTMLURL: "url2"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request details
		if r.URL.Path != "/search/issues" {
			t.Errorf("expected path '/search/issues', got '%s'", r.URL.Path)
		}

		query := r.URL.Query().Get("q")
		if query == "" {
			t.Error("expected 'q' query parameter to be set")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := NewClient("", false, 0)
	client.BaseURL = server.URL

	prs, err := client.SearchMergedPRs("owner/repo", "2026-01-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(prs) != 2 {
		t.Errorf("expected 2 PRs, got %d", len(prs))
	}

	if !reflect.DeepEqual(prs, mockResponse.Items) {
		t.Errorf("expected %v, got %v", mockResponse.Items, prs)
	}
}

func TestGetPRFiles(t *testing.T) {
	mockFilesPage1 := []PullRequestFile{
		{Filename: "file1.go"},
		{Filename: "file2_test.go"},
	}
	mockFilesPage2 := []PullRequestFile{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/repos/owner/repo/pulls/123/files"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path '%s', got '%s'", expectedPath, r.URL.Path)
		}

		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if page == "1" {
			json.NewEncoder(w).Encode(mockFilesPage1)
		} else {
			json.NewEncoder(w).Encode(mockFilesPage2)
		}
	}))
	defer server.Close()

	client := NewClient("", false, 0)
	client.BaseURL = server.URL

	files, err := client.GetPRFiles("owner/repo", 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedFiles := []string{"file1.go", "file2_test.go"}
	if !reflect.DeepEqual(files, expectedFiles) {
		t.Errorf("expected files %v, got %v", expectedFiles, files)
	}
}

func TestRateLimitExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message": "API rate limit exceeded for user"}`))
	}))
	defer server.Close()

	client := NewClient("", false, 0)
	client.BaseURL = server.URL

	_, err := client.SearchMergedPRs("owner/repo", "2026-01-01")
	if err == nil {
		t.Fatal("expected error due to rate limit, got nil")
	}

	expectedErrorStr := `GitHub API returned status 403: {"message": "API rate limit exceeded for user"}`
	if err.Error() != expectedErrorStr {
		t.Errorf("expected error %q, got %q", expectedErrorStr, err.Error())
	}
}

func TestRateLimitRetry(t *testing.T) {
	callCount := 0
	var sleptFor time.Duration

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First call: return 403 rate limit with reset time 10 seconds in the future
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(10*time.Second).Unix()))
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"message": "rate limit exceeded"}`))
		} else {
			// Second call: succeed
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(SearchIssuesResponse{TotalCount: 5})
		}
	}))
	defer server.Close()

	client := NewClient("", false, 0)
	client.BaseURL = server.URL
	client.sleepFunc = func(d time.Duration) {
		sleptFor = d
	}

	count, err := client.getSearchTotalCount("test-query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 5 {
		t.Errorf("expected count 5, got %d", count)
	}

	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}

	// Verify sleptFor is roughly 12 seconds (10s diff + 2s padding)
	if sleptFor < 10*time.Second || sleptFor > 13*time.Second {
		t.Errorf("expected sleep duration around 12s, got %v", sleptFor)
	}
}

func TestSearchMergedPRsSplitting(t *testing.T) {
	callIndex := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		query := r.URL.Query()
		perPage := query.Get("per_page")
		page := query.Get("page")

		callIndex++

		switch callIndex {
		case 1: // Count check for full range
			if perPage != "1" || page != "1" {
				t.Errorf("call 1: expected per_page=1, page=1, got per_page=%s, page=%s", perPage, page)
			}
			json.NewEncoder(w).Encode(SearchIssuesResponse{TotalCount: 1200})
		case 2: // Count check for left sub-range
			if perPage != "1" || page != "1" {
				t.Errorf("call 2: expected per_page=1, page=1, got per_page=%s, page=%s", perPage, page)
			}
			json.NewEncoder(w).Encode(SearchIssuesResponse{TotalCount: 600})
		case 3: // Fetch for left range (page 1)
			if perPage != "100" || page != "1" {
				t.Errorf("call 3: expected per_page=100, page=1, got per_page=%s, page=%s", perPage, page)
			}
			json.NewEncoder(w).Encode(SearchIssuesResponse{
				Items: []PR{{Number: 10, Title: "Left PR", HTMLURL: "left-url"}},
			})
		case 4: // Count check for right sub-range
			if perPage != "1" || page != "1" {
				t.Errorf("call 4: expected per_page=1, page=1, got per_page=%s, page=%s", perPage, page)
			}
			json.NewEncoder(w).Encode(SearchIssuesResponse{TotalCount: 600})
		case 5: // Fetch for right range (page 1)
			if perPage != "100" || page != "1" {
				t.Errorf("call 5: expected per_page=100, page=1, got per_page=%s, page=%s", perPage, page)
			}
			json.NewEncoder(w).Encode(SearchIssuesResponse{
				Items: []PR{{Number: 20, Title: "Right PR", HTMLURL: "right-url"}},
			})
		default:
			t.Errorf("unexpected API call #%d: %s", callIndex, r.URL.String())
		}
	}))
	defer server.Close()

	client := NewClient("", false, 0)
	client.BaseURL = server.URL

	prs, err := client.SearchMergedPRs("owner/repo", "2026-01-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []PR{
		{Number: 10, Title: "Left PR", HTMLURL: "left-url"},
		{Number: 20, Title: "Right PR", HTMLURL: "right-url"},
	}
	if !reflect.DeepEqual(prs, expected) {
		t.Errorf("expected %v, got %v", expected, prs)
	}

	if callIndex != 5 {
		t.Errorf("expected exactly 5 API calls, got %d", callIndex)
	}
}

