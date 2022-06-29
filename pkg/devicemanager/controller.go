package devicemanager

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	appsinformers "k8s.io/client-go/informers/apps/v1"

	faasv1 "github.com/Interstellarss/faas-share/pkg/apis/faas_share/v1"
	kubesharev1 "github.com/Interstellarss/faas-share/pkg/apis/faas_share/v1"

	//faasv1 "github.com/Interstellarss/faas-share/pkg/apis/faas_share/v1"

	clientset "github.com/Interstellarss/faas-share/pkg/client/clientset/versioned"
	kubesharescheme "github.com/Interstellarss/faas-share/pkg/client/clientset/versioned/scheme"
	informers "github.com/Interstellarss/faas-share/pkg/client/informers/externalversions/faas_share/v1"
	listers "github.com/Interstellarss/faas-share/pkg/client/listers/faas_share/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8scontroller "k8s.io/kubernetes/pkg/controller"
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
)

var (
	KeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

type Controller struct {
	kubeclient kubernetes.Interface
	faasclient clientset.Interface

	podControl k8scontroller.PodControlInterface

	podsLister corelisters.PodLister
	podsSynced cache.InformerSynced

	podInformer coreinformers.PodInformer

	sharepodsLister listers.SharePodLister
	sharepodsSynced cache.InformerSynced

	expectations *k8scontroller.UIDTrackingControllerExpectations

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
}

// NewController returns a new sample controller
func NewController(
	kubeclient kubernetes.Interface,
	kubeshareclient clientset.Interface,
	podInformer coreinformers.PodInformer,
	deploymentInformer appsinformers.DeploymentInformer,
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

	podcontrol := k8scontroller.RealPodControl{
		KubeClient: kubeclient,
		Recorder:   eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "faas-share-controller"}),
	}

	controller := &Controller{
		kubeclient:      kubeclient,
		faasclient:      kubeshareclient,
		podsLister:      podInformer.Lister(),
		podsSynced:      podInformer.Informer().HasSynced,
		sharepodsLister: kubeshareInformer.Lister(),
		sharepodsSynced: kubeshareInformer.Informer().HasSynced,
		//deploymentLister:  deploymentInformer.Lister(),
		//depploymentSynced: deploymentInformer.Informer().HasSynced,
		expectations: k8scontroller.NewUIDTrackingControllerExpectations(k8scontroller.NewControllerExpectations()),
		podControl:   podcontrol,
		workqueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "SharePods"),
		recorder:     recorder,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when SharePod resources change
	kubeshareInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueSharePod,
		UpdateFunc: func(old, new interface{}) {
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

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
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

	if err := c.initNodesInfo(); err != nil {
		return fmt.Errorf("failed to init NodeClient: %s", err)
	}
	c.cleanOrphanDummyPod()
	// clientHandler in ConfigManager must have correct PodList of every SharePods,
	// so call it after initNodeClient
	go StartConfigManager(stopCh, c.kubeclient)

	klog.Info("Starting workers")
	// Launch two workers to process SharePod resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

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

type patchValue struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

// syncHandler returns error when we want to re-process the key, otherwise returns nil
func (c *Controller) syncHandler(ctx context.Context, key string) error {

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
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("SharePod '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	//sharepod, err := c.sharepodsLister.SharePods(namespace).Get(name)

	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("SharePod '%s' in not found", key))
			return nil
		}
		return err
	}

	selector, err := metav1.LabelSelectorAsSelector(dep.Spec.Selector)
	if err != nil {
		return err
	}

	//c.recorder.Event(pods, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)

	// sharepod.Print()
	/*
			pod, err := c.podsLister.Pods(sharepod.ObjectMeta.Namespace).Get(sharepod.ObjectMeta.Name)

			// If the resource doesn't exist, we'll create it, but don't create when we knew that Pod will not restart forever
			if errors.IsNotFound(err) && (sharepod.Status.PodStatus == nil ||
				sharepod.Spec.RestartPolicy == corev1.RestartPolicyAlways ||
				(sharepod.Spec.RestartPolicy == corev1.RestartPolicyOnFailure && sharepod.Status.PodStatus.Phase != corev1.PodSucceeded) ||
				(sharepod.Spec.RestartPolicy == corev1.RestartPolicyNever && (sharepod.Status.PodStatus.Phase != corev1.PodSucceeded && sharepod.Status.PodStatus.Phase != corev1.PodFailed))) {
				if n, ok := nodesInfo[sharepod.Spec.NodeName]; ok {
					pod, err = c.kubeclientset.CoreV1().Pods(sharepod.ObjectMeta.Namespace).Create(context.TODO(), newPod(sharepod, isGPUPod, n.PodIP, physicalGPUport), metav1.CreateOptions{})
				}
			}

			if err != nil {
				return err
			}


		if !metav1.IsControlledBy(pod, sharepod) {
			msg := fmt.Sprintf(MessageResourceExists, pod.Name)
			c.recorder.Event(sharepod, corev1.EventTypeWarning, ErrResourceExists, msg)
			return fmt.Errorf(msg)
		}

		err = c.updateSharePodStatus(sharepod, pod, physicalGPUport)
		if err != nil {
			return err
		}
	*/

	//c.recorder.Event(sharepod, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

