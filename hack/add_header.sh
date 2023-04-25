#!/bin/bash
# Copyright (C) 2022, Oracle and/or its affiliates.
# Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

FILE="$1"
echo '# Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.' | cat - $FILE > tmp && mv tmp $FILE
echo '# Copyright (C) 2020, 2023, Oracle and/or its affiliates.' | cat - $FILE > tmp && mv tmp $FILE
