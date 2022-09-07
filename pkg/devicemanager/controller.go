package devicemanager

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	//v1 "k8s.io/api/core/v1"
	//"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/integer"

	// "k8s.io/apimachinery/pkg/labels"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	faasv1 "github.com/Interstellarss/faas-share/pkg/apis/faasshare/v1"
	//kubesharev1 "github.com/Interstellarss/faas-share/pkg/apis/faas_share/v1"

	//faasv1 "github.com/Interstellarss/faas-share/pkg/apis/faas_share/v1"

	clientset "github.com/Interstellarss/faas-share/pkg/client/clientset/versioned"
	kubesharescheme "github.com/Interstellarss/faas-share/pkg/client/clientset/versioned/scheme"
	informers "github.com/Interstellarss/faas-share/pkg/client/informers/externalversions/faasshare/v1"
	listers "github.com/Interstellarss/faas-share/pkg/client/listers/faasshare/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	//"k8s.io/kubernetes/pkg/controller"
	k8scontroller "k8s.io/kubernetes/pkg/controller"

	"github.com/containerd/containerd"
)

const controllerAgentName = "kubeshare-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a SharePod is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a SharePod fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	ErrValueError = "ErrValueError"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by SharePod"
	// MessageResourceSynced is the message used for an Event fired when a SharePod
	// is synced successfully
	MessageResourceSynced = "SharePod synced successfully"

	KubeShareLibraryPath = "/kubeshare/library"
	SchedulerIpPath      = KubeShareLibraryPath + "/schedulerIP.txt"
	PodManagerPortStart  = 50050

	KubeShareScheduleAffinity     = "kubeshare/sched_affinity"
	KubeShareScheduleAntiAffinity = "kubeshare/sched_anti-affinity"
	KubeShareScheduleExclusion    = "kubeshare/sched_exclusion"

	faasKind = "Sharepod"

	FaasShareWarm = "faas-share/warmpool"
)

var (
	KeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

type Controller struct {
	kubeclient kubernetes.Interface
	faasclient clientset.Interface

	//TODO: make new api into a new control interface as kubenetes did in controller
	//podControl k8scontroller.PodControlInterface

	podsLister corelisters.PodLister
	podsSynced cache.InformerSynced

	podInformer coreinformers.PodInformer

	sharepodsLister listers.SharePodLister
	sharepodsSynced cache.InformerSynced

	nodesLister corelisters.NodeLister
	nodeSynced  cache.InformerSynced

	expectations *k8scontroller.UIDTrackingControllerExpectations

	pendingList    *list.List
	pendingListMux *sync.Mutex

	//syncHandler func(ctx context.Context, shrKey string) error

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	//needed?
	//factory
	containerdClient *containerd.Client
}

// NewController returns a new sample controller
func NewController(
	kubeclient kubernetes.Interface,
	kubeshareclient clientset.Interface,
	nodeInformer coreinformers.NodeInformer,
	podInformer coreinformers.PodInformer,
//containerdClient *containerd.Client,
	kubeshareInformer informers.SharePodInformer) *Controller {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(kubesharescheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)

	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	//podcontrol := k8scontroller.RealPodControl{
	//	KubeClient: kubeclient,
	//	Recorder:   eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "faas-share-controller"}),
	//}

	controller := &Controller{
		faasclient: kubeshareclient,
		kubeclient: kubeclient,

		podInformer:     podInformer,
		podsLister:      podInformer.Lister(),
		podsSynced:      podInformer.Informer().HasSynced,
		sharepodsLister: kubeshareInformer.Lister(),
		sharepodsSynced: kubeshareInformer.Informer().HasSynced,

		nodesLister: nodeInformer.Lister(),
		nodeSynced:  nodeInformer.Informer().HasSynced,
		//deploymentLister:  deploymentInformer.Lister(),
		//depploymentSynced: deploymentInformer.Informer().HasSynced,
		expectations: k8scontroller.NewUIDTrackingControllerExpectations(k8scontroller.NewControllerExpectations()),

		pendingList:    list.New(),
		pendingListMux: &sync.Mutex{},
		//podControl:   podcontrol,
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "SharePods"),
		recorder:  recorder,
		//containerdClient: containerd.Client,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when SharePod resources change
	kubeshareInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueSharePod,
		UpdateFunc: func(old, new interface{}) {
			newSHR := new.(*faasv1.SharePod)
			oldSHR := old.(*faasv1.SharePod)
			klog.Infof("DEBUG: updating SHR %s with replica %i ", newSHR.Name, *newSHR.Spec.Replicas)
			klog.Infof("DEBUG: queue length %i", controller.workqueue.Len())
			if newSHR.ResourceVersion == oldSHR.ResourceVersion {
				controller.enqueueSharePod(new)
				return
			}

			controller.enqueueSharePod(new)
		},
		DeleteFunc: controller.handleDeletedSharePod,
	})

	// Set up an event handler for when Deployment resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a SharePod resource will enqueue that SharePod resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Deployment resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*corev1.Pod)
			oldDepl := old.(*corev1.Pod)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				controller.handleObject(new)
				return
			}
			controller.handleObject(new)
		},
		//TODO release pod when scale down?
		DeleteFunc: controller.handleObject,
	})

	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: controller.resourceChanged,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.