//Need to configure here
func (c *Controller) updateSharePodStatus(sharepod *kubesharev1.SharePod, pod *corev1.Pod, port int) error {
	sharepodCopy := sharepod.DeepCopy()
	sharepodCopy.Status.PodStatus = pod.Status.DeepCopy()
	//sharepodCopy.Status.PodObjectMeta = pod.ObjectMeta.DeepCopy()
	if port != 0 {
		sharepodCopy.Status.PodManagerPort = port
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
	sharepodDep, ok := obj.(*appsv1.Deployment)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("handleDeletedSharePodDep: cannot parse object"))
		return
	}
	go c.removeSharePodFromList(sharepodDep)
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

		klog.Infof("A pod %s is being proceed...", pod.Name)

		if pod.ObjectMeta.Namespace == "kube-system" && strings.Contains(pod.ObjectMeta.Name, faasv1.KubeShareDummyPodName) && (pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodFailed) {
			// TODO: change the method of getting GPUID from label to more reliable source
			// e.g. Pod name (kubeshare-dummypod-{NodeName}-{GPUID})
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

		c.enqueueSharePod(foo)
		return
	}
}

func (c *Controller) manageReplicas(ctx context.Context, filteredPods []*corev1.Pod, shr *faasv1.SharePod) error {
	diff := len(filteredPods) - int(*(shr.Spec.Replicas))

	shrKey, err := KeyFunc(shr)

	shrCopy := shr.DeepCopy()

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for %v %#v: %v", shr.Kind, shr, err))
		return nil
	}

	if diff < 0 {
		diff *= -1

		//may also set for burstresplicas?

		c.expectations.ExpectCreations(shrKey, diff)

		klog.V(2).Infof("Too few replicas for this SharePod...\n need %d replicas, creating %d", *(&shrCopy.Spec.Replicas), diff)

		// Batch the pod creates. Batch sizes start at SlowStartInitialBatchSize
		// and double with each successful iteration in a kind of "slow start".
		// This handles attempts to start large numbers of pods that would
		// likely all fail with the same error. For example a project with a
		// low quota that attempts to create a large number of pods will be
		// prevented from spamming the API service with the pod create requests
		// after one of its pods fails.  Conveniently, this also prevents the
		// event spam that those failures would generate.
		successfulCreations, err := slowStartbatch(diff, k8scontroller.SlowStartInitialBatchSize, func() (corev1.Pod, error) {
			newPod, err := c.kubeclient.CoreV1().Pods(shrCopy.Namespace).Create(context.TODO(), newPod(shr, true), metav1.CreateOptions{})

			//
			if err != nil {
				if apierrors.HasStatusCause(err, v1.NamespaceTerminatingCause) {
					return *newPod, nil
				}
			}
			return *newPod, err
		})

		// Any skipped pods that we never attempted to start shouldn't be expected.
		// The skipped pods will be retried later. The next controller resync will
		// retry the slow start process.
		if skippedPods := diff - successfulCreations; skippedPods > 0 {
			klog.V(2).Infof("Slow-start failure. Skipping creation of %d pods, decrementing expectations for %v %v/%v", skippedPods, shr.Kind, shr.Namespace, shr.Name)
			for i := 0; i < skippedPods; i++ {

				//Decrement the expected number of creates because the informer won't observe this pod
				c.expectations.CreationObserved(shrKey)
			}
		}
		return err

	} else if diff > 0 {
		klog.V(2).InfoS("Too many replicas", "SharePod", klog.KObj(shr), "need", *(&shr.Spec.Replicas), "deleting", diff)

		//indirect pods are in our case simply dummy pod that we can ignore
		//relatedPods, err := getIndirectly
	}

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
func slowStartbatch(count int, initailBatchSize int, fn func() (corev1.Pod, error)) (int, error) {
	remaining := count
	successes := 0
	for batchSize := integer.IntMin(remaining, initailBatchSize); batchSize > 0; batchSize = integer.IntMin(2*batchSize, remaining) {
		errCh := make(chan error, batchSize)
		var wg sync.WaitGroup
		wg.Add(batchSize)
		for i := 0; i < batchSize; i++ {
			go func() {
				defer wg.Done()
				if _, err := fn(); err != nil {
					errCh <- err
				}
			}()
		}

		wg.Wait()
		curSuccesses := batchSize - len(errCh)
		successes += curSuccesses
		if len(errCh) > 0 {
			return successes, <-errCh
		}
		remaining -= batchSize
	}

	return successes, nil
}

