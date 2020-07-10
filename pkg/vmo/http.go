// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package vmo

import (
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/wait"
	"net/http"
	"time"
)

// StartHTTPServer runs an embedded HTTP server for any VMO handlers as a "resilient" goroutine meaning it runs in
// the background and will be restarted if it dies.
func StartHTTPServer(controller *Controller) {
	setupHandlers(controller)
	go wait.Until(func() {
		glog.Info("Starting HTTP server")
		err := http.ListenAndServe(":8080", nil)
		if err != nil {
			glog.Errorf("Failed to start HTTP server for vmo: %s", err)
		}
	}, time.Second*3, wait.NeverStop)
}

func setupHandlers(controller *Controller) {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if controller.IsHealthy() {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("ok")); err != nil {
				glog.Errorf("error writing healthcheck response: %v", err)
			}
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