//Try out without stopCh, rather with context
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting SharePod controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.podsSynced, c.sharepodsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	//In our logic, we should have a vGPU pool running...Therefore, clean before create...
	c.cleanOrphanDummyPod()

	if err := c.initNodesInfo(); err != nil {
		return fmt.Errorf("failed to init NodeClient: %s", err)
	}
	//
	// clientHandler in ConfigManager must have correct PodList of every SharePods,
	// so call it after initNodeClient

	pendingInsuranceTicker := time.NewTicker(5 * time.Second)
	pendingInsuranceDone := make(chan bool)
	go c.pendingInsurance(pendingInsuranceTicker, &pendingInsuranceDone)

	go StartConfigManager(stopCh, c.kubeclient)

	klog.Info("Starting workers")
	// Launch two workers to process SharePod resources
	for i := 0; i < threadiness; i++ {
		//
		//go wait.UntilWithContext(ctx, c.runWorker, time.Second)
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")
	pendingInsuranceTicker.Stop()
	pendingInsuranceDone <- true

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

//TODO
/*
func (c *Controller) getSharePods(pod *corev1.Pod) []*faasv1.SharePod {
	shrpods, err := c.sharepodsLister.List()
}
*/

//TODO: get sharepod to this pod
/*
func (c *Controller) getPodSharePod(pod *v1.Pod) []*faasv1.SharePod {
	//shr, err := c.sharepodsLister.Get
}
*/

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
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// SharePod resource to be synced.
		if err := c.syncHandler(key); err != nil {
			if err.Error() == "Waiting4Dummy" {
				c.workqueue.Add(key)
				return fmt.Errorf("TESTING: need to wait for dummy pod '#{key}', requeueing")
			}

			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler returns error when we want to re-process the key, otherwise returns nil
func (c *Controller) syncHandler(key string) error {

	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing SharePod %q (%v)", key, time.Since(startTime))
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	shr, err := c.sharepodsLister.SharePods(namespace).Get(name)

	if err != nil {
		if apierrors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("SharePod '%s' in work queue no longer exists", key))
			c.expectations.DeleteExpectations(key)
			return nil
		}
		return err
	}

	if shr.Spec.Replicas == nil {
		klog.Infof("Waiting sharepod %v/%vreplicas to be updated...", namespace, name)
		return nil
	}

	shrCopy := shr.DeepCopy()

	if shrCopy.Spec.Selector == nil {
		shrCopy.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app":        shrCopy.Name,
				"controller": shrCopy.Name,
			}}
	}

	if shrCopy.Status.BoundDeviceIDs == nil {
		boundIds := make(map[string]string)
		shrCopy.Status.BoundDeviceIDs = &boundIds
	}

	if shrCopy.Status.Pod2Node == nil {
		pod2Node := make(map[string]string)
		shrCopy.Status.Pod2Node = &pod2Node
	}

	if shrCopy.Status.PodManagerPort == nil {
		podmanagerPort := make(map[string]int)
		shrCopy.Status.PodManagerPort = &podmanagerPort
	}

	/*
		if shrCopy.Status.PrewarmPool == nil {
			pool := make([]*corev1.Pod, *shrCopy.Spec.Replicas)
			shrCopy.Status.PrewarmPool = pool
		}
	*/

	if shrCopy.Status.Usage == nil {
		usages := make(map[string]faasv1.SharepodUsage)
		shrCopy.Status.Usage = &usages
	}

	shrNeedsSync := c.expectations.SatisfiedExpectations(key)

	selector, err := metav1.LabelSelectorAsSelector(shrCopy.Spec.Selector)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error converting pod selector to selector for shr %v/%v: %v", namespace, name, err))
	}

	if selector == nil {
		klog.Infof("selector is till nil...")
		return nil
	}

	//sharepod, err := c.sharepodsLister.SharePods(namespace).Get(name)

	allPods, err := c.podsLister.Pods(namespace).List(selector)

	if err != nil {
		return err
	}

	//Ignore inavtive pods
	filteredPods := FilterActivePods(allPods)

	//filterPodss

	klog.Infof("Pod of SharePod %v/%v with length %d", shrCopy.Namespace, shrCopy.Name, len(filteredPods))

	var manageReplicasErr error
	if shrNeedsSync && shr.DeletionTimestamp == nil {
		manageReplicasErr = c.manageReplicas(context.TODO(), filteredPods, shrCopy, key)
	}

	//shrCopy = shrCopy.DeepCopy()
	newStatus := calculateStatus(shrCopy, filteredPods, manageReplicasErr)

	shrCopy.Status.AvailableReplicas = newStatus.AvailableReplicas
	shrCopy.Status.ReadyReplicas = newStatus.ReadyReplicas
	shrCopy.Status.Replicas = newStatus.Replicas

	//erro := c.updateSharePodStatus(shrCopy, )

	//TODO: new update methodology?  //Will this trigger update callback?//maybe too often?
	updatedSHR, err := c.faasclient.KubeshareV1().SharePods(shrCopy.Namespace).Update(context.TODO(), shrCopy, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	//mini ready seconds also need set?
	// Resync the ReplicaSet after MinReadySeconds as a last line of defense to guard against clock-skew.
	if manageReplicasErr != nil && updatedSHR.Status.ReadyReplicas == *(updatedSHR.Spec.Replicas) &&
		updatedSHR.Status.AvailableReplicas != *(updatedSHR.Spec.Replicas) {
		//c.enqueueSHRAfter(updatedSHR, time.Duration(500)*time.Millisecond)
		c.enqueueSharePod(updatedSHR)
	}

	//c.recorder.Event(sharepod, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return manageReplicasErr
}

