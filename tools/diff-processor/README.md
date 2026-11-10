# Diff processor

Provides tools that operate on diffs between provider versions.

## Run

```bash
# set up old / new dirs
make clone OWNER_REPO=modular-magician/terraform-provider-google

# build based on old / new dirs
make build OLD_REF=branch_or_commit NEW_REF=branch_or_commit

# Run breaking change detection on the difference between OLD_REF and NEW_REF
bin/diff-processor breaking-changes

# Compute service labels to add bsaed on the resources changed between OLD_REF and NEW_REF
bin/diff-processor changed-schema-labels

# Report acceptance tests in the given services that the missing test detector
# can't read and that aren't in detector.KnownUnreadableTests, and tests in
# that list that it can now read
bin/diff-processor detect-unreadable-tests path/to/google-beta/services compute container

# Report fields of the given resources and data sources that the missing doc
# detector can't find in their docs and that aren't in
# detector.KnownUnreadableDocs, and fields in that list that it can now find.
# Pass --all instead of resource names to check every resource and data source.
bin/diff-processor detect-unreadable-docs path/to/terraform-provider-google-beta google_compute_instance
```

## Test
```bash
go test ./...
```
