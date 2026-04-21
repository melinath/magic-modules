"""
Go related BUILD rules for Magic Modules.
"""

load("@rules_go//go:def.bzl", "GoInfo", "go_context", "new_go_info")

def _mm_go_library_impl(ctx):
    # TODO: versioned import paths
    copy_inputs = [f for f in ctx.files.srcs]
    copy_outputs = []
    copy_targets = []
    for f in copy_inputs:
        filename = f.path.split("/")[-1]
        output = ctx.actions.declare_file("{}/{}".format(ctx.attr.version, filename))
        copy_targets.append("{}={}".format(f.path, output.path))
        copy_outputs.append(output)

    template_inputs = [f for f in ctx.files.templates]
    template_outputs = []
    template_targets = []
    for f in template_inputs:
        filename = f.path.split("/")[-1]
        basename = ".".join(filename.split(".")[0:-1])
        suffix = filename.split(".")[-1]
        if suffix != "tmpl":
            fail(args = ["templates entry '{}' must end with a .tmpl extension".format(filename)])
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

    go = go_context(
        ctx,
        include_deprecated_properties = False,
        importpath = ctx.attr.importpath,
        importmap = ctx.attr.importmap,
        embed = ctx.attr.embed,
        go_context_data = ctx.attr._go_context_data,
    )
    go_info = new_go_info(
        go,
        ctx.attr,
        generated_srcs = copy_outputs + template_outputs,
        coverage_instrumented = False,
    )

    return [
        DefaultInfo(files = depset([out for out in copy_outputs] + [out for out in template_outputs])),
        go_info,
    ]

mm_go_library = rule(
    implementation = _mm_go_library_impl,
    attrs = {
        "version": attr.string(default = "ga", mandatory = False),
        "srcs": attr.label_list(
            allow_files = [".go"],
        ),
        "templates": attr.label_list(
            allow_files = [".tmpl"],
        ),
        "importpath": attr.string(),
        "importmap": attr.string(),
        "embed": attr.label_list(providers = [GoInfo]),
        "_go_context_data": attr.label(
            default = "@rules_go//:go_context_data",
        ),
        "_compiler": attr.label(
            default = Label("//mmv1/cmd"),
            allow_single_file = True,
            executable = True,
            cfg = "exec",
        ),
    },
    toolchains = ["@rules_go//go:toolchain"],
)
