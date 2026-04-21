"""
Product-related custom build rules for Magic Modules.
"""

load("//build_defs:providers.bzl", "ProductInfo", "ResourceInfo", "TpgResourceInfo")

def _mm_product_impl(ctx):
    return [ProductInfo(
        name = ctx.label.name,
        yaml = ctx.file.src,
        version = ctx.attr.version,
    )]

mm_product = rule(
    implementation = _mm_product_impl,
    attrs = {
        "version": attr.string(default = "ga", mandatory = False),
        "src": attr.label(
            allow_single_file = [".yaml"],
            mandatory = True,
        ),
    },
)

def _tpg_product_impl(ctx):
    product = ctx.attr.product[ProductInfo]
    resources = [res[ResourceInfo] for res in ctx.attr.resources]
    tpg_resources = [res[TpgResourceInfo] for res in ctx.attr.resources]
    operations = [res for res in resources if res.has_operation]
    inputs = [product.yaml] + [res.metadata for res in tpg_resources] + [res.src for res in tpg_resources] + [res.yaml for res in resources] + [f for f in ctx.files._templates]

    outputs = [
        ctx.actions.declare_file("{}/product.go".format(product.version)),
    ]
    ctx.actions.run(
        executable = ctx.executable._compiler,
        arguments = [
            "--product",
            product.yaml.path,
            "--version",
            product.version,
            "--product_name",
            product.name,
            "--type",
            "product",
            "--provider",
            "tpg",
            "--output",
            outputs[0].path,
        ],
        inputs = depset([i for i in inputs]),
        outputs = outputs,
        mnemonic = "TpgGenerateProduct",
    )
    if operations:
        operation_go = ctx.actions.declare_file("{}/{}_operation.go".format(product.version, product.name))
        ctx.actions.run(
            executable = ctx.executable._compiler,
            arguments = [
                "--product",
                product.yaml.path,
                "--resource",
                operations[0].yaml.path,
                "--version",
                product.version,
                "--product_name",
                product.name,
                "--type",
                "operation",
                "--provider",
                "tpg",
                "--output",
                operation_go.path,
            ],
            inputs = depset([i for i in inputs]),
            outputs = [operation_go],
            mnemonic = "TpgGenerateProductOperation",
        )
        outputs.append(operation_go)

    return [
        ctx.attr.product[ProductInfo],
        DefaultInfo(files = depset([out for out in outputs])),
    ]

tpg_product = rule(
    implementation = _tpg_product_impl,
    attrs = {
        "product": attr.label(
            providers = [ProductInfo],
            mandatory = True,
        ),
        "resources": attr.label_list(
            providers = [ResourceInfo, TpgResourceInfo],
            mandatory = True,
        ),
        "_compiler": attr.label(
            default = Label("//mmv1/cmd"),
            allow_single_file = True,
            executable = True,
            cfg = "exec",
        ),
        "_templates": attr.label(
            default = Label("//mmv1/templates"),
        ),
    },
)

