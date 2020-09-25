// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNoPvcs(t *testing.T) {
	threeAds := []string{"ad1", "ad2", "ad3"}
	adCounter := NewAdPvcCounter(threeAds)
	observedAd := adCounter.GetLeastUsedAd()
	assert.Equal(t, true, resources.SliceContains(threeAds, observedAd), "With 3 unused ADs")
}

func TestNoADs(t *testing.T) {
	var noAds []string
	adCounter := NewAdPvcCounter(noAds)
	observedAd := adCounter.GetLeastUsedAd()
	assert.Equal(t, "", observedAd, "With no found ADs")
}

func TestNewThreeNodeES(t *testing.T) {
	threeAds := []string{"ad1", "ad2", "ad3"}
	adCounter := NewAdPvcCounter(threeAds)
	first := adCounter.GetLeastUsedAd()
	adCounter.Inc(first)
	second := adCounter.GetLeastUsedAd()
	adCounter.Inc(second)
	third := adCounter.GetLeastUsedAd()
	adCounter.Inc(third)
	assert.Equal(t, true, resources.SliceContains(threeAds, first), "first")
	assert.Equal(t, true, resources.SliceContains(threeAds, second), "first")
	assert.Equal(t, true, resources.SliceContains(threeAds, third), "third")
	assert.NotEqual(t, first, second)
	assert.NotEqual(t, first, third)
	assert.NotEqual(t, second, third)
}

func TestOnePvc(t *testing.T) {
	threeAds := []string{"ad1", "ad2", "ad3"}
	adCounter := NewAdPvcCounter(threeAds)
	adCounter.Inc("ad1")
	observedAd := adCounter.GetLeastUsedAd()
	assert.Equal(t, true, resources.SliceContains(threeAds, observedAd), "With 2 unused ADs")
	assert.NotEqual(t, "ad1", observedAd, "With one used AD")
}

func TestTwoPvcs(t *testing.T) {
	threeAds := []string{"ad1", "ad2", "ad3"}
	adCounter := NewAdPvcCounter(threeAds)
	adCounter.Inc("ad1")
	adCounter.Inc("ad2")
	observedAd := adCounter.GetLeastUsedAd()
	assert.Equal(t, true, resources.SliceContains(threeAds, observedAd), "With 1 unused AD")
	assert.Equal(t, "ad3", observedAd, "With two used ADs")
}

func TestThreePvcs(t *testing.T) {
	threeAds := []string{"ad1", "ad2", "ad3"}
	adCounter := NewAdPvcCounter(threeAds)
	adCounter.Inc("ad1")
	adCounter.Inc("ad2")
	adCounter.Inc("ad3")
	observedAd := adCounter.GetLeastUsedAd()
	assert.Equal(t, true, resources.SliceContains(threeAds, observedAd), "With 0 unused ADs")
}

func TestFourPvcs(t *testing.T) {
	threeAds := []string{"ad1", "ad2", "ad3"}
	adCounter := NewAdPvcCounter(threeAds)
	adCounter.Inc("ad1")
	adCounter.Inc("ad1")
	adCounter.Inc("ad2")
	adCounter.Inc("ad3")
	observedAd := adCounter.GetLeastUsedAd()
	assert.Equal(t, true, resources.SliceContains(threeAds, observedAd), "With 0 unused ADs")
	assert.NotEqual(t, "ad1", observedAd, "With 3 used ADs")
}

func TestFivePvcs(t *testing.T) {
	threeAds := []string{"ad1", "ad2", "ad3"}
	adCounter := NewAdPvcCounter(threeAds)
	adCounter.Inc("ad1")
	adCounter.Inc("ad1")
	adCounter.Inc("ad2")
	adCounter.Inc("ad2")
	adCounter.Inc("ad3")
	observedAd := adCounter.GetLeastUsedAd()
	assert.Equal(t, true, resources.SliceContains(threeAds, observedAd), "With 0 unused ADs")
	assert.Equal(t, "ad3", observedAd, "With three used ADs")
}

func TestNonSchedulable(t *testing.T) {
	twoAds := []string{"ad1", "ad2"}
	adCounter := NewAdPvcCounter(twoAds)
	adCounter.Inc("ad1")
	adCounter.Inc("ad1")
	adCounter.Inc("ad2")
	adCounter.Inc("ad2")
	adCounter.Inc("ad3")
	observedAd := adCounter.GetLeastUsedAd()
	assert.Equal(t, true, resources.SliceContains(twoAds, observedAd), "With 0 unused ADs")
}

func TestParseStorageClassInfoOciFlex(t *testing.T) {
	storageClass := storagev1.StorageClass{ObjectMeta: v1.ObjectMeta{Name: "storageclass1"}, Provisioner: constants.OciFlexVolumeProvisioner}
	expectedStorageClassInfo := StorageClassInfo{Name: "storageclass1", PvcAcceptsZone: true, PvcZoneMatchLabel: constants.OciAvailabilityDomainLabel}
	assert.Equal(t, expectedStorageClassInfo, parseStorageClassInfo(&storageClass, &config.OperatorConfig{}), "OCI Flex Volume with no operator config")
}

