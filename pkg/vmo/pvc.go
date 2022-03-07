// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/pvcs"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
)

// Creates PVCs for the given VMO instance.  Returns a pvc->AD map, which is populated *only if* AD information
// can be specified for new PVCs or determined from existing PVCs.  A pvc-AD map with empty AD values instructs the
// subsequent deployment processing logic to do the job of choosing ADs.
func createPersistentVolumeClaims(controller *Controller, vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) (map[string]string, error) {
	// Inspect the Storage Class to use
	storageClass, err := determineStorageClass(controller, vmo.Spec.StorageClass)
	if err != nil {
		return nil, err
	}
	storageClassInfo := parseStorageClassInfo(storageClass, controller.operatorConfig)

	pvcList, err := pvcs.New(vmo, storageClass.Name)
	if err != nil {
		controller.log.Errorf("Failed to create PVC specs for VMI %s: %v", vmo.Name, err)
		return nil, err
	}
	deploymentToAdMap := map[string]string{}

	controller.log.Oncef("Creating/updating PVCs for VMI %s", vmo.Name)

	// Get total list of all possible schedulable ADs
	schedulableADs, err := getSchedulableADs(controller)
	if err != nil {
		return deploymentToAdMap, err
	}

	// Keep track of ADs for Prometheus and Elasticsearch PVCs, to ensure they land on all different ADs
	prometheusAdCounter := NewAdPvcCounter(schedulableADs)
	elasticsearchAdCounter := NewAdPvcCounter(schedulableADs)

	var pvcNames []string
	for _, currPvc := range pvcList {
		pvcName := currPvc.Name
		pvcNames = append(pvcNames, pvcName)
		if pvcName == "" {
			// We choose to absorb the error here as the worker would requeue the
			// resource otherwise. Instead, the next time the resource is updated
			// the resource will be queued again.
			runtime.HandleError(errors.New(("Failed, PVC name must be specified")))
			return deploymentToAdMap, nil
		}

		controller.log.Debugf("Applying PVC '%s' in namespace '%s' for VMI '%s'\n", pvcName, vmo.Namespace, vmo.Name)
		existingPvc, err := controller.pvcLister.PersistentVolumeClaims(vmo.Namespace).Get(pvcName)

		// If the PVC already exists, we *only* read its current AD, *if possible* (this is not possible for all storage classes and situations)
		if existingPvc != nil {
			if storageClassInfo.PvcAcceptsZone {
				zone := getZoneFromExistingPvc(storageClassInfo, existingPvc)
				deploymentToAdMap[pvcName] = zone
				if strings.Contains(existingPvc.Name, config.Prometheus.Name) {
					prometheusAdCounter.Inc(zone)
				} else if strings.Contains(existingPvc.Name, "elasticsearch") {
					elasticsearchAdCounter.Inc(zone)
				}
			}
		} else {
			// If the StorageClass allows us to specify zone info on the PVC, we'll do that now
			var newAd string
			if storageClassInfo.PvcAcceptsZone {
				if strings.Contains(currPvc.Name, config.Prometheus.Name) {
					newAd = prometheusAdCounter.GetLeastUsedAd()
					prometheusAdCounter.Inc(newAd)
				} else if strings.Contains(currPvc.Name, "elasticsearch") {
					newAd = elasticsearchAdCounter.GetLeastUsedAd()
					elasticsearchAdCounter.Inc(newAd)
				} else {
					newAd = chooseRandomElementFromSlice(schedulableADs)
				}
				currPvc.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{storageClassInfo.PvcZoneMatchLabel: newAd}}
			}
			controller.log.Oncef("Creating PVC %s in AD %s", currPvc.Name, newAd)

			_, err = controller.kubeclientset.CoreV1().PersistentVolumeClaims(vmo.Namespace).Create(context.TODO(), currPvc, metav1.CreateOptions{})

			if err != nil {
				return deploymentToAdMap, err
			}

			deploymentToAdMap[pvcName] = newAd

		}
		if err != nil {
			return deploymentToAdMap, err
		}
		controller.log.Debugf("Successfully applied PVC '%s'\n", pvcName)
	}

	return deploymentToAdMap, nil
}

// AdPvcCounter type for AD PVC counts
type AdPvcCounter struct {
	pvcCountByAd map[string]int
}

