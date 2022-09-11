package devicemanager

import (
	"container/list"
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog"

	faasshareV1 "github.com/Interstellarss/faas-share/pkg/apis/faasshare/v1"
	//kubesharev1 "github.com/Interstellarss/faas-share/pkg/apis/faasshare/v1"
	"github.com/Interstellarss/faas-share/pkg/lib/bitmap"
)

var (
	ResourceQuantity1 = resource.MustParse("1")
)

type PodRequest struct {
	Key            string
	Request        float64
	Limit          float64
	Memory         int64
	PodManagerPort int
}

type GPUInfo struct {
	UUID    string
	Usage   float64
	Mem     int64
	PodList *list.List
}

type NodeInfo struct {
	// GPUID -> GPU
	GPUID2GPU map[string]*GPUInfo
	// UUID -> Port (string)
	UUID2Port map[string]string

	// port in use
	PodManagerPortBitmap *bitmap.RRBitmap
	PodIP                string
}

var (
	nodesInfo    map[string]*NodeInfo = make(map[string]*NodeInfo)
	nodesInfoMux sync.Mutex

	podNamePool map[string]*list.List = make(map[string]*list.List)
	//is this useful?
	podNamePoolMux sync.Mutex

	schedNodePool map[string]*list.List = make(map[string]*list.List)

	schedNodePoolMux sync.Mutex
)

//TODO here to extend to new Struct for multil access for different sharepod
type NameList struct {
	namelist *list.List
	listMux  sync.Mutex
}

