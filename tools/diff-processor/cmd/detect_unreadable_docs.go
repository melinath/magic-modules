package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/GoogleCloudPlatform/magic-modules/tools/diff-processor/detector"
	"github.com/GoogleCloudPlatform/magic-modules/tools/diff-processor/diff"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/spf13/cobra"

	newProvider "google/provider/new/google/provider"
	oldProvider "google/provider/old/google/provider"
)

const detectUnreadableDocsDesc = "Report fields of the given resources and data sources that the missing doc detector can't find in their docs and that aren't known to be unreadable, and known unreadable fields that it can now find"

type detectUnreadableDocsOptions struct {
	rootOptions                 *rootOptions
	computeSchemaDiff           func() diff.SchemaDiff
	computeDatasourceSchemaDiff func() diff.SchemaDiff
	resources                   func() map[string]*schema.Resource
	dataSources                 func() map[string]*schema.Resource
	knownDocs                   map[string][]string
	knownDataSources            []string
	stdout                      io.Writer
	// Whether to check all resources and data sources rather than the given ones.
	all bool
}

func newDetectUnreadableDocsCmd(rootOptions *rootOptions) *cobra.Command {
	o := &detectUnreadableDocsOptions{
		rootOptions: rootOptions,
		computeSchemaDiff: func() diff.SchemaDiff {
			return schemaDiff
		},
		computeDatasourceSchemaDiff: func() diff.SchemaDiff {
			return diff.ComputeSchemaDiff(oldProvider.DatasourceMap(), newProvider.DatasourceMap())
		},
		resources:        newProvider.ResourceMap,
		dataSources:      newProvider.DatasourceMap,
		knownDocs:        detector.KnownUnreadableDocs,
		knownDataSources: detector.KnownMissingDataSourceDocs,
		stdout:           os.Stdout,
	}
	cmd := &cobra.Command{
		Use:   "detect-unreadable-docs REPO_PATH [RESOURCE...]",
		Short: detectUnreadableDocsDesc,
		Long:  detectUnreadableDocsDesc,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return o.run(args)
		},
	}
	cmd.Flags().BoolVar(&o.all, "all", false, "check all resources and data sources")
	return cmd
}

func (o *detectUnreadableDocsOptions) run(args []string) error {
	repoPath, names := args[0], args[1:]
	var inScope func(string) bool
	if !o.all {
		scope := make(map[string]bool)
		for _, name := range names {
			scope[name] = true
		}
		inScope = func(name string) bool { return scope[name] }
	}
	report, err := detector.DetectUnreadableDocs(o.resources(), o.dataSources(), inScope, o.computeSchemaDiff(), o.computeDatasourceSchemaDiff(), repoPath, o.knownDocs, o.knownDataSources)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(o.stdout).Encode(report); err != nil {
		return fmt.Errorf("error encoding json: %w", err)
	}
	return nil
}