def _mm_template_library_impl(ctx):
    copy_inputs = [f for f in ctx.files.srcs]
    copy_outputs = []
    copy_targets = []
    for f in copy_inputs:
        filename = f.path.split("/")[-1]
        output = ctx.actions.declare_file("{}/{}".format(ctx.attr.version, filename))
        copy_targets.append("{}={}".format(f.path, output.path))
        copy_outputs.append(output)

    template_inputs = [f for f in ctx.files.template_srcs]
    template_outputs = []
    template_targets = []
    for f in template_inputs:
        filename = f.path.split("/")[-1]
        basename = ".".join(filename.split(".")[0:-1])
        suffix = filename.split(".")[-1]
        if suffix != "tmpl":
          fail(args=["template_srcs entry '{}' must end with a .tmpl extension".format(filename)])
        output = ctx.actions.declare_file("{}/{}".format(ctx.attr.version, basename))
        template_targets.append("{}={}".format(f.path, output.path))
        template_outputs.append(output)

    if copy_targets:
      ctx.actions.run(
          executable = ctx.executable._compiler,
          arguments = [
              "--version",
              ctx.attr.version,
              "--type",
              "copy",
              "--provider",
              "tpg",
              "--target",
              ",".join(copy_targets),
          ],
          inputs = depset([i for i in copy_inputs]),
          outputs = copy_outputs,
          mnemonic = "MmCopyFiles",
      )

    if template_targets:
      ctx.actions.run(
          executable = ctx.executable._compiler,
          arguments = [
              "--version",
              ctx.attr.version,
              "--type",
              "template",
              "--provider",
              "tpg",
              "--target",
              ",".join(template_targets),
          ],
          inputs = depset([i for i in template_inputs]),
          outputs = template_outputs,
          mnemonic = "MmTemplateFiles",
      )
    return [
        DefaultInfo(files = depset([out for out in copy_outputs] + [out for out in template_outputs])),
    ]

mm_template_library = rule(
    implementation = _mm_template_library_impl,
    attrs = {
        "version": attr.string(default = "ga", mandatory = False),
        "srcs": attr.label_list(
          allow_files = [".go"]
        ),
        "template_srcs": attr.label_list(
          allow_files = [".tmpl"]
        ),
        "_compiler": attr.label(
            default = Label("//mmv1/cmd"),
            allow_single_file = True,
            executable = True,
            cfg = "exec",
        ),
    },
)


def _mm_go_library_impl(ctx):
    # TODO: version import paths
    copy_inputs = [f for f in ctx.files.srcs]
    copy_outputs = []
    copy_targets = []
    for f in copy_inputs:
        filename = f.path.split("/")[-1]
        output = ctx.actions.declare_file("{}/{}".format(ctx.attr.version, filename))
        copy_targets.append("{}={}".format(f.path, output.path))
        copy_outputs.append(output)

    template_inputs = [f for f in ctx.files.template_srcs]
    template_outputs = []
    template_targets = []
    for f in template_inputs:
        filename = f.path.split("/")[-1]
        basename = ".".join(filename.split(".")[0:-1])
        suffix = filename.split(".")[-1]
        if suffix != "tmpl":
          fail(args=["template_srcs entry '{}' must end with a .tmpl extension".format(filename)])
        output = ctx.actions.declare_file("{}/{}".format(ctx.attr.version, basename))
        template_targets.append("{}={}".format(f.path, output.path))
        template_outputs.append(output)

    if copy_targets:
      ctx.actions.run(
          executable = ctx.executable._compiler,
          arguments = [
              "--version",
              ctx.attr.version,
              "--type",
              "copy",
              "--provider",
              "tpg",
              "--target",
              ",".join(copy_targets),
          ],
          inputs = depset([i for i in copy_inputs]),
          outputs = copy_outputs,
          mnemonic = "MmCopyFiles",
      )

    if template_targets:
      ctx.actions.run(
          executable = ctx.executable._compiler,
          arguments = [
              "--version",
              ctx.attr.version,
              "--type",
              "template",
              "--provider",
              "tpg",
              "--target",
              ",".join(template_targets),
          ],
          inputs = depset([i for i in template_inputs]),
          outputs = template_outputs,
          mnemonic = "MmTemplateFiles",
      )
    return [
        DefaultInfo(files = depset([out for out in copy_outputs] + [out for out in template_outputs])),
    ]

mm_go_library = rule(
    implementation = _mm_go_library_impl,
    attrs = {
        "version": attr.string(default = "ga", mandatory = False),
        "srcs": attr.label_list(
          allow_files = [".go"]
        ),
        "template_srcs": attr.label_list(
          allow_files = [".tmpl"]
        ),
        "_compiler": attr.label(
            default = Label("//mmv1/cmd"),
            allow_single_file = True,
            executable = True,
            cfg = "exec",
        ),
    },
)