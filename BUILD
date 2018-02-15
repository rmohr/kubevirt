package(default_visibility = ["//visibility:public"])

load("@bazel_gazelle//:def.bzl", "gazelle")

gazelle(
    name = "gazelle",
    prefix = "kubevirt.io/kubevirt",
)

load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_pull",
    "container_push",
    "container_image",
    container_repositories = "repositories",
)

container_image(
    name = "virt-launcher",
    base = "@libvirt//image",
    entrypoint = ["/entrypoint.sh"],
    files = [
        "_out/cmd/virt-launcher/entrypoint.sh",
        "_out/cmd/virt-launcher/etc",
        "_out/cmd/virt-launcher/libvirtd.sh",
        "_out/cmd/virt-launcher/sock-connector",
        "_out/cmd/virt-launcher/virt-launcher",
    ],
)

container_push(
    name = "push-virt-launcher",
    format = "Docker",
    image = ":virt-launcher",
    registry = "localhost:5000",
    repository = "kubevirt/virt-launcher",
    tag = "devel",
)

container_image(
    name = "virt-handler",
    base = "@fedora//image",
    entrypoint = ["/virt-handler"],
    files = [
        "_out/cmd/virt-handler/virt-handler",
    ],
)

container_push(
    name = "push-virt-handler",
    format = "Docker",
    image = ":virt-handler",
    registry = "localhost:5000",
    repository = "kubevirt/virt-handler",
    tag = "devel",
)