func (c *Controller) initNodesInfo() error {
	//TODO: need new InitnodeInfo for faas-share that go through
	var pods []*corev1.Pod
	var sharepods []*faasshareV1.SharePod
	//var deployments []*appsv1.Deployment
	var nodes []*corev1.Node

	var err error

	dummyPodsLabel := labels.SelectorFromSet(labels.Set{faasshareV1.KubeShareRole: "dummyPod"})
	if pods, err = c.podsLister.Pods("kube-system").List(dummyPodsLabel); err != nil {
		errrr := fmt.Errorf("error when list Pods: %s", err)
		klog.Error(errrr)
		return errrr
	}
	if sharepods, err = c.sharepodsLister.List(labels.Everything()); err != nil {
		errrr := fmt.Errorf("error when list sharepods: %s", err)
		klog.Error(errrr)
		return errrr
	}

	if nodes, err = c.nodesLister.List(labels.Everything()); err != nil {
		errr := fmt.Errorf("error when list nodes: #{err}")
		klog.Error(errr)
		return errr
	}

	nodesInfoMux.Lock()
	defer nodesInfoMux.Unlock()

	for _, pod := range pods {
		GPUID := ""
		if gpuid, ok := pod.ObjectMeta.Labels[faasshareV1.KubeShareResourceGPUID]; !ok {
			klog.Errorf("Error dummy Pod annotation: %s/%s", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
			continue
		} else {
			GPUID = gpuid
		}
		if node, ok := nodesInfo[pod.Spec.NodeName]; !ok {
			bm := bitmap.NewRRBitmap(512)
			bm.Mask(0)
			node = &NodeInfo{
				GPUID2GPU:            make(map[string]*GPUInfo),
				PodManagerPortBitmap: bm,
			}
			node.GPUID2GPU[GPUID] = &GPUInfo{
				UUID:    "",
				Usage:   0.0,
				Mem:     0,
				PodList: list.New(),
			}
			nodesInfo[pod.Spec.NodeName] = node
		} else {
			_, ok := node.GPUID2GPU[GPUID]
			if ok {
				klog.Errorf("Duplicated GPUID '%s' on node '%s'", GPUID, pod.Spec.NodeName)
				continue
			}
			node.GPUID2GPU[GPUID] = &GPUInfo{
				UUID:    "",
				Usage:   0.0,
				Mem:     0,
				PodList: list.New(),
			}
		}
	}

	type processDummyPodLaterItem struct {
		NodeName string
		GPUID    string
	}
	var processDummyPodLaterList []processDummyPodLaterItem

	for _, sharepod := range sharepods {

		selector, err := metav1.LabelSelectorAsSelector(sharepod.Spec.Selector)
		if err != nil {
			return err
		}

		pods, err := c.podsLister.Pods(sharepod.Namespace).List(selector)

		if err != nil {
			return err
		}

		for _, pod := range pods {
			gpu_request := 0.0
			gpu_limit := 0.0
			gpu_mem := int64(0)
			GPUID := ""

			var err error
			gpu_limit, err = strconv.ParseFloat(pod.ObjectMeta.Annotations[faasshareV1.KubeShareResourceGPULimit], 64)
			if err != nil || gpu_limit > 1.0 || gpu_limit < 0.0 {
				continue
			}
			gpu_request, err = strconv.ParseFloat(pod.ObjectMeta.Annotations[faasshareV1.KubeShareResourceGPURequest], 64)
			if err != nil || gpu_request > gpu_limit || gpu_request < 0.0 {
				continue
			}
			gpu_mem, err = strconv.ParseInt(pod.ObjectMeta.Annotations[faasshareV1.KubeShareResourceGPUMemory], 10, 64)
			if err != nil || gpu_mem < 0 {
				continue
			}

			// after this line, pod of a sharepod requires GPU

			// this sharepod may not be scheduled yet
			if pod.Spec.NodeName == "" {
				continue
			}
			// if Spec.NodeName is assigned but GPUID is empty, it's an error
			if gpuid, ok := sharepod.ObjectMeta.Annotations[faasshareV1.KubeShareResourceGPUID]; !ok {
				continue
			} else {
				GPUID = gpuid
			}

			node, ok := nodesInfo[pod.Spec.NodeName]
			if !ok {
				klog.Errorf("SharePod '%s/%s' doesn't have corresponding dummy Pod!", sharepod.ObjectMeta.Namespace, sharepod.ObjectMeta.Name)
				continue
			}
			gpu, ok := node.GPUID2GPU[GPUID]
			if !ok {
				klog.Errorf("SharePod '%s/%s' doesn't have corresponding dummy Pod!", sharepod.ObjectMeta.Namespace, sharepod.ObjectMeta.Name)
				continue
			}
			gpu.Usage += gpu_request
			gpu.Mem += gpu_mem
			gpu.PodList.PushBack(&PodRequest{
				Key:            fmt.Sprintf("%s/%s", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name),
				Request:        gpu_request,
				Limit:          gpu_limit,
				Memory:         gpu_mem,
				PodManagerPort: (*sharepod.Status.PodManagerPort)[pod.Name],
			})

			node.PodManagerPortBitmap.Mask((*sharepod.Status.PodManagerPort)[pod.Name] - PodManagerPortStart)

			//find bounddeviceID
			var index int
			for i, Env := range pod.Spec.Containers[0].Env {
				if Env.Name == "NVIDIA_VISIBLE_DEVICES" {
					index = i
					break
				}
				continue
			}

			if pod.Spec.Containers[0].Env[index].Value != "" {
				if gpu.UUID == "" {
					tmp := *sharepod.Status.BoundDeviceIDs
					gpu.UUID = tmp[pod.Name]
				}
			} else {
				if gpu.UUID != "" {
					c.workqueue.Add(fmt.Sprintf("%s/%s", sharepod.ObjectMeta.Namespace, sharepod.ObjectMeta.Name))
				} else {
					notFound := true
					for _, item := range processDummyPodLaterList {
						if item.NodeName == pod.Spec.NodeName && item.GPUID == GPUID {
							notFound = false
						}
					}
					if notFound {
						processDummyPodLaterList = append(processDummyPodLaterList, processDummyPodLaterItem{
							NodeName: pod.Spec.NodeName,
							GPUID:    GPUID,
						})
					}
				}
			}
		}
	}

	if len(processDummyPodLaterList) == 0 {
		for _, node := range nodes {
			dummyPodItem := processDummyPodLaterItem{
				NodeName: node.Name,
				GPUID:    faasshareV1.NewGPUID(5),
			}
			processDummyPodLaterList = append(processDummyPodLaterList, dummyPodItem)

			if nod, ok := nodesInfo[node.Name]; !ok {
				bm := bitmap.NewRRBitmap(512)
				bm.Mask(0)
				nod = &NodeInfo{
					GPUID2GPU:            make(map[string]*GPUInfo),
					PodManagerPortBitmap: bm,
				}
				nod.GPUID2GPU[dummyPodItem.GPUID] = &GPUInfo{
					UUID:    "",
					Usage:   0.0,
					Mem:     0,
					PodList: list.New(),
				}
				nodesInfo[node.Name] = nod
			}
		}
	} else {
		for _, node := range nodes {
			notFound := true
			for _, item := range processDummyPodLaterList {
				if item.NodeName == node.Name && item.GPUID != "" {
					notFound = false
				}
			}

			if notFound {
				dummyPodItem := processDummyPodLaterItem{
					NodeName: node.Name,
					GPUID:    faasshareV1.NewGPUID(5),
				}
				processDummyPodLaterList = append(processDummyPodLaterList, dummyPodItem)

				if nod, ok := nodesInfo[node.Name]; !ok {
					bm := bitmap.NewRRBitmap(512)
					bm.Mask(0)
					nod = &NodeInfo{
						GPUID2GPU:            make(map[string]*GPUInfo),
						PodManagerPortBitmap: bm,
					}
					nod.GPUID2GPU[dummyPodItem.GPUID] = &GPUInfo{
						UUID:    "",
						Usage:   0.0,
						Mem:     0,
						PodList: list.New(),
					}
					nodesInfo[node.Name] = nod
				}
			}
		}
	}

	for _, item := range processDummyPodLaterList {
		go c.createDummyPod(item.NodeName, item.GPUID)
	}
	return nil
}

func (c *Controller) cleanOrphanDummyPod() {
	nodesInfoMux.Lock()
	defer nodesInfoMux.Unlock()

	for nodeName, node := range nodesInfo {
		for gpuid, gpu := range node.GPUID2GPU {
			if gpu.PodList.Len() == 0 {
				klog.V(5).Infof("vGPU %s on Node %s is now orphan...", gpuid, nodeName)
				//delete(node.GPUID2GPU, gpuid)
				//c.deleteDummyPod(nodeName, gpuid, gpu.UUID)
			}
		}
	}
}

func FindInQueue(key string, pl *list.List) (*PodRequest, bool) {
	for k := pl.Front(); k != nil; k = k.Next() {
		if k.Value.(*PodRequest).Key == key {
			return k.Value.(*PodRequest), true
		}
	}
	return nil, false
}

/* getPhysicalGPUuuid returns valid uuid if errCode==0
 * errCode 0: no error
 * errCode 1: need dummy Pod
 * errCode 2: resource exceed
 * errCode 3: Pod manager port pool is full
 * errCode 255: other error
 */
func (c *Controller) getPhysicalGPUuuid(nodeName string, GPUID string, gpu_request, gpu_limit float64, gpu_mem int64, key string, port *int) (uuid string, errCode int) {

	nodesInfoMux.Lock()
	defer nodesInfoMux.Unlock()

	node, ok := nodesInfo[nodeName]
	if !ok {
		msg := fmt.Sprintf("No client node: %s", nodeName)
		klog.Errorf(msg)
		return "", 255
	}

	if gpu, ok := node.GPUID2GPU[GPUID]; !ok {
		gpu = &GPUInfo{
			UUID:    "",
			Usage:   gpu_request,
			Mem:     gpu_mem,
			PodList: list.New(),
		}
		tmp := node.PodManagerPortBitmap.FindNextFromCurrentAndSet() + PodManagerPortStart
		if tmp == -1 {
			klog.Errorf("Pod manager port pool is full!!!!!")
			return "", 3
		}
		*port = tmp
		gpu.PodList.PushBack(&PodRequest{
			Key:            key,
			Request:        gpu_request,
			Limit:          gpu_limit,
			Memory:         gpu_mem,
			PodManagerPort: *port,
		})
		node.GPUID2GPU[GPUID] = gpu
		go c.createDummyPod(nodeName, GPUID)
		return "", 1
	} else {
		if podreq, isFound := FindInQueue(key, gpu.PodList); !isFound {
			if tmp := gpu.Usage + gpu_request; tmp > 1.0 {
				klog.Infof("Resource exceed, usage: %f, new_req: %f", gpu.Usage, gpu_request)
				return "", 2
			} else {
				gpu.Usage = tmp
			}
			gpu.Mem += gpu_mem
			tmp := node.PodManagerPortBitmap.FindNextFromCurrentAndSet() + PodManagerPortStart
			if tmp == -1 {
				klog.Errorf("Pod manager port pool is full!!!!!")
				return "", 3
			}
			*port = tmp
			gpu.PodList.PushBack(&PodRequest{
				Key:            key,
				Request:        gpu_request,
				Limit:          gpu_limit,
				Memory:         gpu_mem,
				PodManagerPort: *port,
			})
		} else {
			*port = podreq.PodManagerPort
		}
		if gpu.UUID == "" {
			return "", 1
		} else {
			syncConfig(nodeName, gpu.UUID, gpu.PodList)
			return gpu.UUID, 0
		}
	}

	return "", 255
}

func (c *Controller) createDummyPod(nodeName string, GPUID string) error {
	podName := fmt.Sprintf("%s-%s-%s", faasshareV1.KubeShareDummyPodName, nodeName, GPUID)

	createit := func() error {
		klog.Infof("ERICYEH: creating dummy pod: %s", podName)
		// create a pod for dummy gpu then waiting for running, and get its gpu deviceID
		createdPod, err := c.kubeclient.CoreV1().Pods("kube-system").Create(context.TODO(), &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: "kube-system",
				Labels: map[string]string{
					faasshareV1.KubeShareRole:          "dummyPod",
					faasshareV1.KubeShareNodeName:      nodeName,
					faasshareV1.KubeShareResourceGPUID: GPUID,
				},
			},
			Spec: corev1.PodSpec{
				NodeName:                      nodeName,
				TerminationGracePeriodSeconds: new(int64),
				Containers: []corev1.Container{
					corev1.Container{
						Name:  "sleepforever",
						Image: "ncy9371/kubeshare-vgpupod:200217232846",
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{faasshareV1.ResourceNVIDIAGPU: ResourceQuantity1},
							Limits:   corev1.ResourceList{faasshareV1.ResourceNVIDIAGPU: ResourceQuantity1},
						},
					},
				},
				RestartPolicy: corev1.RestartPolicyNever,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			_, exists := c.kubeclient.CoreV1().Pods("kube-system").Get(context.TODO(), podName, metav1.GetOptions{})
			if exists != nil {
				klog.Errorf("Error when creating dummy pod: \nerror: '%s',\n podspec: %-v", err, createdPod)
				return err
			}
		}
		return nil
	}
	klog.Infof("Dummy pod %s on Node %s is created!", podName, nodeName)

	if dummypod, err := c.kubeclient.CoreV1().Pods("kube-system").Get(context.TODO(), podName, metav1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			havetocreate := true
			nodesInfoMux.Lock()
			_, havetocreate = nodesInfo[nodeName].GPUID2GPU[GPUID]
			nodesInfoMux.Unlock()
			if havetocreate {
				createit()
			}
		} else {
			msg := fmt.Sprintf("List Pods resource error! nodeName: %s", nodeName)
			klog.Errorf(msg)
			return err
		}
	} else {
		if dummypod.ObjectMeta.DeletionTimestamp != nil {
			// TODO: If Dummy Pod had been deleted, re-create it later
			klog.Warningf("Unhandled: Dummy Pod %s is deleting! re-create it later!", podName)
		}
		if dummypod.Status.Phase == corev1.PodRunning || dummypod.Status.Phase == corev1.PodFailed {
			c.getAndSetUUIDFromDummyPod(nodeName, GPUID, podName, dummypod)
		}
	}

	return nil
}

