// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/verrazzano/pkg/diff"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	clientset "github.com/verrazzano/verrazzano-monitoring-operator/pkg/client/clientset/versioned"
	clientsetscheme "github.com/verrazzano/verrazzano-monitoring-operator/pkg/client/clientset/versioned/scheme"
	informers "github.com/verrazzano/verrazzano-monitoring-operator/pkg/client/informers/externalversions"
	listers "github.com/verrazzano/verrazzano-monitoring-operator/pkg/client/listers/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/metricsexporter"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/opensearch"
	dashboards "github.com/verrazzano/verrazzano-monitoring-operator/pkg/opensearch_dashboards"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/signals"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/upgrade"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslistersv1 "k8s.io/client-go/listers/apps/v1"
	corelistersv1 "k8s.io/client-go/listers/core/v1"
	netlistersv1 "k8s.io/client-go/listers/networking/v1"
	rbacv1listers1 "k8s.io/client-go/listers/rbac/v1"
	storagelisters1 "k8s.io/client-go/listers/storage/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

const controllerAgentName = "vmo-controller"

// Controller is the controller implementation for VMO resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// vmoclientset is a clientset for our own API group
	vmoclientset     clientset.Interface
	kubeextclientset apiextensionsclient.Interface

	// listers and syncs
	clusterRoleLister    rbacv1listers1.ClusterRoleLister
	clusterRolesSynced   cache.InformerSynced
	configMapLister      corelistersv1.ConfigMapLister
	configMapsSynced     cache.InformerSynced
	deploymentLister     appslistersv1.DeploymentLister
	deploymentsSynced    cache.InformerSynced
	ingressLister        netlistersv1.IngressLister
	ingressesSynced      cache.InformerSynced
	nodeLister           corelistersv1.NodeLister
	nodesSynced          cache.InformerSynced
	pvcLister            corelistersv1.PersistentVolumeClaimLister
	pvcsSynced           cache.InformerSynced
	roleBindingLister    rbacv1listers1.RoleBindingLister
	roleBindingsSynced   cache.InformerSynced
	secretLister         corelistersv1.SecretLister
	secretsSynced        cache.InformerSynced
	serviceLister        corelistersv1.ServiceLister
	servicesSynced       cache.InformerSynced
	statefulSetLister    appslistersv1.StatefulSetLister
	statefulSetsSynced   cache.InformerSynced
	vmoLister            listers.VerrazzanoMonitoringInstanceLister
	vmosSynced           cache.InformerSynced
	storageClassLister   storagelisters1.StorageClassLister
	storageClassesSynced cache.InformerSynced

	// misc
	namespace      string
	watchNamespace string
	watchVmi       string
	buildVersion   string
	stopCh         <-chan struct{}

	// multi-cluster
	clusterInfo ClusterInfo

	// config
	operatorConfigMapName string
	operatorConfig        *config.OperatorConfig
	latestConfigMap       *corev1.ConfigMap

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// lastEnqueue is the timestamp of when the last element was added to the queue
	lastEnqueue time.Time
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	// VerrazzanoLogger is used to log
	log vzlog.VerrazzanoLogger

	// OpenSearch Client
	osClient *opensearch.OSClient

	// OpenSearchDashboards Client
	osDashboardsClient *dashboards.OSDashboardsClient

	indexUpgradeMonitor *upgrade.Monitor
}

// ClusterInfo has info like ContainerRuntime and managed cluster name
type ClusterInfo struct {
	clusterName      string
	KeycloakURL      string
	KeycloakCABundle []byte
}

