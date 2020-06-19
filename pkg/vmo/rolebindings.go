// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package vmo

import (
	"errors"
	"github.com/golang/glog"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/diff"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func CreateRoleBindings(controller *Controller, sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	glog.V(4).Infof("Creating/updating RoleBindings for sauron '%s' in namespace '%s'", sauron.Name, sauron.Namespace)

	newRoleBindings, err := NewRoleBindings(sauron, controller)
	if err != nil {
		return err
	}

	var roleBindingNames []string
	ownerReferences := getHyperOperatorOwnerReferences(controller)
	for _, newRoleBinding := range newRoleBindings {
		roleBindingNames = append(roleBindingNames, newRoleBinding.Name)
		newRoleBinding.OwnerReferences = ownerReferences // set OwnerReferences to the Hyper Operator deployment
		existingRoleBinding, _ := controller.roleBindingLister.RoleBindings(sauron.Namespace).Get(newRoleBinding.Name)
		var err error
		if existingRoleBinding != nil {
			specDiffs := diff.CompareIgnoreTargetEmpties(existingRoleBinding, newRoleBinding)
			if specDiffs != "" {
				glog.V(4).Infof("RoleBinding %s : Spec differences %s", newRoleBinding.Name, specDiffs)
				err = controller.kubeclientset.RbacV1().RoleBindings(sauron.Namespace).Delete(newRoleBinding.Name, &metav1.DeleteOptions{})
				if err != nil {
					glog.Errorf("Problem deleting role binding %s: %+v", newRoleBinding.Name, err)
				}
				_, err = controller.kubeclientset.RbacV1().RoleBindings(sauron.Namespace).Create(newRoleBinding)
			}
		} else {
			_, err = controller.kubeclientset.RbacV1().RoleBindings(sauron.Namespace).Create(newRoleBinding)
		}
		if err != nil {
			return err
		}
	}

	// Delete RoleBindings that shouldn't exist
	glog.V(4).Infof("Deleting unwanted RoleBindings for sauron '%s' in namespace '%s'", sauron.Name, sauron.Namespace)
	selector := labels.SelectorFromSet(map[string]string{constants.SauronLabel: sauron.Name})
	existingRoleBindings, err := controller.roleBindingLister.RoleBindings(sauron.Namespace).List(selector)
	if err != nil {
		return err
	}
	// While we transition from the old to the new per-Sauron RoleBinding name, the following line explicitly adds
	// the *old* RoleBinding to the list of RoleBindings to remove.  It is otherwise not in the existingRoleBindingsList
	// list because we didn't originally add our usual Sauron label set to it...
	oldRoleBinding, _ := controller.roleBindingLister.RoleBindings(sauron.Namespace).Get("sauron-instance-role-binding")
	if oldRoleBinding != nil {
		existingRoleBindings = append(existingRoleBindings, oldRoleBinding)
	}
	for _, roleBinding := range existingRoleBindings {
		if !contains(roleBindingNames, roleBinding.Name) {
			glog.V(4).Infof("Deleting RoleBinding %s", roleBinding.Name)
			err := controller.kubeclientset.RbacV1().RoleBindings(sauron.Namespace).Delete(roleBinding.Name, &metav1.DeleteOptions{})
			if err != nil {
				glog.Errorf("Failed to delete RoleBinding %s, for the reason (%v)", roleBinding.Name, err)
				return err
			}
		}
	}
	return nil
}

// Constructs the necessary RoleBindings for a Sauron instance's Sub-Operator.
func NewRoleBindings(sauron *vmcontrollerv1.VerrazzanoMonitoringInstance, controller *Controller) ([]*rbacv1.RoleBinding, error) {
	instanceClusterRole, err := findClusterRole(controller, constants.ClusterRoleForSauronInstances)
	if err != nil {
		return nil, err
	}

	roleBindings := []*rbacv1.RoleBinding{
		{
			ObjectMeta: metav1.ObjectMeta{
				Labels:          resources.GetMetaLabels(sauron),
				Name:            resources.GetMetaName(sauron.Name, constants.RoleBindingForSauronInstance),
				Namespace:       sauron.Namespace,
				OwnerReferences: resources.GetOwnerReferences(sauron),
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "default",
					Namespace: sauron.Namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     instanceClusterRole.Name,
			},
		},
	}
	return roleBindings, nil
}

// Search first for a ClusterRole associated with the namespace.  If that fails, look for one associated
// with the default namespace.  This check is mainly to keep integration tests (one particular situation where the Helm
// chart is deployed with a namespace) working smoothly.
func findClusterRole(controller *Controller, prefix string) (*rbacv1.ClusterRole, error) {
	clusterRole, _ := controller.clusterRoleLister.Get(prefix + "-" + controller.namespace)
	if clusterRole == nil {
		clusterRole, _ = controller.clusterRoleLister.Get(prefix + "-default")
		if clusterRole == nil {
			return nil, errors.New("unable to find a valid ClusterRole to assign to Sauron instances")
		}
	}
	return clusterRole, nil
}