// dummyPod status must be Running or Failed
// triggered from Pod event handler, preventing request throttling
func (c *Controller) getAndSetUUIDFromDummyPod(nodeName, GPUID, podName string, dummyPod *corev1.Pod) error {
	if dummyPod.Status.Phase == corev1.PodFailed {
		c.kubeclient.CoreV1().Pods("kube-system").Delete(context.TODO(), podName, metav1.DeleteOptions{})
		time.Sleep(time.Second)
		c.createDummyPod(nodeName, GPUID)
		err := fmt.Errorf("Dummy Pod '%s' status failed, restart it.", podName)
		klog.Errorf(err.Error())
		return err
	}
	// dummyPod.Status.Phase must be Running
	var uuid string
	rawlog, logerr := c.kubeclient.CoreV1().Pods("kube-system").GetLogs(podName, &corev1.PodLogOptions{}).Do(context.TODO()).Raw()
	if logerr != nil {
		err := fmt.Errorf("Error when get dummy pod's log! pod namespace/name: %s/%s, error: %s", "kube-system", podName, logerr)
		klog.Errorf(err.Error())
		return err
	}
	uuid = strings.Trim(string(rawlog), " \n\t")
	klog.Infof("Dummy Pod %s get device ID: '%s'", podName, uuid)
	isFound := false
	for id := range nodesInfo[nodeName].UUID2Port {
		if id == uuid {
			isFound = true
		}
	}

	if !isFound {
		err := fmt.Errorf("Cannot find UUID '%s' from dummy Pod: '%s' in UUID database. ", uuid, podName)
		klog.Errorf(err.Error())
		// possibly not print UUID yet, try again
		time.Sleep(time.Second)
		go c.createDummyPod(nodeName, GPUID)
		return err
	}

	nodesInfoMux.Lock()
	defer nodesInfoMux.Unlock()

	nodesInfo[nodeName].GPUID2GPU[GPUID].UUID = uuid

	PodList := nodesInfo[nodeName].GPUID2GPU[GPUID].PodList
	klog.Infof("After dummy Pod created, PodList Len: %d", PodList.Len())
	for k := PodList.Front(); k != nil; k = k.Next() {
		klog.Infof("Add MtgpuPod back to queue then process: %s", k.Value)
		c.workqueue.Add(k.Value.(*PodRequest).Key)
	}

	return nil
}

