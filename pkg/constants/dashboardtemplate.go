// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package constants

const GrafanaTmplPrometheusURI = "PrometheusURI"
const GrafanaTmplAlertManagerURI = "AlertManagerURI"

// Define the grafana dashboard provisioning provider. Here you specify the path where your dashboard
// ConfigMap will be mounted.
const (
	DashboardProviderTmpl = `
apiVersion: 1
providers:
- name: 'SauronProvider'
  orgId: 1
  folder: ''
  type: file
  disableDeletion: false
  editable: true
  options:
    path: /etc/grafana/provisioning/dashboards
`

	DataSourcesTmpl = `
apiVersion: 1

datasources:
- name: Prometheus
  type: prometheus
  orgId: 1
  access: proxy
  url: http://{{.PrometheusURI}}:9090
  isDefault: true
- name: Prometheus AlertManager
  type: camptocamp-prometheus-alertmanager-datasource
  orgId: 1
  access: proxy
  url: http://{{.AlertManagerURI}}:9093
  isDefault: false
  jsonData:
    severity_critical: "4"
    severity_high: "3"
    severity_warning: "2"
    severity_info: "1"`
)
