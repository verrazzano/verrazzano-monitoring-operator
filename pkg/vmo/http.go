// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"net/http"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
)

// StartHTTPServer runs an embedded HTTP server for any VMO handlers as a "resilient" goroutine meaning it runs in
// the background and will be restarted if it dies.
func StartHTTPServer(controller *Controller) {
	setupHandlers(controller)
	go wait.Until(func() {
		controller.log.Oncef("Starting HTTP server")
		err := http.ListenAndServeTLS(":8080", "/etc/certs/tls.crt", "/etc/certs/tls.key", nil)
		if err != nil {
			zap.S().Errorf("Failed to start HTTP server for vmo: %s", err)
		}
	}, time.Second*3, wait.NeverStop)
}

func setupHandlers(controller *Controller) {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if controller.IsHealthy() {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("ok")); err != nil {
				zap.S().Errorf("error writing healthcheck response: %v", err)
			}
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