//Need to configure here
func (c *Controller) updateSharePodStatus(sharepod *faasv1.SharePod, pod *corev1.Pod, port int) error {
	sharepodCopy := sharepod.DeepCopy()
	//sharepodCopy.Status.PodStatus = pod.Status.DeepCopy()
	//sharepodCopy.Status.PodObjectMeta = pod.ObjectMeta.DeepCopy()
	if port != 0 {
		(*sharepodCopy.Status.PodManagerPort)[pod.Name] = port
	}

	_, err := c.faasclient.KubeshareV1().SharePods(sharepodCopy.Namespace).Update(context.TODO(), sharepodCopy, metav1.UpdateOptions{})
	return err
}

func (c *Controller) enqueueSharePod(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

func (c *Controller) enqueueSHRAfter(shr *faasv1.SharePod, duration time.Duration) {
	key, err := KeyFunc(shr)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("could\t get key from object %#v : %v", shr, err))
		return
	}

	c.workqueue.AddAfter(key, duration)
}

func (c *Controller) handleDeletedSharePod(obj interface{}) {
	sharepod, ok := obj.(*faasv1.SharePod)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("handleDeletedSharePod: cannot parse object"))
		return
	}
	klog.Infof("Starting to delete pod of sharepod %s/%s", sharepod.Namespace, sharepod.Name)
	//TODO: delete sharepod
	go c.removeSharePodFromList(sharepod)
}

func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.V(4).Infof("Processing object: %s", object.GetName())

	// get physical GPU UUID from dummy Pod
	// get UUID here to prevent request throttling
	if pod, ok := obj.(*corev1.Pod); ok {

		if pod.ObjectMeta.Namespace == "kube-system" && strings.Contains(pod.ObjectMeta.Name, faasv1.KubeShareDummyPodName) && (pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodFailed) {
			// TODO: change the method of getting GPUID from label to more reliable source
			// e.g. Pod name (kubeshare-dummypod-{NodeName}-{GPUID})

			klog.Infof("A Dummy pod %s is being proceed...", pod.Name)
			if pod.Spec.NodeName != "" {
				if gpuid, ok := pod.ObjectMeta.Labels[faasv1.KubeShareResourceGPUID]; ok {
					needSetUUID := false
					nodesInfoMux.Lock()
					// klog.Infof("ERICYEH1: %#v", nodeClients[pod.Spec.NodeName])
					if node, ok := nodesInfo[pod.Spec.NodeName]; ok {
						if _, ok := node.GPUID2GPU[gpuid]; ok && node.GPUID2GPU[gpuid].UUID == "" {
							needSetUUID = true
						}
					}
					nodesInfoMux.Unlock()
					if needSetUUID {
						klog.Infof("Start go routine to get UUID from dummy Pod")
						go c.getAndSetUUIDFromDummyPod(pod.Spec.NodeName, gpuid, pod.ObjectMeta.Name, pod)
					}
				} else {
					klog.Errorf("Detect empty %s label from dummy Pod: %s", faasv1.KubeShareResourceGPUID, pod.ObjectMeta.Name)
				}
			} else {
				klog.Errorf("Detect empty NodeName from dummy Pod: %s", pod.ObjectMeta.Name)
			}
		}
	}

	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a SharePod, we should not do anything more
		// with it.
		if ownerRef.Kind != "SharePod" {
			return
		}

		foo, err := c.sharepodsLister.SharePods(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.V(4).Infof("ignoring orphaned object '%s' of SharePod '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		klog.Infof("A pod %s is being proceed...", foo.Name)

		c.enqueueSharePod(foo)
		return
	}
}