func (c *Controller) removePodFromList(sharepod *faasshareV1.SharePod, pod *corev1.Pod) {
	nodeName := pod.Spec.NodeName
	GPUID := pod.Annotations[faasshareV1.KubeShareResourceGPUID]
	key := fmt.Sprintf("%s/%s", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)

	nodesInfoMux.Lock()
	defer nodesInfoMux.Unlock()
	if node, nodeOk := nodesInfo[nodeName]; nodeOk {
		if gpu, gpuOk := node.GPUID2GPU[GPUID]; gpuOk {
			podlist := gpu.PodList
			for pod := podlist.Front(); pod != nil; pod = pod.Next() {
				podRequest := pod.Value.(*PodRequest)
				//TODO
				if podRequest.Key == key {
					klog.Infof("Remove MtgpuPod %s from list, remaining %d MtgpuPod(s).", key, podlist.Len())
					podlist.Remove(pod)

					uuid := gpu.UUID
					remove := false

					if podlist.Len() == 0 {
						//delete(node.GPUID2GPU, GPUID)
						//remove = true
					} else {
						gpu.Usage -= podRequest.Request
						gpu.Mem -= podRequest.Memory
						syncConfig(nodeName, uuid, podlist)
					}
					node.PodManagerPortBitmap.Unmask(podRequest.PodManagerPort - PodManagerPortStart)

					//nodesInfoMux.Unlock()

					if remove {
						c.deleteDummyPod(nodeName, GPUID, uuid)
					}
					continue
				}
			}
		}
	}
	//nodesInfoMux.Unlock()
}

