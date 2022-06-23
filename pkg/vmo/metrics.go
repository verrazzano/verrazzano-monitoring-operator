package vmo

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/apimachinery/pkg/util/wait"
)

// StartMetricsServer starts the metrics endpoint. See StartHTTPsServer for the original implementation from which this function is derived.
func StartMetricsServer(controller *Controller) {

	go wait.Until(func() {
		controller.log.Oncef("Starting metrics server")
		http.Handle("/metrics", promhttp.Handler())
		err := http.ListenAndServe(":9100", nil)
		if err != nil {
			controller.log.Errorf("Failed to start metrics server for VMI: %v", err)
		}
	}, time.Second*3, wait.NeverStop)

}
