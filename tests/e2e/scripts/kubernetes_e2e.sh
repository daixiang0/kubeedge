#!/usr/bin/env bash

# Copyright 2020 The KubeEdge Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

KUBEEDGE_ROOT=$PWD
WORKDIR=$(dirname $0)
E2E_DIR=$(realpath $(dirname $0)/..)

function cleanup {
  echo "Cleaning up..."

  kind delete cluster test
}

prepare_tests() {
  git clone -b v1.18.6 https://github.com/kubernetes/kubernetes $GOPATH/src/k8s.io/kubernetes
  git clone https://github.com/kubernetes/test-infra.git
  cd test-infra
  GO111MODULE=on go install ./kubetest
  cd $GOPATH/src/k8s.io/kubernetes
  kubetest --build
}

start_cluster() {
  ENABLE_DAEMON=true ${KUBEEDGE_ROOT}/hack/local-up-kubeedge.sh
}

run_tests() {
  cd $GOPATH/src/k8s.io/kubernetes

  # docker-config-file flag is not introduced in 1.18.6
  sed -i "/docker-config-file/d" hack/ginkgo-e2e.sh

  kubetest --provider=local --test \
		--test_args="--ginkgo.skip=Slow|Serial|Flaky|Feature" \
		--down
}

trap cleanup EXIT

prepare_tests
start_cluster
run_tests