// NewController returns a new vmo controller
func NewController(namespace string, configmapName string, buildVersion string, kubeconfig string, masterURL string, watchNamespace string, watchVmi string) (*Controller, error) {

	zap.S().Debugw("Building config")
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		zap.S().Fatalf("Error building kubeconfig: %v", err)
	}

	zap.S().Debugw("Building kubernetes clientset")
	kubeclientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		zap.S().Fatalf("Error building kubernetes clientset: %v", err)
	}

	zap.S().Debugw("Building vmo clientset")
	vmoclientset, err := clientset.NewForConfig(cfg)
	if err != nil {
		zap.S().Fatalf("Error building vmo clientset: %v", err)
	}

	zap.S().Debugw("Building api extensions clientset")
	kubeextclientset, err := apiextensionsclient.NewForConfig(cfg)
	if err != nil {
		zap.S().Fatalf("Error building apiextensions-apiserver clientset: %v", err)
	}

	// Get the config from the ConfigMap
	zap.S().Debugw("Loading ConfigMap ", configmapName)

	operatorConfigMap, err := kubeclientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), configmapName, metav1.GetOptions{})
	if err != nil {
		zap.S().Fatalf("No configuration ConfigMap called %s found in namespace %s.", configmapName, namespace)
	}
	zap.S().Debugf("Building config from ConfigMap %s", configmapName)
	operatorConfig, err := config.NewConfigFromConfigMap(operatorConfigMap)
	if err != nil {
		zap.S().Fatalf("Error building verrazzano-monitoring-operator config from config map: %s", err.Error())
	}

	var kubeInformerFactory kubeinformers.SharedInformerFactory
	var vmoInformerFactory informers.SharedInformerFactory
	if watchNamespace == "" {
		// Consider all namespaces if our namespace is left wide open our set to default
		kubeInformerFactory = kubeinformers.NewSharedInformerFactory(kubeclientset, constants.ResyncPeriod)
		vmoInformerFactory = informers.NewSharedInformerFactory(vmoclientset, constants.ResyncPeriod)
	} else {
		// Otherwise, restrict to a specific namespace
		kubeInformerFactory = kubeinformers.NewSharedInformerFactoryWithOptions(kubeclientset, constants.ResyncPeriod, kubeinformers.WithNamespace(watchNamespace), kubeinformers.WithTweakListOptions(nil))
		vmoInformerFactory = informers.NewSharedInformerFactoryWithOptions(vmoclientset, constants.ResyncPeriod, informers.WithNamespace(watchNamespace), informers.WithTweakListOptions(nil))
	}

	// obtain references to shared index informers for the Deployment and VMO
	// types.
	clusterRoleInformer := kubeInformerFactory.Rbac().V1().ClusterRoles()
	configmapInformer := kubeInformerFactory.Core().V1().ConfigMaps()
	deploymentInformer := kubeInformerFactory.Apps().V1().Deployments()
	ingressInformer := kubeInformerFactory.Networking().V1().Ingresses()
	nodeInformer := kubeInformerFactory.Core().V1().Nodes()
	pvcInformer := kubeInformerFactory.Core().V1().PersistentVolumeClaims()
	roleBindingInformer := kubeInformerFactory.Rbac().V1().RoleBindings()
	secretsInformer := kubeInformerFactory.Core().V1().Secrets()
	serviceInformer := kubeInformerFactory.Core().V1().Services()
	statefulSetInformer := kubeInformerFactory.Apps().V1().StatefulSets()
	vmoInformer := vmoInformerFactory.Verrazzano().V1().VerrazzanoMonitoringInstances()
	storageClassInformer := kubeInformerFactory.Storage().V1().StorageClasses()
	// Create event broadcaster
	// Add vmo-controller types to the default Kubernetes Scheme so Events can be
	// logged for vmo-controller types.
	if err := clientsetscheme.AddToScheme(scheme.Scheme); err != nil {
		zap.S().Warnf("error adding scheme: %+v", err)
	}
	zap.S().Infow("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(zap.S().Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	zap.S().Infow("Creating OpenSearch client")
	osClient := opensearch.NewOSClient()

	zap.S().Infow("Creating OpenSearchDashboards client")
	osDashboardsClient := dashboards.NewOSDashboardsClient()

	controller := &Controller{
		namespace:        namespace,
		watchNamespace:   watchNamespace,
		watchVmi:         watchVmi,
		kubeclientset:    kubeclientset,
		vmoclientset:     vmoclientset,
		kubeextclientset: kubeextclientset,

		clusterRoleLister:     clusterRoleInformer.Lister(),
		clusterRolesSynced:    clusterRoleInformer.Informer().HasSynced,
		configMapLister:       configmapInformer.Lister(),
		configMapsSynced:      configmapInformer.Informer().HasSynced,
		deploymentLister:      deploymentInformer.Lister(),
		deploymentsSynced:     deploymentInformer.Informer().HasSynced,
		ingressLister:         ingressInformer.Lister(),
		ingressesSynced:       ingressInformer.Informer().HasSynced,
		nodeLister:            nodeInformer.Lister(),
		nodesSynced:           nodeInformer.Informer().HasSynced,
		pvcLister:             pvcInformer.Lister(),
		pvcsSynced:            pvcInformer.Informer().HasSynced,
		roleBindingLister:     roleBindingInformer.Lister(),
		roleBindingsSynced:    roleBindingInformer.Informer().HasSynced,
		secretLister:          secretsInformer.Lister(),
		secretsSynced:         secretsInformer.Informer().HasSynced,
		serviceLister:         serviceInformer.Lister(),
		servicesSynced:        serviceInformer.Informer().HasSynced,
		statefulSetLister:     statefulSetInformer.Lister(),
		statefulSetsSynced:    statefulSetInformer.Informer().HasSynced,
		vmoLister:             vmoInformer.Lister(),
		vmosSynced:            vmoInformer.Informer().HasSynced,
		storageClassLister:    storageClassInformer.Lister(),
		storageClassesSynced:  storageClassInformer.Informer().HasSynced,
		workqueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "VMOs"),
		recorder:              recorder,
		buildVersion:          buildVersion,
		operatorConfigMapName: configmapName,
		operatorConfig:        operatorConfig,
		latestConfigMap:       operatorConfigMap,
		clusterInfo:           ClusterInfo{},
		log:                   vzlog.DefaultLogger(),
		osClient:              osClient,
		osDashboardsClient:    osDashboardsClient,
		indexUpgradeMonitor:   &upgrade.Monitor{},
	}

	zap.S().Infow("Setting up event handlers")

	// Set up an event handler for when VMO resources change
	vmoInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueVMO,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueVMO(new)
		},
	})

	// Create watchers on the operator ConfigMap, which may signify a need to reload our config
	configMapInformer := kubeInformerFactory.Core().V1().ConfigMaps()
	configMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			newConfigMap := new.(*corev1.ConfigMap)
			// If the configMap has changed from our last known copy, process it
			if newConfigMap.Name == controller.operatorConfigMapName && !reflect.DeepEqual(newConfigMap.Data, controller.latestConfigMap.Data) {
				zap.S().Infof("Reloading config...")
				newOperatorConfig, err := config.NewConfigFromConfigMap(newConfigMap)
				if err != nil {
					zap.S().Errorf("Errors processing config updates - so we're staying at current configuration: %s", err)
				} else {
					zap.S().Infof("Successfully reloaded config")
					controller.operatorConfig = newOperatorConfig
					controller.latestConfigMap = newConfigMap
				}
			}
		},
	})

	// set up signals so we handle the first shutdown signal gracefully
	zap.S().Debugw("Setting up signals")
	controller.stopCh = signals.SetupSignalHandler()

	go kubeInformerFactory.Start(controller.stopCh)
	go vmoInformerFactory.Start(controller.stopCh)

	return controller, nil
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	zap.S().Infow("Starting VMO controller")

	// Wait for the caches to be synced before starting workers
	zap.S().Infow("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(c.stopCh, c.clusterRolesSynced, c.configMapsSynced,
		c.deploymentsSynced, c.ingressesSynced, c.nodesSynced, c.pvcsSynced, c.roleBindingsSynced, c.secretsSynced,
		c.servicesSynced, c.statefulSetsSynced, c.vmosSynced, c.storageClassesSynced); !ok {
		return errors.New("failed to wait for caches to sync")
	}

	zap.S().Infow("Starting workers")
	// Launch two workers to process VMO resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, c.stopCh)
	}

	zap.S().Infow("Started workers")
	<-c.stopCh
	zap.S().Infow("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// VMO resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// Process an update to a VMO
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return err
	}
	if c.watchVmi != "" && c.watchVmi != name {
		return nil
	}

	// Get the VMO resource with this namespace/name
	vmo, err := c.vmoLister.VerrazzanoMonitoringInstances(namespace).Get(name)
	if err != nil {
		runtime.HandleError(fmt.Errorf("error getting VMO %s in namespace %s: %v", name, namespace, err))
		return err
	}

	// Get the resource logger needed to log message using 'progress' and 'once' methods
	log, err := vzlog.EnsureResourceLogger(&vzlog.ResourceConfig{
		Name:           vmo.Name,
		Namespace:      vmo.Namespace,
		ID:             string(vmo.UID),
		Generation:     vmo.Generation,
		ControllerName: "vmi",
	})
	if err != nil {
		zap.S().Errorf("Failed to create controller logger for VMO controller", err)
	}
	c.log = log

	log.Progressf("Reconciling vmi resource %v, generation %v", types.NamespacedName{Namespace: vmo.Namespace, Name: vmo.Name}, vmo.Generation)
	return c.syncHandlerStandardMode(vmo)
}