func (c *Controller) manageReplicas(ctx context.Context, filteredPods []*corev1.Pod, gpupod *faasv1.SharePod, key string) error {
	podNamePoolMux.Lock()
	defer podNamePoolMux.Unlock()

	diff := len(filteredPods) - int(*(gpupod.Spec.Replicas))

	klog.Infof("SHR %s with current replica %s", gpupod.Name, diff)
	shrKey, err := KeyFunc(gpupod)

	shrCopy := gpupod.DeepCopy()

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for %v %#v: %v", gpupod.Kind, gpupod, err))
		return nil
	}

	if diff < 0 {
		diff *= -1

		//may also set for burstresplicas?
		/*
			if gpupod.Status.PrewarmPool == nil {
				pool := make([]*corev1.Pod, *shrCopy.Spec.Replicas)
				gpupod.Status.PrewarmPool = pool
			} else {

				warmSize := len(gpupod.Status.PrewarmPool)

				podlist := gpupod.Status.PrewarmPool

				klog.Infof("Prewarm pool size %d...", warmSize)
				if warmSize != 1 || podlist[0] != nil {
					//update in the delete way go func with
					if diff <= warmSize {
						//TODO: when tobe created is smaller or equal to the number of pods in the pre-warm pool

						//podlist[0].Status.
						for m := 0; m < diff; m++ {
							pod := podlist[m]

							//pod.Annotations[FaasShareWarm] = "false"
							if pod.Annotations[FaasShareWarm] != "" {
								pod.Annotations[FaasShareWarm] = "false"
							}
							//gpupod.Status.PrewarmPool[m] = nil

							//delete(gpupod.Status.PrewarmPool, pod.Name)

							//update new scheduled node and

							_, err := c.kubeclient.CoreV1().Pods(pod.Namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})

							if err != nil {

							}

							//c
							//patch or updates?
						}
						gpupod.Status.PrewarmPool = make([]*corev1.Pod, 3)

						return err

					} else {
						for m, pod := range podlist {
							pod.Annotations[FaasShareWarm] = "false"
							gpupod.Status.PrewarmPool[m] = nil

							_, err := c.kubeclient.CoreV1().Pods(pod.Namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})

							if err != nil {

							}
						}

						diff -= warmSize
					}
				}
			}

		*/
		c.expectations.ExpectCreations(shrKey, diff)

		klog.V(2).Infof("Too few replicas for this SharePod...\n need %d replicas, creating %d", shrCopy.Spec.Replicas, diff)

		// Batch the pod creates. Batch sizes start at SlowStartInitialBatchSize
		// and double with each successful iteration in a kind of "slow start".
		// This handles attempts to start large numbers of pods that would
		// likely all fail with the same error. For example a project with a
		// low quota that attempts to create a large number of pods will be
		// prevented from spamming the API service with the pod create requests
		// after one of its pods fails.  Conveniently, this also prevents the
		// event spam that those failures would generate.
		successfulCreations, err := slowStartbatch(diff, k8scontroller.SlowStartInitialBatchSize, func() (*corev1.Pod, error) {
			//if gpupod.Status.PrewarmPool.

			isGPUPod := false
			gpu_request := 0.0
			gpu_limit := 0.0
			gpu_mem := int64(0)

			physicalGPUuuid := ""
			physicalGPUport := 0

			if gpupod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPURequest] != "" ||
				gpupod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPULimit] != "" ||
				gpupod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPUMemory] != "" {
				var err error
				gpu_limit, err = strconv.ParseFloat(gpupod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPULimit], 64)
				if err != nil || gpu_limit > 1.0 || gpu_limit < 0.0 {
					utilruntime.HandleError(fmt.Errorf("Pod of SharePod %s/%s value error: %s", gpupod.ObjectMeta.Namespace, gpupod.ObjectMeta.Name, faasv1.KubeShareResourceGPULimit))
					return nil, err
				}
				gpu_request, err = strconv.ParseFloat(gpupod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPURequest], 64)
				if err != nil || gpu_request > gpu_limit || gpu_request < 0.0 {
					utilruntime.HandleError(fmt.Errorf("Pod of SharePod %s/%s value error: %s", gpupod.ObjectMeta.Namespace, gpupod.ObjectMeta.Name, faasv1.KubeShareResourceGPURequest))

				}
				gpu_mem, err = strconv.ParseInt(gpupod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPUMemory], 10, 64)
				if err != nil || gpu_mem < 0 {
					utilruntime.HandleError(fmt.Errorf("Pod of SharePod %s/%s value error: %s", gpupod.ObjectMeta.Namespace, gpupod.ObjectMeta.Name, faasv1.KubeShareResourceGPUMemory))

				}
				isGPUPod = true
			}
			var schedNode, schedGPUID string

			schedNode, schedGPUID = c.schedule(gpupod, gpu_request, gpu_limit, gpu_mem, isGPUPod, key)

			/*
				if len(gpupod.Status.Node2Id) == 0 {
					schedNode, schedGPUID = c.schedule(gpupod, gpu_request, gpu_limit, gpu_mem, isGPUPod, key)

					gpupod.Status.Node2Id = append(gpupod.Status.Node2Id, faasv1.Scheded{Node: schedNode, GPU: schedGPUID})
				} else {
					schedNode = gpupod.Status.Node2Id[len(gpupod.Status.Node2Id)-1].Node
					schedGPUID = gpupod.Status.Node2Id[len(gpupod.Status.Node2Id)-1].GPU
				}
			*/

			//TODO:
			//if gpupod.Status.

			if schedNode == "" {
				//klog.Infof("No enough resources for SharePod: %s/%s", gpupod.ObjectMeta.Namespace, gpupod.ObjectMeta.Name)
				// return fmt.Errorf("No enough resources for SharePod: %s/%s")
				//c.pendingListMux.Lock()
				//c.pendingList.PushBack(key)
				//c.pendingListMux.Unlock()
				return nil, errors.New("NoSchedNode")
			}

			klog.Infof("SharePod '%s' had been scheduled to node '%s' GPUID '%s'.", key, schedNode, schedGPUID)

			//error management check if node is nil

			//boundDeviceId :=
			var subpodName string
			var podKey string
			if isGPUPod {
				var errCode int
				//podNamePoolMux.Lock()
				//defer podNamePoolMux.Unlock()
				if podNamePool[shrCopy.Name] != nil {
					if podNamePool[shrCopy.Name].Len() == 0 {
						newpodName := shrCopy.Name + "-" + RandStr(5)
						podNamePool[shrCopy.Name].PushBack(newpodName)
						subpodName = newpodName
					} else {
						subpodName = podNamePool[shrCopy.Name].Back().Value.(string)
					}
				} else {
					podNamePool[shrCopy.Name] = list.New()
					newpodName := shrCopy.Name + "-" + RandStr(5)
					podNamePool[shrCopy.Name].PushBack(newpodName)
					subpodName = newpodName
				}
				podKey = fmt.Sprintf("%s/%s", shrCopy.ObjectMeta.Namespace, subpodName)

				physicalGPUuuid, errCode = c.getPhysicalGPUuuid(schedNode, schedGPUID, gpu_request, gpu_limit, gpu_mem, podKey, &physicalGPUport)
				klog.Infof("Pod of Sharepod %v/%v with vGPU port: %d", shrCopy.Namespace, shrCopy.Name, physicalGPUport)
				switch errCode {
				case 0:
					klog.Infof("SharePod %s is bound to GPU uuid: %s", key, physicalGPUuuid)
					//delete the last element
					podNamePool[shrCopy.Name].Remove(podNamePool[shrCopy.Name].Back())
					//podNamePoolMux.Unlock()
				case 1:
					klog.Infof("SharePod %s/%s is waiting for dummy Pod", gpupod.ObjectMeta.Namespace, gpupod.ObjectMeta.Name)
					//
					//_, err := c.faasclient.KubeshareV1().SharePods(gpupod.Namespace).Update(context.TODO(), gpupod, metav1.UpdateOptions{})
					if err != nil {
						utilruntime.HandleError(err)
					}
					return nil, errors.New("Wait4Dummy")
				case 2:
					err := fmt.Errorf("Resource exceed!")
					utilruntime.HandleError(err)
					c.recorder.Event(gpupod, corev1.EventTypeWarning, ErrValueError, "Resource exceed")
					return nil, err
				case 3:
					err := fmt.Errorf("Pod manager port pool is full!")
					utilruntime.HandleError(err)
					return nil, err
				default:
					utilruntime.HandleError(fmt.Errorf("Unknown Error"))
					c.recorder.Event(gpupod, corev1.EventTypeWarning, ErrValueError, "Unknown Error")
					return nil, nil
				}
				//gpupod.Status.BoundDeviceID = physicalGPUuuid
			}

			if n, ok := nodesInfo[schedNode]; ok {
				klog.Infof("TESTING: Starting to create new pod of sharepod %v/%v in batch", gpupod.Namespace, gpupod.Name)
				newPod, err := c.kubeclient.CoreV1().Pods(shrCopy.Namespace).Create(context.TODO(), newPod(gpupod, false, n.PodIP, physicalGPUport, physicalGPUuuid, schedNode, schedGPUID, subpodName), metav1.CreateOptions{})
				if err != nil {
					klog.Errorf("error creating pod of Sharepod %v/%v", gpupod.Namespace, gpupod.Name)
					if apierrors.HasStatusCause(err, corev1.NamespaceTerminatingCause) {
						return nil, nil
					}
					//podNamePoolMux.Unlock()
					return nil, err //
				}
				//remove the last element
				//gpupod.Status.Node2Id = gpupod.Status.Node2Id[:len(gpupod.Status.Node2Id)-1]
				//(*gpupod.Status.Pod2Node)[newPod.Name] = schedNode

				//should be mapping from pod to physical devciceID, use vGPU id for simplicity
				(*gpupod.Status.BoundDeviceIDs)[newPod.Name] = schedGPUID
				//(*gpupod.Status.Usage)[schedGPUID] = faasv1.SharepodUsage{GPU: gpu_request, TotalMemoryBytes: float64(gpu_mem) / 8}

				(*shrCopy.Status.PodManagerPort)[newPod.Name] = physicalGPUport

				//podNamePoolMux.Unlock()
				return newPod, err
			}
			return nil, nil
		})

		// Any skipped pods that we never attempted to start shouldn't be expected.
		// The skipped pods will be retried later. The next controller resync will
		// retry the slow start process.
		if skippedPods := diff - successfulCreations; skippedPods > 0 {
			klog.V(2).Infof("Slow-start failure. Skipping creation of %d pods, decrementing expectations for %v %v/%v", skippedPods, gpupod.Kind, gpupod.Namespace, gpupod.Name)
			for i := 0; i < skippedPods; i++ {

				//Decrement the expected number of creates because the informer won't observe this pod
				c.expectations.CreationObserved(shrKey)
			}
		}
		return err

	} else if diff > 0 {
		klog.V(2).InfoS("Too many replicas", "SharePod", klog.KObj(gpupod), "need", gpupod.Spec.Replicas, "deleting", diff)

		//indirect pods are in our case simply dummy pod that we can ignore
		//relatedPods, err := getIndirectly

		//TODO
		podsToDelete := getPodsToDelete(filteredPods, diff)

		//Comment: delete logic for container pool
		//podsToDelete[i] = podsToDelete[diff - 1]

		//warmpod := podsToDelete[diff-1]

		//err := c.updateWarmpod(warmpod, "TRUE")

		//if err != nil {
		//	return err
		//}

		//podsToDelete = podsToDelete[:diff-1]

		//diff--

		//podInPools := podsToDelete[len(podsToDelete)-1].DeepCopy()

		//podInPools.Status.Phase = corev1.PodUnknown

		//containerId := podInPools.Status.ContainerStatuses[0].ContainerID

		//klog.Infof("ContainerID for the warm container: %s", containerId)

		//con, err := c.containerdClient.LoadContainer(ctx.Done(), containerId)

		//con.

		c.expectations.ExpectDeletions(key, getPodKeys(podsToDelete))

		errCh := make(chan error, diff)

		var wg sync.WaitGroup
		for _, pod := range podsToDelete {
			go func(targetPod *corev1.Pod) {
				//c.kubeclient.CoreV1().Pods(pod.Namespace).de

				defer wg.Done()
				//c.clientset
				if err := c.kubeclient.CoreV1().Pods(targetPod.Namespace).Delete(ctx, targetPod.Name, metav1.DeleteOptions{}); err != nil {
					// Decrement the expected number of deletes because the informer won't observe this deletion
					podKey := k8scontroller.PodKey(targetPod)
					c.expectations.DeletionObserved(key, podKey)
					if !apierrors.IsNotFound(err) {
						klog.V(2).Infof("Failed to delete %v, decremented expectations for %v %s/%s", podKey, shrCopy.Kind, shrCopy.Namespace, shrCopy.Name)
						errCh <- err
					}

				}
			}(pod)
		}
		wg.Wait()

		select {
		case err := <-errCh:
			// all errors have been reported before and they're likely to be the same, so we'll only return the first one we hit.
			if err != nil {
				return err
			}
		default:
		}

	}
	return nil
}

