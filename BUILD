package(default_visibility = ["//visibility:public"])

load("@bazel_gazelle//:def.bzl", "gazelle")
load("@bazel_tools//tools/build_defs/pkg:pkg.bzl", "pkg_tar")
load("@io_bazel_rules_docker//contrib:push-all.bzl", "docker_push")
load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_bundle",
    container_repositories = "repositories",
)

gazelle(
    name = "gazelle",
    prefix = "kubevirt.io/kubevirt",
)

genrule(
    name = "alpine-iso",
    srcs = [],
    outs = ["disk/alpine.iso"],
    cmd = "mkdir disk && curl http://dl-cdn.alpinelinux.org/alpine/v3.7/releases/x86_64/alpine-virt-3.7.0-x86_64.iso > $@",
)

pkg_tar(
    name = "alpine-iso-tar",
    srcs = [":alpine-iso"],
    package_dir = "disk",
    visibility = ["//visibility:public"],
)

genrule(
    name = "cirros-iso",
    srcs = [],
    outs = ["disk/cirros.img"],
    cmd = "mkdir disk && curl https://download.cirros-cloud.net/0.4.0/cirros-0.4.0-x86_64-disk.img > $@",
)

pkg_tar(
    name = "cirros-iso-tar",
    srcs = [":cirros-iso"],
    package_dir = "disk",
    visibility = ["//visibility:public"],
)

genrule(
    name = "fedora-iso",
    srcs = [],
    outs = ["disk/fedora.qcow2"],
    cmd = "mkdir disk && curl -g -L https://download.fedoraproject.org/pub/fedora/linux/releases/27/CloudImages/x86_64/images/Fedora-Cloud-Base-27-1.6.x86_64.qcow2 > $@",
)

pkg_tar(
    name = "fedora-iso-tar",
    srcs = [":fedora-iso"],
    package_dir = "disk",
    visibility = ["//visibility:public"],
)


container_bundle(
    name = "bundle",
    images = {
            "localhost:5000/kubevirt/virt-launcher:devel": "//cmd/virt-launcher:virt-launcher-image",
            "localhost:5000/kubevirt/virt-controller:devel": "//cmd/virt-controller:virt-controller-image",
            "localhost:5000/kubevirt/virt-handler:devel": "//cmd/virt-handler:virt-handler-image",
            "localhost:5000/kubevirt/cirros-registry-disk-demo:devel": "//cmd/registry-disk-v1alpha:cirros-registry-disk-demo",
            "localhost:5000/kubevirt/alpine-registry-disk-demo:devel": "//cmd/registry-disk-v1alpha:alpine-registry-disk-demo",
            "localhost:5000/kubevirt/fedora-registry-disk-demo:devel": "//cmd/registry-disk-v1alpha:fedora-registry-disk-demo",
            "localhost:5000/kubevirt/vm-killer:devel": "@libvirt//image",
            "localhost:5000/kubevirt/iscsi-demo-target-tgtd:devel": "//images/iscsi-demo-target-tgtd:tgtd",
    },
)

docker_push(
    name = "push-all",
    bundle = ":bundle",
)
