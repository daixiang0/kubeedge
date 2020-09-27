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

set -e
set -x

KUBEEDGE_ROOT=$PWD
WORKDIR=$(dirname $0)
E2E_DIR=$(realpath $(dirname $0)/..)
KUBERNETES_ROOT=$GOPATH/src/k8s.io/kubernetes
TEST=$1

prepare_tests() {
  echo "Download kubetest..."

  local infra_repo=$GOPATH/src/infra
  [[ -d $infra_repo ]] || git clone https://github.com/kubernetes/test-infra.git $infra_repo
  pushd $infra_repo
  go install ./kubetest
  popd

  echo "Download kubernetes tests..."

  # keep same with supported version
  [[ -d $KUBERNETES_ROOT ]] || git clone -b v1.18.6 https://github.com/kubernetes/kubernetes $KUBERNETES_ROOT
  cd $KUBERNETES_ROOT
  make WHAT=test/e2e/e2e.test

  # skip binary build
  ln -s "$(which kubectl)" _output/bin/kubectl
  ln -s "$(which ginkgo)" _output/bin/ginkgo
}

start_cluster() {
  ENABLE_DAEMON=true $KUBEEDGE_ROOT/hack/local-up-kubeedge.sh
}

run_tests() {
  cd $KUBERNETES_ROOT

  # docker-config-file flag is not introduced in 1.18.6
  sed -i "/docker-config-file/d" hack/ginkgo-e2e.sh

  local apps_skip_case="--ginkgo.skip=DaemonRestart"
  local node_skip_case="--ginkgo.skip='Pods Extended'"
  local storage_skip_case="--ginkgo.skip=nfs|csi-hostpath"

  local test_args
  if [[ ${TEST}x == "x" ]]; then
    test_args="${apps_skip_case} ${node_skip_case} ${storage_skip_case}"
  else
    test_args="-ginkgo.focus=sig-${TEST}"

    local skip_case
    eval skip_case='$'${TEST}_skip_case
    [[ ${skip_case}x == "x" ]] || test_args+=" ${skip_case}"
  fi

  kubetest --provider=local --test \
    --test_args="--ginkgo.skip=Slow|Serial|Flaky|Feature ${test_args}" \
    --check-version-skew=false \
    --down
}

#prepare_tests
#start_cluster
run_tests
