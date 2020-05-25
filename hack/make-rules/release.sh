#!/usr/bin/env bash

###
#Copyright 2020 The KubeEdge Authors.
#
#Licensed under the Apache License, Version 2.0 (the "License");
#you may not use this file except in compliance with the License.
#You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
#Unless required by applicable law or agreed to in writing, software
#distributed under the License is distributed on an "AS IS" BASIS,
#WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#See the License for the specific language governing permissions and
#limitations under the License.
###

set -o errexit
set -o nounset
set -o pipefail

KUBEEDGE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
source "${KUBEEDGE_ROOT}/hack/lib/init.sh"
source "${KUBEEDGE_ROOT}/hack/lib/docker.sh"

# binaries
support_arch="amd64 arm arm64"
for arch in $support_arch; do
  ARCH=$arch kubeedge::golang::crossbuild_binaries
done

for dir in ${KUBEEDGE_OUTPUT_BINPATH}/*; do
  pushd $dir >/dev/null
  for file in *; do
    arch=${dir##*/}
    name="${file}-${VERSION}-linux-${arch}"
    sha256sum $file > "${name}.txt"
    tar -czf ${name}.tar.gz ${file}*
    sha256sum ${name}.tar.gz > "${name}.tar.gz.txt"
  done
  popd >/dev/null
done

mkdir output
cp -a ${KUBEEDGE_OUTPUT_BINPATH}/**/*.tar.gz* output/

# images
kubeedge::docker::all_build
kubeedge::docker::login
kubeedge::docker::push
