http_archive(
    name = "io_bazel_rules_go",
    url = "https://github.com/bazelbuild/rules_go/releases/download/0.9.0/rules_go-0.9.0.tar.gz",
    sha256 = "4d8d6244320dd751590f9100cf39fd7a4b75cd901e1f3ffdfd6f048328883695",
)
http_archive(
    name = "bazel_gazelle",
    url = "https://github.com/bazelbuild/bazel-gazelle/releases/download/0.9/bazel-gazelle-0.9.tar.gz",
    sha256 = "0103991d994db55b3b5d7b06336f8ae355739635e0c2379dea16b8213ea5a223",
)

git_repository(
    name = "io_bazel_rules_docker",
    remote = "https://github.com/bazelbuild/rules_docker.git",
    tag = "v0.4.0",
)

load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_pull",
    "container_image",
    container_repositories = "repositories",
)
load("@io_bazel_rules_go//go:def.bzl", "go_rules_dependencies", "go_register_toolchains")
load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

go_rules_dependencies()
go_register_toolchains()
gazelle_dependencies()
container_repositories()

container_pull(
  name = "fedora",
  registry = "index.docker.io",
  repository = "library/fedora",
  digest = "sha256:1b9bfb4e634dc1e5c19d0fa1eb2e5a28a5c2b498e3d3e4ac742bd7f5dae08611",
  #tag = "27",
)

container_pull(
  name = "libvirt",
  registry = "index.docker.io",
  repository = "kubevirt/libvirt",
  digest = "sha256:ce2d4eb931f4e6027793ee6e1492d0bcbc68f9182ae2182c2599f8941d3596c6",
  #tag = "3.7.0-bazel",
)

new_local_repository(
    name = "libvirt_libs",
    # pkg-config --variable=libdir libvirt
    path = "/usr/lib64",
    build_file_content = """
cc_library(
    name = "libs",
    srcs = glob([
        "libvirt*.so",
    ]),
    visibility = ["//visibility:public"],
)
""",
)

new_local_repository(
    name = "libvirt_headers",
    # pkg-config --variable=includedir libvirt
    path = "/usr/include",
    build_file_content = """
cc_library(
    name = "headers",
    hdrs = glob([
        "libvirt/*.h",
    ]),
    visibility = ["//visibility:public"],
)
""",
)
