#!/bin/bash
#
# Copyright (c) 2020, Oracle and/or its affiliates.
# Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
#
# Code coverage generation
CVG_EXCLUDE="${CVG_EXCLUDE:-test}"

go test -coverpkg=./... -coverprofile ./coverage.cov $(go list ./... |  grep -Ev "${CVG_EXCLUDE}")

# Display the global code coverage.  This generates the total number the badge uses
go tool cover -func=coverage.cov ;

# If needed, generate HTML report
if [ "$1" == "html" ]; then
    go tool cover -html=coverage.cov -o coverage.html ;
fi