// In Standard Mode, we compare the actual state with the desired, and attempt to
// converge the two.  We then update the Status block of the VMO resource
// with the current status.
func (c *Controller) syncHandlerStandardMode(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	var errorObserved bool
	functionMetric, functionError := metricsexporter.GetFunctionMetrics(metricsexporter.NamesReconcile)
	if functionError == nil {
		functionMetric.LogStart()
		defer func() { functionMetric.LogEnd(errorObserved) }()
	} else {
		return functionError
	}

	originalVMO := vmo.DeepCopy()

	// populate clusterInfo
	clusterSecret, err := c.secretLister.Secrets(constants.VerrazzanoSystemNamespace).Get(constants.MCRegistrationSecret)
	if err == nil {
		c.clusterInfo.clusterName = string(clusterSecret.Data[constants.ClusterNameData])
		c.clusterInfo.KeycloakURL = string(clusterSecret.Data[constants.KeycloakURLData])
		c.clusterInfo.KeycloakCABundle = clusterSecret.Data[constants.KeycloakCABundleData]
	}

	// If lock, controller will not sync/process the VMO env
	if vmo.Spec.Lock {
		c.log.Progressf("[%s/%s] Lock is set to true, this VMO env will not be synced/processed.", vmo.Name, vmo.Namespace)
		return nil
	}

	/*********************
	 * Initialize VMO Spec
	 **********************/
	InitializeVMOSpec(c, vmo)

	errorObserved = false

	/***************************************
	 * Configure Index AutoExpand settings
	 ****************************************/
	autoExpandIndexChannel := c.osClient.SetAutoExpandIndices(vmo)

	/*********************
	 * Configure ISM
	 **********************/
	ismChannel := c.osClient.ConfigureISM(vmo)

	/********************************************
	 * Migrate old indices if any to data streams
	*********************************************/
	err = c.indexUpgradeMonitor.MigrateOldIndices(c.log, vmo, c.osClient, c.osDashboardsClient)
	if err != nil {
		c.log.Errorf("Failed to migrate old indices to data stream: %v", err)
		errorObserved = true
	}

	/*********************
	 * Create RoleBindings
	 **********************/
	err = CreateRoleBindings(c, vmo)
	if err != nil {
		c.log.Errorf("Failed to create Role Bindings for VMI %s: %v", vmo.Name, err)
		errorObserved = true
	}

	/*********************
	* Create configmaps
	**********************/
	err = CreateConfigmaps(c, vmo)
	if err != nil {
		c.log.Errorf("Failed to create config maps for VMI %s: %v", vmo.Name, err)
		errorObserved = true
	}

	/*********************
	 * Create Services
	 **********************/
	err = CreateServices(c, vmo)
	if err != nil {
		c.log.Errorf("Failed to create Services for VMI %s: %v", vmo.Name, err)
		errorObserved = true
	}

	/*********************
	 * Create Persistent Volume Claims
	 **********************/
	pvcToAdMap, err := CreatePersistentVolumeClaims(c, vmo)
	if err != nil {
		c.log.Errorf("Failed to create/update PVCs for VMI %s: %v", vmo.Name, err)
		errorObserved = true
	}

	/*********************
	 * Create StatefulSets
	 **********************/
	existingCluster, err := CreateStatefulSets(c, vmo)
	if err != nil {
		errorObserved = true
	}

	/*********************
	 * Create Deployments
	 **********************/
	var deploymentsDirty bool
	if !errorObserved {
		deploymentsDirty, err = CreateDeployments(c, vmo, pvcToAdMap, existingCluster)
		if err != nil {
			functionMetric.IncError()
			errorObserved = true
		}
	}
	/*********************
	 * Create Ingresses
	 **********************/
	err = CreateIngresses(c, vmo)
	if err != nil {
		c.log.Errorf("Failed to create Ingresses for VMI %s: %v", vmo.Name, err)
		errorObserved = true
	}

	/*********************
	* Update VMO itself (if necessary, if anything has changed)
	**********************/
	specDiffs := diff.Diff(originalVMO, vmo)
	if specDiffs != "" {
		c.log.Debugf("Acquired lock in namespace: %s", vmo.Namespace)
		c.log.Debugf("VMO %s : Spec differences %s", vmo.Name, specDiffs)
		c.log.Oncef("Updating VMO")
		metric, err := metricsexporter.GetCounterMetrics(metricsexporter.NamesVMOUpdate)
		if err != nil {
			return err
		}
		metric.Inc()
		_, err = c.vmoclientset.VerrazzanoV1().VerrazzanoMonitoringInstances(vmo.Namespace).Update(context.TODO(), vmo, metav1.UpdateOptions{})
		if err != nil {
			c.log.Errorf("Failed to update status for VMI %s: %v", vmo.Name, err)
			errorObserved = true
		}
	}

	autExpandIndexErr := <-autoExpandIndexChannel
	if autExpandIndexErr != nil {
		c.log.Errorf("Failed to update auto expand settings for indices: %v", err)
		errorObserved = true
	}

	ismErr := <-ismChannel
	if ismErr != nil {
		c.log.Errorf("Failed to configure ISM Policies: %v", ismErr)
		errorObserved = true
	}

	if !errorObserved && !deploymentsDirty && len(c.buildVersion) > 0 && vmo.Spec.Versioning.CurrentVersion != c.buildVersion {
		// The spec.versioning.currentVersion field should not be updated to the new value until a sync produces no
		// changes.  This allows observers (e.g. the controlled rollout scripts used to put new versions of operator
		// into production) to know when a given vmo has been (mostly) updated, and thus when it's relatively safe to
		// start checking various aspects of the vmo for health.
		vmo.Spec.Versioning.CurrentVersion = c.buildVersion
		_, err = c.vmoclientset.VerrazzanoV1().VerrazzanoMonitoringInstances(vmo.Namespace).Update(context.TODO(), vmo, metav1.UpdateOptions{})
		if err != nil {
			c.log.Errorf("Failed to update currentVersion for VMI %s: %v", vmo.Name, err)
		} else {
			c.log.Oncef("Updated VMI currentVersion to %s", c.buildVersion)
			timeMetric, timeErr := metricsexporter.GetTimestampMetrics(metricsexporter.NamesVMOUpdate)
			if timeErr != nil {
				return timeErr
			}
			timeMetric.SetLastTime()
		}
	}

	// Create a Hash on vmo/Status object to identify changes to vmo spec
	hash, err := vmo.Hash()
	if err != nil {
		c.log.Errorf("Error getting VMO hash: %v", err)
	}
	if vmo.Status.Hash != hash {
		vmo.Status.Hash = hash
	}

	c.log.Oncef("Successfully synced VMI'%s/%s'", vmo.Namespace, vmo.Name)
	return nil
}