// slowStartBatch tries to call the provided function a total of 'count' times,
// starting slow to check for errors, then speeding up if calls succeed.
//
// It groups the calls into batches, starting with a group of initialBatchSize.
// Within each batch, it may call the function multiple times concurrently.
//
// If a whole batch succeeds, the next batch may get exponentially larger.
// If there are any failures in a batch, all remaining batches are skipped
// after waiting for the current batch to complete.
//
// It returns the number of successful calls to the function.
func slowStartbatch(count int, initailBatchSize int, fn func() (*corev1.Pod, error)) (int, error) {
	remaining := count
	successes := 0
	need2wait := 0
	for batchSize := integer.IntMin(remaining, initailBatchSize); batchSize > 0; batchSize = integer.IntMin(2*batchSize, remaining) {
		errCh := make(chan error, batchSize)
		var wg sync.WaitGroup
		wg.Add(batchSize)
		for i := 0; i < batchSize; i++ {
			func() {
				defer wg.Done()
				if pod, err := fn(); err != nil {
					if err.Error() != "Wait4Dummy" {
						//continue
						errCh <- err
					}

					if pod == nil && err.Error() == "Wait4Dummy" {
						need2wait++
					}

				}
			}()
		}

		wg.Wait()
		curSuccesses := batchSize - len(errCh) - need2wait
		successes += curSuccesses
		if len(errCh) > 0 {
			return successes, <-errCh
		}
		remaining -= batchSize
	}

	if need2wait > 0 {
		return successes, errors.New("Wait4Dmmy")
	}
	return successes, nil
}

