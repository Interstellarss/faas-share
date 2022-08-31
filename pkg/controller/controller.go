package controller

import (
	"context"
	"fmt"
	"github.com/Interstellarss/faas-share/pkg/devicemanager"
	"github.com/Interstellarss/faas-share/pkg/k8s"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	corelister "k8s.io/client-go/listers/core/v1"
	"strconv"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	//"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	//"k8s.io/klog"
	glog "k8s.io/klog"

	//utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	faasv1 "github.com/Interstellarss/faas-share/pkg/apis/faasshare/v1"
	clientset "github.com/Interstellarss/faas-share/pkg/client/clientset/versioned"
	faasscheme "github.com/Interstellarss/faas-share/pkg/client/clientset/versioned/scheme"
	informers "github.com/Interstellarss/faas-share/pkg/client/informers/externalversions"
	listers "github.com/Interstellarss/faas-share/pkg/client/listers/faasshare/v1"
	//faasv1 "github.com/Interstellarss/faas-share/pkg/apis/faas_share/v1"
)

const (
	controllerAgentName = "faasshare-operator"
	faasKind            = "SharePod"
	functionPort        = 8080
	LabelMinReplicas    = "com.openfaas.scale.min"
	// SuccessSynced is used as part of the Event 'reason' when a Function is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Function fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by OpenFaaS"
	// MessageResourceSynced is the message used for an Event fired when a Function
	// is synced successfully
	MessageResourceSynced = "SharePod synced successfully"
)

