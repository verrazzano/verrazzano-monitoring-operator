// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package metrics

import (
	"flag"
	"fmt"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

var DanglingPVC = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: constants.MetricsNameSpace,
		Name:      "dangling_pvc",
		Help:      "value tells the dangling pvc exists",
	},
	[]string{"pvc_name", "availability_domain"},
)

var Lock = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: constants.MetricsNameSpace,
		Name:      "do_not_sync",
		Help:      "value tells the Lock flag is on",
	},
	[]string{"namespace", "sauron_name"},
)

func RegisterMetrics() {
	prometheus.MustRegister(DanglingPVC)
	prometheus.MustRegister(Lock)
}

func StartServer(port int) {
	flag.IntVar(&port, "port", port, "Port on which to expose Prometheus metrics")
	flag.Parse()
	router := mux.NewRouter().StrictSlash(true)
	router.Handle("/metrics", promhttp.Handler())

	glog.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), router))
}