func TestParseStorageClassInfoWithMatchLabel(t *testing.T) {
	storageClass := storagev1.StorageClass{ObjectMeta: v1.ObjectMeta{Name: "storageclass2"}, Provisioner: "someprovisioner"}
	expectedStorageClassInfo := StorageClassInfo{Name: "storageclass2", PvcAcceptsZone: true, PvcZoneMatchLabel: "somematchlabel"}
	assert.Equal(t, expectedStorageClassInfo, parseStorageClassInfo(&storageClass, &config.OperatorConfig{Pvcs: config.Pvcs{ZoneMatchLabel: "somematchlabel"}}), "Match label specified in operator config")
}

func TestParseStorageClassInfoWithoutMatchLabel(t *testing.T) {
	storageClass := storagev1.StorageClass{ObjectMeta: v1.ObjectMeta{Name: "storageclass3"}, Provisioner: "someprovisioner"}
	expectedStorageClassInfo := StorageClassInfo{Name: "storageclass3", PvcAcceptsZone: false}
	assert.Equal(t, expectedStorageClassInfo, parseStorageClassInfo(&storageClass, &config.OperatorConfig{}), "No match label specified in operator config")
}

func TestAdFromExistingPVC1(t *testing.T) {
	// Storage class accepts an AD, and the PVC is labels as expected
	storageClassInfo := StorageClassInfo{Name: "storageclass1", PvcAcceptsZone: true, PvcZoneMatchLabel: "somematchlabel"}
	pvc := corev1.PersistentVolumeClaim{Spec: corev1.PersistentVolumeClaimSpec{Selector: &v1.LabelSelector{MatchLabels: map[string]string{"somematchlabel": "zone1"}}}}
	assert.Equal(t, "zone1", getZoneFromExistingPvc(storageClassInfo, &pvc), "Existing PVC contains expected AD label")
}

func TestAdFromExistingPVC2(t *testing.T) {
	// Storage class accepts an AD, but the PVC is not labeled correctly - should just return an empty string
	storageClassInfo := StorageClassInfo{Name: "storageclass1", PvcAcceptsZone: true, PvcZoneMatchLabel: "somematchlabel"}
	pvc := corev1.PersistentVolumeClaim{}
	assert.Equal(t, "", getZoneFromExistingPvc(storageClassInfo, &pvc), "Existing PVC contains expected AD label")
}

func TestAdFromExistingPVC3(t *testing.T) {
	// Storage class doesn't accept an AD
	storageClassInfo := StorageClassInfo{Name: "storageclass1", PvcAcceptsZone: false}
	pvc := corev1.PersistentVolumeClaim{}
	assert.Equal(t, "", getZoneFromExistingPvc(storageClassInfo, &pvc), "Storage class doesn't accept AD labels on PVCs")
}

func TestGetDefaultStorageClass1(t *testing.T) {
	storageClass1 := storagev1.StorageClass{ObjectMeta: v1.ObjectMeta{Name: "storageclass1"}}
	storageClass2 := storagev1.StorageClass{ObjectMeta: v1.ObjectMeta{Name: "storageclass2", Annotations: map[string]string{constants.K8sDefaultStorageClassAnnotation: "true"}}}
	storageClass3 := storagev1.StorageClass{ObjectMeta: v1.ObjectMeta{Name: "storageclass3"}}
	result, _ := getDefaultStorageClass([]*storagev1.StorageClass{&storageClass1, &storageClass2, &storageClass3})
	assert.Equal(t, result, &storageClass2, "Default storage class found based on standard label")
}

func TestGetDefaultStorageClass2(t *testing.T) {
	storageClass1 := storagev1.StorageClass{ObjectMeta: v1.ObjectMeta{Name: "storageclass1"}}
	storageClass2 := storagev1.StorageClass{ObjectMeta: v1.ObjectMeta{Name: "storageclass2"}}
	storageClass3 := storagev1.StorageClass{ObjectMeta: v1.ObjectMeta{Name: "storageclass3", Annotations: map[string]string{constants.K8sDefaultStorageClassBetaAnnotation: "true"}}}
	result, _ := getDefaultStorageClass([]*storagev1.StorageClass{&storageClass1, &storageClass2, &storageClass3})
	assert.Equal(t, result, &storageClass3, "Default storage class found based on beta label")
}

func TestGetDefaultStorageClass3(t *testing.T) {
	storageClass1 := storagev1.StorageClass{ObjectMeta: v1.ObjectMeta{Name: "storageclass1"}}
	storageClass2 := storagev1.StorageClass{ObjectMeta: v1.ObjectMeta{Name: "storageclass2"}}
	storageClass3 := storagev1.StorageClass{ObjectMeta: v1.ObjectMeta{Name: "storageclass3"}}
	result, err := getDefaultStorageClass([]*storagev1.StorageClass{&storageClass1, &storageClass2, &storageClass3})
	var expectedStorageClass *storagev1.StorageClass = nil
	assert.Equal(t, expectedStorageClass, result, "No default storage class")
	assert.NotEqual(t, nil, err, "Error from no default storage class")
}

func TestGetDefaultStorageClass4(t *testing.T) {
	result, err := getDefaultStorageClass([]*storagev1.StorageClass{})
	var expectedStorageClass *storagev1.StorageClass = nil
	assert.Equal(t, expectedStorageClass, result, "No storage classes")
	assert.NotEqual(t, nil, err, "Error from no storage classes")
}