var (
	KeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

// Controller is the controller implementation for Function resources
type Controller struct {
	// kubeclient is a standard kubernetes clientset
	kubeclient kubernetes.Interface
	// faasclientset is a clientset for our own API group
	faasclientset clientset.Interface

	//here is different from the controller in KubeShare, need to check here
	//deploymentsLister appslisters.DeploymentLister
	//deploymentsSynced cache.InformerSynced
	sharepodsLister listers.SharePodLister

	// sharepodsSynced returns true if the pod store has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	sharepodsSynced cache.InformerSynced

	nodelister corelister.NodeLister

	nodesSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	resolver *k8s.FunctionLookup

	// OpenFaaS function factory
	factory FunctionFactory

	//podQueue for handling events
	//podQueue
}

// NewController returns a new OpenFaaS controller
func NewController(
	kubeclientset kubernetes.Interface,
	faasclientset clientset.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	faasInformerFactory informers.SharedInformerFactory,
	factory FunctionFactory,
	resolver *k8s.FunctionLookup) *Controller {

	// obtain references to shared index informers for the Deployment and Function types
	//deploymentInformer := kubeInformerFactory.Apps().V1().Deployments()
	faasInformer := faasInformerFactory.Kubeshare().V1().SharePods()

	// Create event broadcaster
	// Add o6s types to the default Kubernetes Scheme so Events can be
	// logged for faas-controller types.
	faasscheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.V(4).Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclient:      kubeclientset,
		faasclientset:   faasclientset,
		sharepodsLister: faasInformer.Lister(),
		sharepodsSynced: faasInformer.Informer().HasSynced,
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Sharepods"),
		recorder:        recorder,
		resolver:        resolver,
		nodelister:      kubeInformerFactory.Core().V1().Nodes().Lister(),
		factory:         factory,
	}

	glog.Info("Setting up event handlers")

	//  Add Function (OpenFaaS CRD-entry) Informer
	//
	// Set up an event handler for when Function resources change
	faasInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.addSHR,
		UpdateFunc: func(old, new interface{}) {
			controller.addSHR(new)
		},
		DeleteFunc: controller.handleDeletedSharePod,
	})

	//seems we do not need a Indexer here to find this Sharepod vary fast
	/*
		faasInformer.Informer().AddIndexers(cache.Indexers{

		})
	*/

	//podQueue := ocache.NewEventQueue(cache.MetaNamespaceKeyFunc)

	//podLW := &cache.wa

	//watch.EventType()

	// Set up an event handler for when functions related resources like pods, deployments, replica sets
	// can't be materialized. This logs abnormal events like ImagePullBackOff, back-off restarting failed container,
	// failed to start container, oci runtime errors, etc
	// Enable this with -v=3
	kubeInformerFactory.Core().V1().Events().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {

			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				event := obj.(*corev1.Event)
				//glog.Infof() event.FirstTimestamp
				since := time.Since(event.LastTimestamp.Time)
				// log abnormal events occurred in the last minute
				if since.Seconds() < 61 && strings.Contains(event.Type, "Warning") {
					glog.V(3).Infof("Abnormal event detected on %s %s: %s", event.LastTimestamp, key, event.Message)
				}
			}
			//controller.updateRunningPod()
		},
	})

	go controller.podWatch()
	//kube

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()

	//Start events processing pipeline
	//need to figure a way here, inconsistency with version
	//c.eventBroadcaster.StartStructuredLogging(0)
	//c.eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: c.kubeclient.CoreV1().Events("")})
	//defer c.eventBroadcaster.Shutdown()

	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.sharepodsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	// Launch two workers to process Function resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the workqueue.
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

	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		c.workqueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the SharePod resource with this namespace/name
	sharepod, err := c.sharepodsLister.SharePods(namespace).Get(name)
	if err != nil {
		// The Function resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("sharepod '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	needUpdate := false
	/*
		if sharepod.Spec.Selector == nil {
			sharepod.Spec.Selector = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":        sharepod.Name,
					"controller": sharepod.Name,
				}}
			needUpdate = true
		}

		//default have one replica
		if sharepod.Spec.Replicas == nil {
			sharepod.Spec.Replicas = getReplicas(sharepod)

			needUpdate = true
		}

	*/

	if needUpdate {
		_, err := c.faasclientset.KubeshareV1().SharePods(namespace).Update(context.TODO(), sharepod, metav1.UpdateOptions{})

		if err != nil {
			glog.Errorf("Error updating sharepods %v/%v replica and selector...", namespace, name)
		}
	}
	// Get the deployment with the name specified in Function.spec
	//deployment, err := c.deploymentsLister.Deployments(sharepod.GetNamespace()).Get(deploymentName)
	// If the resource doesn't exist, we'll create it

	svcGetOptions := metav1.GetOptions{}
	_, getSvcErr := c.kubeclient.CoreV1().Services(sharepod.Namespace).Get(context.TODO(), sharepod.Name, svcGetOptions)
	if errors.IsNotFound(getSvcErr) {
		glog.Infof("Creating ClusterIP service for '%s'", sharepod.Name)
		if _, err := c.kubeclient.CoreV1().Services(sharepod.Namespace).Create(context.TODO(), newService(sharepod), metav1.CreateOptions{}); err != nil {
			// If an error occurs during Service Create, we'll requeue the item
			if errors.IsAlreadyExists(err) {
				err = nil
				glog.V(2).Infof("ClusterIP service '%s' already exists. Skipping creation.", sharepod.Name)
			} else {
				return err
			}
		}
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return fmt.Errorf("transient error: %v", err)
	}

	existingService, err := c.kubeclient.CoreV1().Services(sharepod.Namespace).Get(context.TODO(), sharepod.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	existingService.Annotations = makeAnnotations(sharepod)
	_, err = c.kubeclient.CoreV1().Services(sharepod.Namespace).Update(context.TODO(), existingService, metav1.UpdateOptions{})
	if err != nil {
		glog.Errorf("Updating service for '%s' failed: %v", sharepod.Name, err)
	}

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. THis could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	c.recorder.Event(sharepod, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// enqueueFunction takes a Function resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Function.
func (c *Controller) addSHR(obj interface{}) {
	//shr := obj.(*faasv1.SharePod)

	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	//c.enqueueSHR(shr)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)

	if err != nil {
		//handle error
		runtime.HandleError(err)
	}
	shr, err := c.faasclientset.KubeshareV1().SharePods(namespace).Get(context.TODO(), name, metav1.GetOptions{})

	if err != nil {
		glog.Errorf("Error getting shr %v", name)

	}
	copy := shr.DeepCopy()

	//create the map for this sharepod/function
	c.resolver.AddFunc(copy.Name)

	nodeList, err := c.nodelister.List(labels.Everything())

	/*
		replica := getReplicas(copy)

		copy.Spec.Replicas = replica

		_, Err := c.faasclientset.KubeshareV1().SharePods(namespace).Update(context.TODO(), copy, metav1.UpdateOptions{})

		if Err != nil {
			runtime.HandleError(Err)
		}
	*/

	//job, err := c.kubeclient.BatchV1().Jobs(namespace).Create()
	if copy.Spec.PodSpec.InitContainers != nil {

		for _, node := range nodeList {
			_, err := c.kubeclient.BatchV1().Jobs(namespace).Create(context.TODO(), newJob(node.Name, copy), metav1.CreateOptions{})

			if err != nil {
				glog.Errorf("Error %v starting init container for sharepod %v/%v", err, namespace, name)
				runtime.HandleError(err)
			}
		}
		//_, err := c.kubeclient.CoreV1().Pods(namespace).Create(context.TODO(), newInitPod(copy), metav1.CreateOptions{})

	}

	c.workqueue.AddRateLimited(key)
}

func newJob(node string, shrCopy *faasv1.SharePod) *batchv1.Job {
	namePrefix := shrCopy.Name + "-init-"
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: namePrefix,
			Namespace:    shrCopy.Namespace,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: int32p(100),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					NodeName:      node,
					Containers:    shrCopy.Spec.PodSpec.InitContainers,
					Volumes:       shrCopy.Spec.PodSpec.Volumes,
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Function resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Function resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		glog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	glog.V(4).Infof("Processing object: %s", object.GetName())

	if pod, ok := obj.(*corev1.Pod); ok {
		if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
			if ownerRef.Kind != faasKind {
				return
			}
			if (pod.Status.Phase == corev1.PodRunning || *pod.Status.ContainerStatuses[0].Started) && pod.Annotations[devicemanager.FaasShareWarm] != "true" {
				//(*c.faasShareInfos)[ownerRef.Name]
				if pod.Status.PodIP != "" {
					c.resolver.Insert(ownerRef.Name, pod.Name, pod.Status.PodIP)
				}
			}
		}
	}

	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a function, we should not do anything more
		// with it.
		if ownerRef.Kind != faasKind {
			return
		}

		function, err := c.sharepodsLister.SharePods(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			glog.Infof("Sharepod '%s' deleted. Ignoring orphaned object '%s'", ownerRef.Name, object.GetSelfLink())
			return
		}

		c.addSHR(function)
		return
	}
}

/*
// getSecrets queries Kubernetes for a list of secrets by name in the given k8s namespace.
func (c *Controller) getSecrets(namespace string, secretNames []string) (map[string]*corev1.Secret, error) {
	secrets := map[string]*corev1.Secret{}

	for _, secretName := range secretNames {
		secret, err := c.kubeclientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
		if err != nil {
			return secrets, err
		}
		secrets[secretName] = secret
	}

	return secrets, nil
}
*/

func (c *Controller) handleDeletedSharePod(obj interface{}) {
	sharepod, ok := obj.(*faasv1.SharePod)

	if !ok {
		runtime.HandleError(fmt.Errorf("handleDeletedSharePod: cannot parse object"))
		return
	}

	//TODO: delete sharepod key

	//c.resolver

	namespace := sharepod.Namespace

	name := sharepod.Name

	//handle delete need?
	glog.Infof("Sharepod %v/%v deleted...", namespace, name)

	//go c.removeSharepodFromList(sharepod)
	//todo: keep these pod? or keep the vGPU
	//
	//c.kubeclientset.AppsV1().Deployments()
}

// getReplicas returns the desired number of replicas for a function taking into account
// the min replicas label, HPA, the OF autoscaler and scaled to zero deployments
func getReplicas(sharepod *faasv1.SharePod) *int32 {
	var minReplicas *int32

	// extract min replicas from label if specified
	if sharepod != nil && sharepod.Labels != nil {
		lb := sharepod.Labels
		if value, exists := lb[LabelMinReplicas]; exists {
			r, err := strconv.Atoi(value)
			if err == nil && r > 0 {
				minReplicas = int32p(int32(r))
			}
		}
	}

	// extract current deployment replicas if specified
	sharepodReplicas := sharepod.Spec.Replicas

	// do not set replicas if min replicas is not set
	// and current deployment has no replicas count
	if minReplicas == nil && sharepodReplicas == nil {
		return nil
	}

	// set replicas to min if deployment has no replicas and min replicas exists
	if minReplicas != nil && sharepodReplicas == nil {
		return minReplicas
	}

	// do not override replicas when deployment is scaled to zero
	if sharepodReplicas != nil && *sharepodReplicas == 0 {
		return sharepodReplicas
	}

	// do not override replicas when min is not specified
	if minReplicas == nil && sharepodReplicas != nil {
		return sharepodReplicas
	}

	// do not override HPA or OF autoscaler replicas if the value is greater than min
	if minReplicas != nil && sharepodReplicas != nil {
		if *sharepodReplicas >= *minReplicas {
			return sharepodReplicas
		}
	}
	return minReplicas
}

func newInitPod(gpu *faasv1.SharePod) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gpu.Name + "-init",
			Namespace: gpu.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: gpu.Spec.PodSpec.InitContainers,
			Volumes:    gpu.Spec.PodSpec.Volumes,
		},
	}
}

