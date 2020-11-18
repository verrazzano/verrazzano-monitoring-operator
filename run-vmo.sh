#!/bin/bash
# Copyright (c) 2020, Oracle and/or its affiliates.
# Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

# If on corporate network set proxy environment variables

# Customize these to provide the location of your verrazzano and verrazzano repos
export THIS_REPO=$(pwd)
export VERRAZZANO_INSTALLER_REPO=${THIS_REPO}/../verrazzano
  
echo "Building and installing the verrazzano-monitoring-operator."
cd ${THIS_REPO}
make go-install
echo ""

echo "Stopping the in-cluster verrazzano-monitoring-operator."
set -x
kubectl scale deployment verrazzano-monitoring-operator --replicas=0 -n verrazzano-system
set +x
echo ""
  
# Extract the images required by verrazzano-operator from values.yaml into environment variables.
export GRAFANA_IMAGE=$(grep grafanaImage ${VERRAZZANO_INSTALLER_REPO}/install/chart/values.yaml | cut -d':' -f2,3 | sed -e 's/^[[:space:]]*//')
export PROMETHEUS_IMAGE=$(grep prometheusImage ${VERRAZZANO_INSTALLER_REPO}/install/chart/values.yaml | cut -d':' -f2,3 | sed -e 's/^[[:space:]]*//')
export PROMETHEUS_INIT_IMAGE=$(grep prometheusInitImage ${VERRAZZANO_INSTALLER_REPO}/install/chart/values.yaml | cut -d':' -f2,3 | sed -e 's/^[[:space:]]*//')
export PROMETHEUS_GATEWAY_IMAGE=$(grep prometheusGatewayImage ${VERRAZZANO_INSTALLER_REPO}/install/chart/values.yaml | cut -d':' -f2,3 | sed -e 's/^[[:space:]]*//')
export ALERT_MANAGER_IMAGE=$(grep alertManagerImage ${VERRAZZANO_INSTALLER_REPO}/install/chart/values.yaml | cut -d':' -f2,3 | sed -e 's/^[[:space:]]*//')
export ELASTICSEARCH_WAIT_TARGET_VERSION=7.6.1
export ELASTICSEARCH_WAIT_IMAGE=$(grep esWaitImage ${VERRAZZANO_INSTALLER_REPO}/install/chart/values.yaml | cut -d':' -f2,3 | sed -e 's/^[[:space:]]*//')
export KIBANA_IMAGE=$(grep kibanaImage ${VERRAZZANO_INSTALLER_REPO}/install/chart/values.yaml | cut -d':' -f2,3 | sed -e 's/^[[:space:]]*//')
export ELASTICSEARCH_IMAGE=$(grep esImage ${VERRAZZANO_INSTALLER_REPO}/install/chart/values.yaml | cut -d':' -f2,3 | sed -e 's/^[[:space:]]*//')
export ELASTICSEARCH_INIT_IMAGE=$(grep esInitImage ${VERRAZZANO_INSTALLER_REPO}/install/chart/values.yaml | cut -d':' -f2,3 | sed -e 's/^[[:space:]]*//')
export VERRAZZANO_MONITORING_INSTANCE_API_IMAGE=$(grep monitoringInstanceApiImage ${VERRAZZANO_INSTALLER_REPO}/install/chart/values.yaml | cut -d':' -f2,3 | sed -e 's/^[[:space:]]*//')
export CONFIG_RELOADER_IMAGE=$(grep configReloaderImage ${VERRAZZANO_INSTALLER_REPO}/install/chart/values.yaml | cut -d':' -f2,3 | sed -e 's/^[[:space:]]*//')
export NODE_EXPORTER_IMAGE=$(grep nodeExporterImage ${VERRAZZANO_INSTALLER_REPO}/install/chart/values.yaml | head -1 | cut -d':' -f2,3 | sed -e 's/^[[:space:]]*//')

# Extract the API server realm from values.yaml.
export API_SERVER_REALM=$(grep apiServerRealm ${VERRAZZANO_INSTALLER_REPO}/install/chart/values.yaml | cut -d':' -f2 | sed -e 's/^[[:space:]]*//')
  
# Extract the Verrazzano system ingress IP from the NGINX ingress controller status.
export VERRAZZANO_SYSTEM_INGRESS_IP=$(kubectl get svc -n ingress-nginx ingress-controller-nginx-ingress-controller -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

export WATCH_VMI=${WATCH_VMI:-""}
export WATCH_NAMESPACE=${WATCH_NAMESPACE:-""}

cat <<EOF
Variables:

GRAFANA_IMAGE=${GRAFANA_IMAGE}
PROMETHEUS_IMAGE=${PROMETHEUS_IMAGE}
PROMETHEUS_INIT_IMAGE=${PROMETHEUS_INIT_IMAGE}
PROMETHEUS_GATEWAY_IMAGE=${PROMETHEUS_GATEWAY_IMAGE}
ALERT_MANAGER_IMAGE=${ALERT_MANAGER_IMAGE}
ELASTICSEARCH_WAIT_TARGET_VERSION=${ELASTICSEARCH_WAIT_TARGET_VERSION}
ELASTICSEARCH_WAIT_IMAGE=${ELASTICSEARCH_WAIT_IMAGE}
KIBANA_IMAGE=${KIBANA_IMAGE}
ELASTICSEARCH_IMAGE=${ELASTICSEARCH_IMAGE}
ELASTICSEARCH_INIT_IMAGE=${ELASTICSEARCH_INIT_IMAGE}
VERRAZZANO_MONITORING_INSTANCE_API_IMAGE=${VERRAZZANO_MONITORING_INSTANCE_API_IMAGE}
CONFIG_RELOADER_IMAGE=${CONFIG_RELOADER_IMAGE}
NODE_EXPORTER_IMAGE=${NODE_EXPORTER_IMAGE}

WATCH_VMI=${WATCH_VMI}
WATCH_NAMESPACE=${WATCH_NAMESPACE}
EOF

# Run the out-of-cluster Verrazzano operator.
cmd="verrazzano-monitoring-ctrl \
 --zap-log-level=debug \
 --namespace=verrazzano-system \
 --watchNamespace=${WATCH_NAMESPACE} \
 --watchVmi=${WATCH_VMI} \
 --kubeconfig=${KUBECONFIG:-${HOME}/.kube/config}"

echo "Command"
echo "${cmd}"
eval ${cmd}
