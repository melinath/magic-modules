package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"
	"time"

	"github.com/GoogleCloudPlatform/magic-modules/mmv1/api"
	"github.com/GoogleCloudPlatform/magic-modules/mmv1/google"
	"github.com/GoogleCloudPlatform/magic-modules/mmv1/provider"
)

type stringList []string

func (s *stringList) Set(val string) error {
	*s = append(*s, strings.Split(val, ",")...)
	return nil
}

func (s *stringList) String() string {
	return strings.Join(*s, ",")
}

var versionFlag = flag.String("version", "", "provider version to generate")
var productNameFlag = flag.String("product_name", "", "name of the product referenced by --product")
var productFlag = flag.String("product", "", "path to product.yaml input file")
var productOverrideFlag = flag.String("product_override", "", "path to a product override input file")
var resourceFlag = flag.String("resource", "", "path to resource.yaml input file")
var resourceOverrideFlag = flag.String("resource_override", "", "path to a resource override input file")
var outputPathFlag = flag.String("output", "", "output path for generated files")
var typeFlag = flag.String("type", "", "type of output to generate [product|resource|operation]")
var providerFlag = flag.String("provider", "", "target provider")

func main() {
	var target stringList
	flag.Var(&target, "target", "comma-separated list of input=output files to templatize or copy")

	flag.Parse()

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("could not find wd: %v", err)
	}
	fsys := os.DirFS(filepath.Join(wd, "mmv1"))

	if *versionFlag == "" {
		log.Fatal("--version is required")
	}
	if *providerFlag == "" {
		log.Fatal("--provider is required")
	}

	switch *typeFlag {
	case "":
		log.Fatal("--type is required")
	case "product":
		if *providerFlag == "" || *outputPathFlag == "" || *productNameFlag == "" || *productFlag == "" {
			log.Fatal("--product, --product_name, and --output are required with --type=product")
		}
	case "resource":
		if *resourceFlag == "" || *providerFlag == "" || *outputPathFlag == "" || *productNameFlag == "" || *productFlag == "" {
			log.Fatal("--resource, --product, --product_name, and --output are required with --type=resource")
		}
	case "metadata":
		if *resourceFlag == "" || *providerFlag == "" || *outputPathFlag == "" || *productNameFlag == "" || *productFlag == "" {
			log.Fatal("--resource, --product, --product_name, and --output are required with --type=metadata")
		}
	case "operation":
		if *resourceFlag == "" || *providerFlag == "" || *outputPathFlag == "" || *productNameFlag == "" || *productFlag == "" {
			log.Fatal("--resource, --product, --product_name, and --output are required with --type=operation")
		}
	case "sweeper":
		if *resourceFlag == "" || *providerFlag == "" || *outputPathFlag == "" || *productNameFlag == "" || *productFlag == "" {
			log.Fatal("--resource, --product, --product_name, and --output are required with --type=sweeper")
		}
	case "copy":
		files, err := parseTarget(target)
		if err != nil {
			log.Fatalf("parsing --target: %v", err)
		}
		if err := copyFiles(files); err != nil {
			log.Fatalf("copying files: %v", err)
		}
		return
	case "template":
		files, err := parseTarget(target)
		if err != nil {
			log.Fatalf("parsing --target: %v", err)
		}
		if err := templateFiles(fsys, files); err != nil {
			log.Fatalf("templating files: %v", err)
		}
		return
	default:
		log.Fatalf("unrecognized --type %q", *typeFlag)
	}

	var product api.Product
	api.Compile(*productFlag, &product)
	if *productOverrideFlag != "" {
		var override api.Product
		api.Compile(*productOverrideFlag, &override)
		api.Merge(reflect.ValueOf(product), reflect.ValueOf(override), *versionFlag)
	}
	if !product.ExistsAtVersionOrLower(*versionFlag) {
		log.Fatalf("product %q does not support version %q", *productNameFlag, *versionFlag)
	}
	product.Version = product.VersionObjOrClosest(*versionFlag)

	if *resourceFlag != "" {
		var resource api.Resource
		api.Compile(*resourceFlag, &resource)
		if *resourceOverrideFlag != "" {
			var override api.Resource
			api.Compile(*resourceOverrideFlag, &override)
			api.Merge(reflect.ValueOf(resource), reflect.ValueOf(override), *versionFlag)
		}
		resource.TargetVersionName = *versionFlag
		resource.SetDefault(&product)
		resource.Properties = resource.AddExtraFields(resource.PropertiesWithExcluded(), nil)
		resource.SetDefault(&product)
		product.Objects = []*api.Resource{&resource}
	}

	product.Validate()

	switch *providerFlag {
	case "tgc", "tgc_cai2hcl", "tgc_next", "oics":
		log.Fatalf("--provider %q is not yet supported", *providerFlag)
	case "tpg":
	default:
		log.Fatalf("unrecognized --provider %q", *providerFlag)
	}

	generator := provider.NewTerraform(&product, *versionFlag, time.Now(), fsys)

	switch *typeFlag {
	case "product":
		generator.GenerateProductFile(*outputPathFlag)
	case "resource":
		generator.GenerateResourceFile(*product.Objects[0], *outputPathFlag)
	case "metadata":
		generator.GenerateResourceMetadataFile(*product.Objects[0], *outputPathFlag)
	case "operation":
		generator.GenerateOperationFile(*product.Objects[0], *outputPathFlag)
	case "sweeper":
		generator.GenerateResourceSweeperFile(*product.Objects[0], *outputPathFlag)
	}
}

