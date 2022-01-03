#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(cd $(dirname "${BASH_SOURCE[0]}")/.. && pwd)
CONTROLLER_GEN="go run -modfile=${SCRIPT_ROOT}/hack/go.mod sigs.k8s.io/controller-tools/cmd/controller-gen"
DIEGEN="go run -modfile=${SCRIPT_ROOT}/hack/go.mod dies.dev/diegen"

( cd $SCRIPT_ROOT ; $CONTROLLER_GEN object:headerFile="./hack/boilerplate.go.txt" paths="./..." )
( cd $SCRIPT_ROOT ; $DIEGEN die:headerFile="./hack/boilerplate.go.txt" paths="./..." )
