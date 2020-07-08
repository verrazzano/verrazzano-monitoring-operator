# Copyright (C) 2020, Oracle and/or its affiliates.
# Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
#!/bin/bash

if [ -z "$1" ]; then
    echo "[error] Missing path to the kubeconfig file to use for CLI requests"
    echo
	echo "Usage: scripts/get-externalIP.sh <KUBECONFIG>"
	exit 0
fi


for n in $(kubectl --kubeconfig=$1 get nodes --selector node-role.kubernetes.io/node="" -o 'jsonpath={.items[*].metadata.name}')
do
	status=$(kubectl get node $n -o jsonpath='{ range .status.conditions[?(@.type == "Ready")].status }{ @ }{ end }')
	if [[ "$status" == "True" ]]; then
	    unschedulable=$(kubectl --kubeconfig=$1 get node $n -o jsonpath='{.spec.unschedulable}')
	    if [[ ("$unschedulable" == "") || ("$unschedulable" == "false") ]]; then
	        externalIP=$(kubectl --kubeconfig=$1 get node $n -o jsonpath='{.status.addresses[?(@.type=="ExternalIP")].address}')
	        break
	    fi
	fi
done
if [ -n "$externalIP" ]; then
  echo $externalIP
else
  echo "Could not determine external IP to use"
  exit 1
fi
