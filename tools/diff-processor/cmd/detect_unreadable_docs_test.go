package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/GoogleCloudPlatform/magic-modules/tools/diff-processor/detector"
	"github.com/GoogleCloudPlatform/magic-modules/tools/diff-processor/diff"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestDetectUnreadableDocs(t *testing.T) {
	repo := t.TempDir()
	docs := filepath.Join(repo, "website", "docs", "r")
	if err := os.MkdirAll(docs, 0755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"x.html.markdown": "# Title\n\n## Argument Reference\n\n* `documented` - (Optional) Documented.\n",
		"y.html.markdown": "# Title\n\n## Argument Reference\n\n* `documented` - (Optional) Documented.\n\n* `fixed` - (Optional) Fixed.\n",
	} {
		if err := os.WriteFile(filepath.Join(docs, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	str := &schema.Schema{Type: schema.TypeString, Optional: true}
	resources := map[string]*schema.Resource{
		"google_x": {Schema: map[string]*schema.Schema{"documented": str, "undocumented": str}},
		"google_y": {Schema: map[string]*schema.Schema{"documented": str, "fixed": str}},
	}
	cases := []struct {
		name string
		args []string
		all  bool
		want detector.UnreadableDocsReport
	}{
		{
			name: "given resources",
			args: []string{repo, "google_x"},
			want: detector.UnreadableDocsReport{
				NewUnreadableDocs:          map[string][]string{"google_x": {"undocumented"}},
				StaleUnreadableDocs:        map[string][]string{},
				NewMissingDataSourceDocs:   []string{},
				StaleMissingDataSourceDocs: []string{},
			},
		},
		{
			name: "no resources",
			args: []string{repo},
			want: detector.UnreadableDocsReport{
				NewUnreadableDocs:          map[string][]string{},
				StaleUnreadableDocs:        map[string][]string{},
				NewMissingDataSourceDocs:   []string{},
				StaleMissingDataSourceDocs: []string{},
			},
		},
		{
			name: "all resources",
			args: []string{repo},
			all:  true,
			want: detector.UnreadableDocsReport{
				NewUnreadableDocs:          map[string][]string{"google_x": {"undocumented"}},
				StaleUnreadableDocs:        map[string][]string{"google_y": {"fixed"}},
				NewMissingDataSourceDocs:   []string{},
				StaleMissingDataSourceDocs: []string{},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			o := detectUnreadableDocsOptions{
				computeSchemaDiff:           func() diff.SchemaDiff { return diff.SchemaDiff{} },
				computeDatasourceSchemaDiff: func() diff.SchemaDiff { return diff.SchemaDiff{} },
				resources:                   func() map[string]*schema.Resource { return resources },
				dataSources:                 func() map[string]*schema.Resource { return map[string]*schema.Resource{} },
				knownDocs:                   map[string][]string{"google_y": {"fixed"}},
				knownDataSources:            []string{},
				stdout:                      &buf,
				all:                         tc.all,
			}
			if err := o.run(tc.args); err != nil {
				t.Fatalf("run(%v) = %v", tc.args, err)
			}
			var got detector.UnreadableDocsReport
			if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
				t.Fatalf("Failed to unmarshal output: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("run(%v) returned diff (-want, +got):\n%s", tc.args, diff)
			}
		})
	}
}