// newDeployment creates a new Deployment for a SharePod resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the SharePod resource that 'owns' it.
func newPod(shrpod *faasv1.SharePod, isWarm bool, podManagerIP string, podManagerPort int, boundDeviceId string, scheNode string, scheGPUID string, podName string) *corev1.Pod {
	specCopy := shrpod.Spec.PodSpec.DeepCopy()

	//scheNode, scheGPUID := c.schedule(shrpod)

	specCopy.NodeName = scheNode

	labelCopy := makeLabels(shrpod)
	annotationCopy := make(map[string]string, len(shrpod.ObjectMeta.Annotations)+5)
	for key, val := range shrpod.ObjectMeta.Annotations {
		annotationCopy[key] = val
	}

	//annotationCopy = append(annotationCopy,)
	if isWarm {
		annotationCopy[FaasShareWarm] = "true"
	} else {
		annotationCopy[FaasShareWarm] = "false"
	}

	//annotationCopy[faasv1.KubeShareResourceGPUID]

	// specCopy.Containers = append(specCopy.Containers, corev1.Container{
	// 	Name:    "podmanager",
	// 	Image:   "ncy9371/debian:stretch-slim-wget",
	// 	Command: []string{"sh", "-c", "wget -qO /pod_manager 140.114.78.229/web/pod_manager && chmod +x /pod_manager && SCHEDULER_IP=$(cat " + SchedulerIpPath + ") /pod_manager"},
	// })
	for i := range specCopy.Containers {
		c := &specCopy.Containers[i]
		c.Env = append(c.Env,
			corev1.EnvVar{
				Name:  "NVIDIA_VISIBLE_DEVICES",
				Value: boundDeviceId,
			},
			corev1.EnvVar{
				Name:  "NVIDIA_DRIVER_CAPABILITIES",
				Value: "compute,utility",
			},
			corev1.EnvVar{
				Name:  "LD_PRELOAD",
				Value: KubeShareLibraryPath + "/libgemhook.so.1",
			},
			corev1.EnvVar{
				Name:  "POD_MANAGER_IP",
				Value: podManagerIP,
			},
			corev1.EnvVar{
				Name:  "POD_MANAGER_PORT",
				Value: fmt.Sprintf("%d", podManagerPort),
			},
			corev1.EnvVar{
				Name:  "POD_NAME",
				Value: fmt.Sprintf("%s/%s", shrpod.ObjectMeta.Namespace, podName),
			},
		)
		c.VolumeMounts = append(c.VolumeMounts,
			corev1.VolumeMount{
				Name:      "kubeshare-lib",
				MountPath: KubeShareLibraryPath,
			},
		)
	}
	specCopy.Volumes = append(specCopy.Volumes,
		corev1.Volume{
			Name: "kubeshare-lib",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: KubeShareLibraryPath,
				},
			},
		},
	)
	annotationCopy[faasv1.KubeShareResourceGPURequest] = shrpod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPURequest]
	annotationCopy[faasv1.KubeShareResourceGPULimit] = shrpod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPULimit]
	annotationCopy[faasv1.KubeShareResourceGPUMemory] = shrpod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPUMemory]
	annotationCopy[faasv1.KubeShareResourceGPUID] = scheGPUID

	//ownerRef := shrpod.ObjectMeta.DeepCopy().OwnerReferences

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			//TODO: here
			//GenerateName: shrpod.Name + "-",
			Name:      podName,
			Namespace: shrpod.ObjectMeta.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(shrpod, schema.GroupVersionKind{
					Group:   faasv1.SchemeGroupVersion.Group,
					Version: faasv1.SchemeGroupVersion.Version,
					Kind:    faasKind,
				}),
			},
			Annotations: annotationCopy,
			Labels:      labelCopy,
		},
		Spec: corev1.PodSpec{
			NodeName:   scheNode,
			Containers: specCopy.Containers,
			Volumes:    specCopy.Volumes,
			//InitContainers: []corev1.Container{},
		},
	}
}