// enqueueVMO takes a VMO resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than VMO.
func (c *Controller) enqueueVMO(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}

	c.workqueue.AddRateLimited(key)
	c.lastEnqueue = time.Now()
}

// IsHealthy returns true if this controller is healthy, false otherwise. It's health is determined based on: (1) its
// workqueue is 0 or decreasing in a timely manner, (2) it can communicate with API server, and (3) the CRD exists.
func (c *Controller) IsHealthy() bool {
	metric, err := metricsexporter.GetGaugeMetrics(metricsexporter.NamesQueue)
	if err != nil {
		zap.S().Error("Unable to retrieve simple gauge metric in isHealthy function")
	} else {
		metric.Set(float64(c.workqueue.Len()))
	}
	// Make sure if workqueue > 0, make sure it hasn't remained for longer than 60 seconds.
	if startQueueLen := c.workqueue.Len(); startQueueLen > 0 {
		if time.Since(c.lastEnqueue).Seconds() > float64(60) {
			return false
		}
	}

	// Make sure the controller can talk to the API server and its CRD is defined.
	crds, err := c.kubeextclientset.ApiextensionsV1().CustomResourceDefinitions().List(context.TODO(), metav1.ListOptions{})
	// Error getting CRD from API server
	if err != nil {
		return false
	}
	// No CRDs defined
	if len(crds.Items) == 0 {
		return false
	}
	crdExists := false
	for _, crd := range crds.Items {
		if crd.Name == constants.VMOFullname {
			crdExists = true
		}
	}
	return crdExists
}
