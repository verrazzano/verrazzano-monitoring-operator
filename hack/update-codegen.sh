#!/bin/bash
# Copyright (C) 2020, 2022, Oracle and/or its affiliates.
# Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

set -o errexit
set -o nounset
set -o pipefail

CODEGEN_PATH=k8s.io/code-generator
GOPATH=$(go env GOPATH)
SCRIPT_ROOT=$(dirname $0)/..
# Obtain k8s.io/code-generator version
codeGenVer=$(go list -m -f '{{.Version}}' k8s.io/code-generator)
# ensure code-generator has been downloaded
go get -d k8s.io/code-generator@${codeGenVer}
CODEGEN_PKG=${CODEGEN_PKG:-${GOPATH}/pkg/mod/${CODEGEN_PATH}@${codeGenVer}}
echo "codegen_pkg = ${CODEGEN_PKG}"
chmod +x ${CODEGEN_PKG}/generate-groups.sh

GENERATED_ZZ_FILE=$SCRIPT_ROOT/pkg/apis/vmcontroller/v1/zz_generated.deepcopy.go
echo Remove $GENERATED_ZZ_FILE file if exist
rm -f $GENERATED_ZZ_FILE

GENERATED_CLIENT_DIR=$SCRIPT_ROOT/pkg/client
echo Remove $GENERATED_CLIENT_DIR dir if exist
rm -rf $GENERATED_CLIENT_DIR

# generate the code with:
# --output-base    because this script should also be able to run inside the vendor dir of
#                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
#                  instead of the $GOPATH directly. For normal projects this can be dropped.
${CODEGEN_PKG}/generate-groups.sh "deepcopy,client,informer,lister" \
  github.com/verrazzano/verrazzano-monitoring-operator/pkg/client github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis \
  vmcontroller:v1 \
  --output-base "${GOPATH}/temp" \
  --go-header-file ${SCRIPT_ROOT}/hack/custom-header.txt

ls ${GOPATH}/temp -r