func parseTarget(target stringList) (map[string]string, error) {
	files := make(map[string]string)
	for _, pair := range target {
		split := strings.Split(pair, "=")
		if len(split) != 2 || split[0] == "" || split[1] == "" {
			return nil, fmt.Errorf("invalid --target %q", pair)
		}
		if _, ok := files[split[0]]; ok {
			return nil, fmt.Errorf("duplicate target input %q", pair)
		}
		files[split[0]] = split[1]
	}
	return files, nil
}

func copyFiles(files map[string]string) error {
	for in, out := range files {
		contents, err := os.ReadFile(in)
		if err != nil {
			return fmt.Errorf("reading %q: %w", in, err)
		}
		st, err := os.Stat(in)
		if err != nil {
			return fmt.Errorf("statting %q: %w", in, err)
		}
		// TODO: insert copyrights, fix import paths, add AUTOGENERATED CODE, and format .go files
		dir := filepath.Dir(out)
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return fmt.Errorf("creating directory %q: %w", dir, err)
		}
		if err := os.WriteFile(out, contents, st.Mode()); err != nil {
			return fmt.Errorf("writing %q: %w", out, err)
		}
	}
	return nil
}

func templateFiles(fsys fs.FS, files map[string]string) error {
	for in, out := range files {
		input, err := os.ReadFile(in)
		if err != nil {
			return fmt.Errorf("reading %q: %w", in, err)
		}
		templateFileName := filepath.Base(out)

		funcMap := template.FuncMap{
			"TemplatePath": func() string { return in },
		}
		for k, v := range google.TemplateFunctions(fsys) {
			funcMap[k] = v
		}

		tmpl, err := template.New(templateFileName).Funcs(funcMap).Parse(string(input))
		if err != nil {
			return fmt.Errorf("parsing template %q: %w", in, err)
		}

		data := provider.ProviderWithProducts{
			Terraform: provider.Terraform{
				TargetVersionName: *versionFlag,
			},
		}

		var output bytes.Buffer
		if err = tmpl.ExecuteTemplate(&output, templateFileName, &data); err != nil {
			return fmt.Errorf("executing template %q: %w", in, err)
		}
		// TODO: insert copyrights, fix import paths, add AUTOGENERATED CODE, and format .go files
		if err := os.WriteFile(out, output.Bytes(), 0644); err != nil {
			return fmt.Errorf("writing %q: %w", out, err)
		}
	}
	return nil
}
