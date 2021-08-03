#!/bin/bash
#
# This file is part of the KubeVirt project
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Copyright 2021 Red Hat, Inc.
#
set -e

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    source hack/common.sh
    source hack/config.sh
fi

sandbox_root=${SANDBOX_DIR}/default/root
sandbox_hash_x86_64="dcf10ba6720e92acd45e36e52e6ba010fa55de32"
sandbox_hash_aarch64="38ee5a6e0926ef69428e530516ec4a724fe1b884"

declare -A hashes=(["x86_64"]="${sandbox_hash_x86_64}" ["aarch64"]="${sandbox_hash_aarch64}")

function kubevirt::bootstrap::regenerate() {
    (
        if [ -f "${SANDBOX_DIR}/${hashes[${1}]}" ]; then
            echo "Sandbox is up to date"
            return
        fi
        echo "Regenerating sandbox"
        cd ${KUBEVIRT_DIR}
        rm ${SANDBOX_DIR} -rf
        rm sandbox.bazelrc -f
        bazel run --config ${HOST_ARCHITECTURE} //rpm:sandbox_${1}
        bazel clean

        cat <<EOT >>sandbox.bazelrc
build --sandbox_add_mount_pair=${sandbox_root}/usr/:/usr/
build --sandbox_add_mount_pair=${sandbox_root}/lib64:/lib64
build --sandbox_add_mount_pair=${sandbox_root}/lib:/lib
build --sandbox_add_mount_pair=${sandbox_root}/bin:/bin

build --incompatible_enable_cc_toolchain_resolution --platforms=//bazel/platforms:x86_64-none-linux-gnu
EOT
        local sha=$(kubevirt::bootstrap::sha256)
        touch ${SANDBOX_DIR}/${sha}
        sed -i "/^[[:blank:]]*sandbox_hash_${1}[[:blank:]]*=/s/=.*/=\"${sha}\"/" hack/bootstrap.sh
    )
}

function kubevirt::bootstrap::sha256() {
    (
        cd ${KUBEVIRT_DIR}
        find ${sandbox_root}/ -type f -exec sha256sum {} \; | sha256sum | head -c 40
    )
}