// newDeployment creates a new Deployment for a SharePod resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the SharePod resource that 'owns' it.
func newPod(oldpod *corev1.Pod, isGPUPod bool, podManagerIP string, podManagerPort int, boundDeviceId string) *corev1.Pod {
	specCopy := oldpod.Spec.DeepCopy()
	labelCopy := make(map[string]string, len(oldpod.ObjectMeta.Labels))
	for key, val := range oldpod.ObjectMeta.Labels {
		labelCopy[key] = val
	}
	annotationCopy := make(map[string]string, len(oldpod.ObjectMeta.Annotations)+4)
	for key, val := range oldpod.ObjectMeta.Annotations {
		annotationCopy[key] = val
	}
	if isGPUPod {
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
					Value: fmt.Sprintf("%s/%s", oldpod.ObjectMeta.Namespace, oldpod.ObjectMeta.Name),
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
		annotationCopy[faasv1.KubeShareResourceGPURequest] = oldpod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPURequest]
		annotationCopy[faasv1.KubeShareResourceGPULimit] = oldpod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPULimit]
		annotationCopy[faasv1.KubeShareResourceGPUMemory] = oldpod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPUMemory]
		annotationCopy[faasv1.KubeShareResourceGPUID] = oldpod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPUID]
	}

	ownerRef := oldpod.ObjectMeta.DeepCopy().OwnerReferences

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            oldpod.ObjectMeta.Name,
			Namespace:       oldpod.ObjectMeta.Namespace,
			OwnerReferences: ownerRef,
			Annotations:     annotationCopy,
			Labels:          labelCopy,
		},
		Spec: *specCopy,
	}
}

func patchSharepod(oldpod *corev1.Pod, isGPUPod bool, podManagerIP string, podManagerPort int, boundDeviceId string) map[string]interface{} {
	specCopy := oldpod.Spec.DeepCopy()
	//labelCopy := make(map[string]string, len(oldpod.ObjectMeta.Labels))
	cData := map[string]interface{}{}
	newEnvVar := []corev1.EnvVar{}

	newEnvVar = append(newEnvVar,
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
			Value: fmt.Sprintf("%s/%s", oldpod.ObjectMeta.Namespace, oldpod.ObjectMeta.Name),
		})

	for _, c := range specCopy.Containers {
		cData[c.Name] = map[string]interface{}{
			"env": newEnvVar,
			"volumeMounts": corev1.VolumeMount{
				Name:      "kubeshare-lib",
				MountPath: KubeShareLibraryPath,
			},
		}
	}

	patchData := map[string]interface{}{
		"containers": cData,
		"volumes": corev1.Volume{
			Name: "kubeshare-lib",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: KubeShareLibraryPath,
				},
			},
		},
	}

	return patchData
}