func (c *Controller) updateRunningPod(pod *corev1.Pod) {
	if len(pod.Status.PodIP) > 0 {
		if ownerRef := metav1.GetControllerOf(pod); ownerRef != nil {
			if ownerRef.Kind != faasKind {
				return
			}
			if pod.Status.Phase == corev1.PodRunning || *pod.Status.ContainerStatuses[0].Started {
				c.resolver.Insert(ownerRef.Name, pod.Name, pod.Status.PodIP)
			}
		}
	}
}

func (c *Controller) podWatch() error {
	podsWatcher, err := c.kubeclient.CoreV1().Pods("faas-share-fn").Watch(context.Background(), metav1.ListOptions{Watch: true})
	if err != nil {
		return err
	}
	podsChan := podsWatcher.ResultChan()
	for event := range podsChan {
		pod, err := event.Object.(*corev1.Pod)
		if err {
			return nil
		}

		switch event.Type {
		case watch.Added:
			c.updateRunningPod(pod)
		case watch.Modified:
			c.updateRunningPod(pod)
		case watch.Bookmark:
			c.updateRunningPod(pod)
			//todo case delete

		}
	}

	return nil
}

/*
func newInitJob(shr *faasv1.SharePod, name string) *batchv1.Job {
	//completions := (int32)1
	jobName := name + "-init"
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: shr.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(shr, schema.GroupVersionKind{
					Group:   faasv1.SchemeGroupVersion.Group,
					Version: faasv1.SchemeGroupVersion.Version,
					Kind:    "SharePod",
				}),
			},
		},
		Spec: batchv1.JobSpec{
			Completions: int32p(1),
			Template:    corev1.PodTemplateSpec{},
		},
	}
}
*/
