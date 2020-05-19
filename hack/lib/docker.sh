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
GO_LDFLAGS="$(bash ${KUBEEDGE_ROOT}/hack/make-rules/version.sh)"

kubeedge::docker::build_images(){
  local binArg="$@"

  case $binArg in
    cloudcore | admission | csidriver)
      kubeedge::docker::origin_build $binArg
      ;;
    edgecore | edgesite)
      kubeedge::docker::qemu_build $binArg
      ;;
    bluetooth)
      kubeedge::docker::mapper_build $binArg
      ;;
    *)
      kubeedge::docker::all_build
  esac
}

kubeedge::docker::all_build() {
  local origin_components="cloudcore admission csidriver"
  for component in $origin_components; do
    kubeedge::docker::origin_build $component
  done

  local qemu_components="edgecore edgesite"
  for component in $qemu_components; do
    kubeedge::docker::qemu_build $component
  done
}

kubeedge::docker::origin_build() {
  local component="$1"
  docker build --build-arg GO_LDFLAGS="${GO_LDFLAGS}" -t "kubeedge/${component}:${VERSION}" -f ${KUBEEDGE_ROOT}/build/${component}/Dockerfile ${KUBEEDGE_ROOT}
}

kubeedge::docker::qemu_build() {
  local component="$1"
  rm -rf ${KUBEEDGE_ROOT}/build/${component}/tmp
  mkdir -p ${KUBEEDGE_ROOT}/build/${component}/tmp
  curl -L -o ${KUBEEDGE_ROOT}/build/${component}/tmp/qemu-${QEMU_ARCH}-static.tar.gz https://github.com/multiarch/qemu-user-static/releases/download/v3.0.0/qemu-${QEMU_ARCH}-static.tar.gz
  tar -xzf ${KUBEEDGE_ROOT}/build/${component}/tmp/qemu-${QEMU_ARCH}-static.tar.gz -C ${KUBEEDGE_ROOT}/build/${component}/tmp
  docker build -t kubeedge/${component}:${VERSION} \
    --build-arg GO_LDFLAGS="${GO_LDFLAGS}" \
    --build-arg BUILD_FROM=${ARCH}/golang:1.13.8-alpine3.10 \
    --build-arg RUN_FROM=${ARCH}/docker:dind \
    -f ${KUBEEDGE_ROOT}/build/${component}/Dockerfile ${KUBEEDGE_ROOT}
}

kubeedge::docker::mapper_build() {
  local component="$1"
  docker build --build-arg GO_LDFLAGS="${GO_LDFLAGS}" -t "kubeedge/${component}:${VERSION}" -f ${KUBEEDGE_ROOT}/mappers/${component}/Dockerfile ${KUBEEDGE_ROOT}
}

kubeedge::docker::login() {
    echo "${DOCKERHUB_PASSWORD}" | docker login -u="${DOCKERHUB_USERNAME}" --password-stdin
}

kubeedge::docker::push() {
    local components="cloud admission csidriver edge edgesite"
    for component in "$components"; do
        docker push "kubeedge/${component}:${VERSION}"
    done
}
