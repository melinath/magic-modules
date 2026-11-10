package detector

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoogleCloudPlatform/magic-modules/tools/diff-processor/diff"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestDetectUnreadableDocs(t *testing.T) {
	repo := t.TempDir()
	for path, content := range map[string]string{
		"r/a_resource.html.markdown":        "# Title\n\n## Argument Reference\n\n* `documented` - (Optional) Documented.\n\n* `fixed` - (Optional) Fixed.\n",
		"r/b_resource.html.markdown":        "# Title\n\n## Argument Reference\n\n* `documented` - (Optional) Documented.\n",
		"d/fixed_data_source.html.markdown": "# Title\n\n## Argument Reference\n",
		"d/a_data_source.html.markdown":     "# Title\n\n## Argument Reference\n",
	} {
		fullPath := filepath.Join(repo, "website", "docs", path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	str := &schema.Schema{Type: schema.TypeString, Optional: true}
	resources := map[string]*schema.Resource{
		"a_resource": {Schema: map[string]*schema.Schema{
			"documented": str,
			"fixed":      str,
			"known":      str,
			"unknown":    str,
			"added":      str,
			"condition":  str,
		}},
		"b_resource": {Schema: map[string]*schema.Schema{
			"documented": str,
			"unknown":    str,
		}},
	}
	dataSources := map[string]*schema.Resource{
		"a_data_source":       {},
		"fixed_data_source":   {},
		"known_data_source":   {},
		"unknown_data_source": {},
		"added_data_source":   {},
	}
	schemaDiff := diff.SchemaDiff{
		"a_resource": {Fields: map[string]diff.FieldDiff{"added": {New: str}}},
	}
	dataSourceSchemaDiff := diff.SchemaDiff{
		"added_data_source": {ResourceConfig: diff.ResourceConfigDiff{New: &schema.Resource{}}},
	}
	knownDocs := map[string][]string{
		"a_resource":       {"fixed", "known", "removed_field"},
		"removed_resource": {"field"},
	}
	knownDataSources := []string{"fixed_data_source", "known_data_source", "removed_data_source"}

	for _, tc := range []struct {
		name    string
		inScope func(string) bool
		want    UnreadableDocsReport
	}{
		{
			name: "all",
			want: UnreadableDocsReport{
				NewUnreadableDocs: map[string][]string{
					"a_resource": {"unknown"},
					"b_resource": {"unknown"},
				},
				StaleUnreadableDocs: map[string][]string{
					"a_resource":       {"fixed", "removed_field"},
					"removed_resource": {"field"},
				},
				NewMissingDataSourceDocs:   []string{"unknown_data_source"},
				StaleMissingDataSourceDocs: []string{"fixed_data_source", "removed_data_source"},
			},
		},
		{
			name:    "in scope",
			inScope: func(name string) bool { return name == "b_resource" },
			want: UnreadableDocsReport{
				NewUnreadableDocs: map[string][]string{
					"b_resource": {"unknown"},
				},
				StaleUnreadableDocs: map[string][]string{
					"removed_resource": {"field"},
				},
				NewMissingDataSourceDocs:   []string{},
				StaleMissingDataSourceDocs: []string{"removed_data_source"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DetectUnreadableDocs(resources, dataSources, tc.inScope, schemaDiff, dataSourceSchemaDiff, repo, knownDocs, knownDataSources)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("DetectUnreadableDocs() returned diff (-want, +got):\n%s", diff)
			}
		})
	}
}