func (c *Controller) schedule(gpupod *faasv1.SharePod, gpu_request float64, gpu_limit float64, gpu_mem int64, isGPUPod bool, key string) (string, string) {

	nodeList, err := c.nodesLister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(fmt.Errorf(""))
	}
	podList, err := c.podsLister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(fmt.Errorf(""))
	}
	sharePodList, err := c.sharepodsLister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(fmt.Errorf(""))
	}
	schedNode, schedGPUID := scheduleSharePod(isGPUPod, gpu_request, gpu_mem, gpupod, nodeList, podList, sharePodList)
	if schedNode == "" {
		klog.Infof("No enough resources for Pod of a SharePod: %s/%s", gpupod.ObjectMeta.Namespace, gpupod.ObjectMeta.Name)
		// return fmt.Errorf("No enough resources for SharePod: %s/%s")
		c.pendingListMux.Lock()
		c.pendingList.PushBack(key)
		c.pendingListMux.Unlock()
		return "", ""
	}

	return schedNode, schedGPUID
}

func getPodKeys(pods []*corev1.Pod) []string {
	podKeys := make([]string, 0, len(pods))
	for _, pod := range pods {
		podKeys = append(podKeys, PodKey(pod))
	}
	return podKeys
}

func (c *Controller) updateWarmpod(pod *corev1.Pod, warm string) error {
	pod.ObjectMeta.Annotations[FaasShareWarm] = warm
	//TODO: change to patch?
	_, err := c.kubeclient.CoreV1().Pods(pod.Namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})

	return err
}

func (c *Controller) pendingInsurance(ticker *time.Ticker, done *chan bool) {
	for {
		select {
		case <-(*done):
			return
		case <-ticker.C:
			c.resourceChanged(nil)
		}
	}
}

func (c *Controller) resourceChanged(obj interface{}) {
	// push pending SharePods into workqueue
	c.pendingListMux.Lock()
	for p := c.pendingList.Front(); p != nil; p = p.Next() {
		c.workqueue.Add(p.Value)
	}
	c.pendingList.Init()
	c.pendingListMux.Unlock()
}

func (c *Controller) stopContainer(ID string, pod *corev1.Pod) {
	//container, err := c.containerdClient.LoadContainer(context.Background(), ID)
	//if err != nil {
	//}

	//stopTask, err := container.NewTask(context.TODO(), cio.NewCreator(cio.WithStdio))

	//TODO:kill a running container
	//stopTask.Kill(context.TODO(), syscall.DLT_ATM_CLIP)

}
