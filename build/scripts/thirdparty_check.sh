#!/bin/sh
#
# Copyright (c) 2020, Oracle Corporation and/or its affiliates.
# Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
#
set -e

SCRIPT_DIR=$(cd $(dirname $0); pwd -P)
PROJECT_DIR=$(cd ${SCRIPT_DIR}/../.. ; pwd -P)

command -v md5sum >/dev/null 2>&1 || {
    echo >&2 "md5sum is required but cannot be found  Aborting.";
    exit 1;
}

echo "Verifying THIRD_PARTY_LICENSES.txt file is up to date with project dependencies."
echo "If the following checks fail, please update the THIRD_PARTY_LICENSES.txt file "
echo "following the steps in the project README.md"

test -e ${PROJECT_DIR}/THIRD_PARTY_LICENSES.txt

GOMOD_SUM=$(md5sum go.mod | awk '{print $1}')
THIRDPARTY_LICENSE_SUM=$(tail -1 ${PROJECT_DIR}/THIRD_PARTY_LICENSES.txt)
EXPECTED_THIRD_PARTY_LICENSE_SUM="License file based on go.mod with md5 sum: ${GOMOD_SUM}"

if [ "${THIRDPARTY_LICENSE_SUM}" != "${EXPECTED_THIRD_PARTY_LICENSE_SUM}" ] ; then
    echo "Third party license file ends with '${THIRDPARTY_LICENSE_SUM}' expected '${EXPECTED_THIRD_PARTY_LICENSE_SUM}'"
    exit 1
fi

exit 0