func (c *Controller) removeSharePodFromList(sharepod *faasshareV1.SharePod) {

	selector, err := metav1.LabelSelectorAsSelector(sharepod.Spec.Selector)
	if err != nil {
		//return err
		utilruntime.HandleError(err)
	}

	namepsace := sharepod.Namespace

	pods, err := c.podsLister.Pods(namepsace).List(selector)

	if err != nil {
		utilruntime.HandleError(err)
	}
	nodesInfoMux.Lock()
	defer nodesInfoMux.Unlock()

	for _, pod := range pods {
		nodeName := pod.Spec.NodeName
		GPUID := pod.Annotations[faasshareV1.KubeShareResourceGPUID]
		key := fmt.Sprintf("%s/%s", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)

		if node, nodeOk := nodesInfo[nodeName]; nodeOk {
			if gpu, gpuOk := node.GPUID2GPU[GPUID]; gpuOk {
				podlist := gpu.PodList
				for pod := podlist.Front(); pod != nil; pod = pod.Next() {
					podRequest := pod.Value.(*PodRequest)
					//TODO
					if podRequest.Key == key {
						klog.Infof("Remove MtgpuPod %s from list, remaining %d MtgpuPod(s).", key, podlist.Len())
						podlist.Remove(pod)

						uuid := gpu.UUID
						remove := false

						if podlist.Len() == 0 {
							//delete(node.GPUID2GPU, GPUID)
							//remove = true
						} else {
							gpu.Usage -= podRequest.Request
							gpu.Mem -= podRequest.Memory
							syncConfig(nodeName, uuid, podlist)
						}
						node.PodManagerPortBitmap.Unmask(podRequest.PodManagerPort - PodManagerPortStart)

						//nodesInfoMux.Unlock()

						if remove {
							c.deleteDummyPod(nodeName, GPUID, uuid)
						}
						continue
					}
				}
			}
		}
		//nodesInfoMux.Unlock()
	}

}

func (c *Controller) deleteDummyPod(nodeName, GPUID, uuid string) {
	key := fmt.Sprintf("%s-%s-%s", faasshareV1.KubeShareDummyPodName, nodeName, GPUID)
	klog.Infof("Deleting dummy Pod: %s", key)
	c.kubeclient.CoreV1().Pods("kube-system").Delete(context.TODO(), key, metav1.DeleteOptions{})
	syncConfig(nodeName, uuid, nil)
}