// NewAdPvcCounter return new counter.  The provided ADs are the only ones schedulable; create entries in the map
func NewAdPvcCounter(ads []string) *AdPvcCounter {
	var counter AdPvcCounter
	counter.pvcCountByAd = make(map[string]int)
	for _, ad := range ads {
		counter.pvcCountByAd[ad] = 0
	}
	return &counter
}

// Inc increments counter. Any AD not already in map is not schedulable, so ignore
func (p *AdPvcCounter) Inc(ad string) {
	if _, ok := p.pvcCountByAd[ad]; ok {
		p.pvcCountByAd[ad] = p.pvcCountByAd[ad] + 1
	}
}

// GetLeastUsedAd returns least used AD
func (p *AdPvcCounter) GetLeastUsedAd() string {
	adsByPvcCount := make(map[int][]string)
	var pvcCounts []int
	for ad, count := range p.pvcCountByAd {
		adsByPvcCount[count] = append(adsByPvcCount[count], ad)
		pvcCounts = append(pvcCounts, count)
	}
	if len(pvcCounts) == 0 {
		return ""
	}
	// Now sort the PVC-counts-per-AD to put the smallest count at element 0
	sort.Ints(pvcCounts)
	// Get the array of ADs that have that smallest PVC count, and pick one at random
	candidateAds := adsByPvcCount[pvcCounts[0]]
	return chooseRandomElementFromSlice(candidateAds)
}

// Determines the storage class to use for the current environment
func determineStorageClass(controller *Controller, className *string) (*storagev1.StorageClass, error) {
	if className != nil {
		// If a storage class was explicitly specified via the VMO API, use that
		return getStorageClassByName(controller, *className)
	} else if controller.operatorConfig.Pvcs.StorageClass != "" {
		// if a storageclass was configured in the operator, use that
		return getStorageClassByName(controller, controller.operatorConfig.Pvcs.StorageClass)
	}

	// Otherwise we'll use the "default" storage class
	storageClasses, err := controller.storageClassLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	return getDefaultStorageClass(storageClasses)
}

func getStorageClassByName(controller *Controller, className string) (*storagev1.StorageClass, error) {
	storageClass, err := controller.storageClassLister.Get(className)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch storage class %s: %v", className, err)
	}

	return storageClass, err
}

// Parses the given storage class into a StorageClassInfo objects
func parseStorageClassInfo(storageClass *storagev1.StorageClass, operatorConfig *config.OperatorConfig) StorageClassInfo {
	pvcAcceptsZone := false
	pvcZoneMatchLabel := ""

	if storageClass.Provisioner == constants.OciFlexVolumeProvisioner { // Special case - we already know how to handle the OCI flex volume storage class
		pvcAcceptsZone = true
		pvcZoneMatchLabel = constants.OciAvailabilityDomainLabel
	} else if operatorConfig.Pvcs.ZoneMatchLabel != "" { // The user has explicitly specified to use zone match labels
		pvcAcceptsZone = true
		pvcZoneMatchLabel = operatorConfig.Pvcs.ZoneMatchLabel
	}

	return StorageClassInfo{
		Name:              storageClass.Name,
		PvcAcceptsZone:    pvcAcceptsZone,
		PvcZoneMatchLabel: pvcZoneMatchLabel,
	}
}

// Determines the availability domain from the given PVC, if possible.
func getZoneFromExistingPvc(storageClassInfo StorageClassInfo, existingPvc *corev1.PersistentVolumeClaim) string {
	zone := ""

	// If the StorageClass has allowed us to specify zone info on the PVC, we'll read that from the existing PVC
	if storageClassInfo.PvcAcceptsZone && existingPvc.Spec.Selector != nil && existingPvc.Spec.Selector.MatchLabels != nil {
		if thisZone, ok := existingPvc.Spec.Selector.MatchLabels[storageClassInfo.PvcZoneMatchLabel]; ok {
			zone = thisZone
		}
	}
	return zone
}

// Determines the "default" storage class from a list of storage classes.
func getDefaultStorageClass(storageClasses []*storagev1.StorageClass) (*storagev1.StorageClass, error) {
	for _, storageClass := range storageClasses {
		if storageClass.ObjectMeta.Annotations[constants.K8sDefaultStorageClassAnnotation] == "true" ||
			storageClass.ObjectMeta.Annotations[constants.K8sDefaultStorageClassBetaAnnotation] == "true" {
			return storageClass, nil
		}
	}
	return nil, fmt.Errorf("Failed to find a default storage class")
}
