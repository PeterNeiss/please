go_library(
    name = "plzinit",
    srcs = [
        "go.go",
        "init.go",
        "pleasings.go",
        "plugin_go.go",
        "plugins.go",
    ],
    pgo_file = "//:pgo",
    visibility = ["PUBLIC"],
    deps = [
        "///third_party/go/github.com_coreos_go-semver//semver",
        "///third_party/go/github.com_please-build_buildtools//build",
        "///third_party/go/github.com_please-build_gcfg//ast",
        "//src/assets",
        "//src/cli",
        "//src/cli/logging",
        "//src/core",
        "//src/fs",
        "//src/scm",
        "//src/version",
    ],
)

go_test(
    name = "plzinit_test",
    srcs = [
        "init_test.go",
        "plugin_go_test.go",
        "plugins_test.go",
    ],
    data = ["//plugins:build_file"],
    deps = [
        ":plzinit",
        "///third_party/go/github.com_please-build_buildtools//build",
        "///third_party/go/github.com_stretchr_testify//assert",
        "///third_party/go/github.com_stretchr_testify//require",
        "//src/fs",
    ],
)
