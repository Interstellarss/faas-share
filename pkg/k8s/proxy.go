// Copyright (c) Alex Ellis 2017. All rights reserved.
// Copyright 2020 OpenFaaS Author(s)
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package k8s

import (
	"context"
	"errors"
	"fmt"
	clientset "github.com/Interstellarss/faas-share/pkg/client/clientset/versioned"
	v1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"math"
	//"math/rand"
	"strconv"

	//"net/http"
	"sort"
	"time"

	//"math/rand"
	"net/url"
	"strings"
	"sync"

	faas "github.com/Interstellarss/faas-share/pkg/client/listers/faasshare/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corelister "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog"
)

// watchdogPort for the OpenFaaS function watchdog
const watchdogPort = 8080

//target request per pod
const target = "com.openfaas.scale.target"

type PodsWithInfos struct {
	//Pods     []PodInfo
	Pods     []*v1.Pod
	podInfos map[string]PodInfo

	Now metav1.Time
}

func (s PodsWithInfos) Len() int {
	return len(s.Pods)
}

func (s PodsWithInfos) Swap(i, j int) {
	s.Pods[i], s.Pods[j] = s.Pods[j], s.Pods[i]
}

func (s PodsWithInfos) Less(i, j int) bool {
	name_i := s.Pods[i].Name

	name_j := s.Pods[j].Name

	//if a pod is unsigned, then the unsigned one is smaller

	_, ok := s.podInfos[name_i]

	_, ok2 := s.podInfos[name_j]

	if !ok || !ok2 {
		return !ok
	}

	//rate smaller < larger rate
	/*
		if s.podInfos[name_i].RateChange >= s.podInfos[name_j].RateChange {
			return s.podInfos[name_i].Rate < s.podInfos[name_j].Rate
		}
	*/
	if s.podInfos[name_i].Rate == 0 || s.podInfos[name_j].Rate == 0 {
		return !(s.podInfos[name_i].Rate == 0)
	}

	return s.podInfos[name_i].Rate < s.podInfos[name_j].Rate

	//return s.podInfos[name_i].RateChange < s.podInfos[name_j].RateChange
}

func NewFunctionLookup(ns string, podLister corelister.PodLister, faasLister faas.SharePodLister, sharepodInfos map[string]*SharePodInfo) *FunctionLookup {
	return &FunctionLookup{
		DefaultNamespace: ns,
		//EndpointLister:   lister,
		faasLister: faasLister,
		podLister:  podLister,
		Listers:    map[string]shareLister{},
		ShareInfos: sharepodInfos,
		lock:       sync.RWMutex{},
	}
}

type FunctionLookup struct {
	DefaultNamespace string
	//EndpointLister   corelister.EndpointsLister
	//endpoint lister may not needed for custom version
	faasLister faas.SharePodLister
	podLister  corelister.PodLister
	Listers    map[string]shareLister

	RateRep bool

	//Service bool

	ShareInfos map[string]*SharePodInfo
	lock       sync.RWMutex
}

type shareLister struct {
	corelister.PodNamespaceLister
	faas.SharePodNamespaceLister
}

//extension to moultiple namespaces
func (f *FunctionLookup) GetLister(ns string) shareLister {
	f.lock.RLock()
	defer f.lock.RUnlock()
	return f.Listers[ns]
}

func (f *FunctionLookup) SetLister(ns string, lister shareLister) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.Listers[ns] = lister
}

func getNamespace(name, defaultNamespace string) string {
	namespace := defaultNamespace
	if strings.Contains(name, ".") {
		namespace = name[strings.LastIndexAny(name, ".")+1:]
	}
	return namespace
}

func (l *FunctionLookup) Resolve(name string, suffix string) (url.URL, string, error) {
	functionName := name
	namespace := getNamespace(name, l.DefaultNamespace)
	if err := l.verifyNamespace(namespace); err != nil {
		return url.URL{}, "", err
	}

	if strings.Contains(name, ".") {
		functionName = strings.TrimSuffix(name, "."+namespace)
	}

	shrpod, err := l.faasLister.SharePods(namespace).Get(functionName)

	if err != nil {
		return url.URL{}, "", err
	}

	selector, err := metav1.LabelSelectorAsSelector(shrpod.Spec.Selector)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error converting pod selector to selector for shr %v/%v: %v", namespace, name, err))
	}

	if selector == nil {
		klog.Infof("selector is till nil...")
		return url.URL{}, "", errors.New("NilSelector")
	}

	//sharepod, err := c.sharepodsLister.SharePods(namespace).Get(name)

	pods, err := l.podLister.Pods(namespace).List(selector)

	if err != nil {
		return url.URL{}, "", err
	}

	filteredPods := FilterActivePods(pods)

	var podName string

	var serviceIP string
	//if

	if len(filteredPods) > 0 {
		if shareinfo, ok := l.ShareInfos[functionName]; ok {

			shareinfo.Lock.Lock()

			pInfos := (l.ShareInfos[functionName]).PodInfos

			if len(pInfos) > 0 {
				podsWithinfo := PodsWithInfos{
					Pods:     filteredPods,
					podInfos: pInfos,
					Now:      metav1.Now(),
				}
				sort.Sort(podsWithinfo)
			}
			//pods := make([]PodInfo, len(pInfos))
			/*
				for _, v := range pInfos {
					pods = append(pods, v)
				}

				if err != nil {
					return url.URL{}, "", err
				}
				//pods[0].Status.PodIP

				//podsWithRanks :=

				infos := PodsWithInfos{
					Pods:     pods,
					podInfos: pInfos,
					Now:      metav1.Now(),
				}
			*/
			target := GenerateRangeNum(len(filteredPods)/2, len(filteredPods))
			podName = filteredPods[target].Name
			//TODO: ip is nil?
			serviceIP = filteredPods[target].Status.PodIP

			if podinfo, ok := l.ShareInfos[functionName].PodInfos[podName]; ok {
				podinfo.TotalInvoke++
				if podinfo.PodIp == "" {
					podinfo.PodIp = serviceIP
				}
			} else {
				l.ShareInfos[functionName].PodInfos[podName] = PodInfo{
					PodName:     podName,
					ServiceName: functionName,
					PodIp:       serviceIP,
					TotalInvoke: 1,
				}
			}

			shareinfo.Lock.Unlock()
		} else {
			podName = pods[len(pods)-1].Name
			serviceIP = pods[len(pods)-1].Status.PodIP

			l.AddFunc(functionName)
		}
	}
	//klog.Infof("picking pod %s out of sharpeod %s with pod IP %s", podName, name, serviceIP)
	//pods[0].Status.ContainerStatuses[0].ContainerID
	/*
		nsEndpointLister := l.GetLister(namespace)

		if nsEndpointLister == nil {
			l.SetLister(namespace, l.EndpointLister.Endpoints(namespace))
			nsEndpointLister = l.GetLister(namespace)

		}

		svc, err := nsEndpointLister.Get(functionName)
		if err != nil {
			return url.URL{}, fmt.Errorf("error listing \"%s.%s\": %s", functionName, namespace, err.Error())
		}

		if len(svc.Subsets) == 0 {
			return url.URL{}, fmt.Errorf("no subsets available for \"%s.%s\"", functionName, namespace)
		}

		all := len(svc.Subsets[0].Addresses)
		if len(svc.Subsets[0].Addresses) == 0 {
			return url.URL{}, fmt.Errorf("no addresses in subset for \"%s.%s\"", functionName, namespace)
		}

		target := rand.Intn(all)

		serviceIP := svc.Subsets[0].Addresses[target].IP
	*/
	var urlStr string
	if serviceIP != "" {
		if suffix == "" {
			urlStr = fmt.Sprintf("http://%s:%d/", serviceIP, watchdogPort)
		} else {
			urlStr = fmt.Sprintf("http://%s:%d/%s/", serviceIP, watchdogPort, suffix)
		}
	} else {
		if suffix == "" {
			urlStr = fmt.Sprintf("http://%s:%d", functionName, watchdogPort)
		} else {
			urlStr = fmt.Sprintf("http://%s%s:%d", functionName, suffix, watchdogPort)
		}
	}

	klog.Infof("picking pod %s out of sharpeod %s with pod IP %s", podName, name, serviceIP)

	urlRes, err := url.Parse(urlStr)
	if err != nil {
		return url.URL{}, "", err
	}

	return *urlRes, podName, nil
}

func (l *FunctionLookup) DeleteFunction(name string) {
	//l.lock
	delete(l.ShareInfos, name)
	return
}

func (l *FunctionLookup) DeletePodInfo(funcName string, podName string) {
	if shrInfo, ok := l.ShareInfos[funcName]; ok {
		shrInfo.Lock.Lock()
		defer shrInfo.Lock.Unlock()
		delete(shrInfo.PodInfos, podName)
		klog.Infof("Deleting pod %s info of shr %s", podName, funcName)
	}
}

func (l *FunctionLookup) GetSharePodInfo(name string) SharePodInfo {
	if _, ok := l.ShareInfos[name]; ok {
		return *l.ShareInfos[name]
	} else {
		l.AddFunc(name)
		return *l.ShareInfos[name]
	}
}

func (l *FunctionLookup) AddFunc(funcname string) {

	if sharepodinfo, ok := l.ShareInfos[funcname]; !ok {
		l.ShareInfos[funcname] = &SharePodInfo{PodInfos: make(map[string]PodInfo), Lock: sync.RWMutex{}}
		klog.Infof("Info of Sharepod %s initialized...", funcname)
	} else {
		if sharepodinfo.PodInfos == nil {
			sharepodinfo.PodInfos = make(map[string]PodInfo)
		}
	}

}

func (l *FunctionLookup) Update(duration time.Duration, functionName string, podName string, kube clientset.Interface) {
	//podinfo := *((*l.ShareInfos)[functionName])
	//var sharepodInfo SharePodInfo
	//sharepodInfo = (*l.ShareInfos)[functionName]
	//TODO:

	if _, ok := l.ShareInfos[functionName]; !ok {

		podinfos := make(map[string]PodInfo)

		podinfos[podName] = PodInfo{PodName: podName, ServiceName: functionName, AvgResponseTime: duration, LastResponseTime: duration, Rate: float32(1000) / float32(duration.Milliseconds()), TotalInvoke: 1, RateChange: Inc}

		l.ShareInfos[functionName] = &SharePodInfo{
			PodInfos: podinfos,
			Lock:     sync.RWMutex{},
		}
		klog.Infof("DEBUG: initializing, SharePod info %s", functionName)
		return
	} else {
		newReplica := false
		var totalInvoke int32 = 0
		var dec = 0
		l.ShareInfos[functionName].Lock.Lock()
		//test.lock.Lock()
		defer func() {
			l.ShareInfos[functionName].Lock.Unlock()
			if newReplica {
				go l.UpdateReplica(kube, l.DefaultNamespace, functionName, totalInvoke)
			}
		}()

		if podInfo, ok := l.ShareInfos[functionName].PodInfos[podName]; ok {
			//podInfo.totalInvoke++
			//time.Duration()
			var invoke_pre = podInfo.TotalInvoke - 1
			var invoke_cur = podInfo.TotalInvoke

			podInfo.AvgResponseTime = (podInfo.AvgResponseTime*(time.Duration(invoke_pre)) + duration) / time.Duration(invoke_cur)

			//podInfo.TotalInvoke++

			oldRate := podInfo.Rate
			if podInfo.AvgResponseTime.Milliseconds() > 0 {
				podInfo.Rate = float32(1000) / float32(podInfo.AvgResponseTime.Milliseconds())
			} else {
				podInfo.Rate = float32(1000) / float32(duration.Milliseconds())
				podInfo.AvgResponseTime = duration
			}

			podInfo.LastInvoke = time.Now()

			if podInfo.Rate/oldRate > 1.2 {
				podInfo.RateChange = ChangeType(Inc)
			} else if podInfo.Rate/oldRate < 0.8 {
				podInfo.RateChange = ChangeType(Dec)
				//needUpdate := false

			} else {
				podInfo.RateChange = ChangeType(Sta)
			}
		} else {
			klog.Infof("Sharepod %s with Pod %s 's info nil...", functionName, podName)
			l.ShareInfos[functionName].PodInfos[podName] = PodInfo{PodName: podName, ServiceName: functionName, AvgResponseTime: duration, TotalInvoke: 1,
				LastResponseTime: duration, RateChange: Inc, Rate: float32(1000) / float32(duration.Milliseconds())}
			//return
		}
		for _, podinfo := range l.ShareInfos[functionName].PodInfos {
			totalInvoke += podinfo.TotalInvoke
			if podinfo.RateChange == Dec {
				dec++
			}
		}
		//var ratio float32
		// <= or < ?
		//debugging
		klog.Infof("Sharepod %s with %d PodInfos and %d dec pods...", functionName, len(l.ShareInfos[functionName].PodInfos), dec)
		if len(l.ShareInfos[functionName].PodInfos)-dec < 1 {
			newReplica = true
		}
	}
}

func (l *FunctionLookup) UpdateReplica(kube clientset.Interface, namepsace string, shrName string, invoke int32) {
	//klog.Infof("pod %s of sharepod %s rate decrease...", podInfo.PodName, functionName)
	if l.RateRep {
		klog.Infof("Starting Update Sharepod %s Replica ...", shrName)

		shr, err := kube.KubeshareV1().SharePods(namepsace).Get(context.TODO(), shrName, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("Sharepod %s get error: %v", shrName, err)
			return
		}
		shrCopy := shr.DeepCopy()

		var targetRep int32
		/*
			if value, ok := shrCopy.Labels[controller.LabelMaxReplicas];ok{
				r, err := strconv.Atoi(value)
				if err == nil && r > 0{

				}
			}
		*/

		if t, ok := shrCopy.ObjectMeta.Labels[target]; ok {

			tar, errr := strconv.ParseInt(t, 10, 32)
			if errr != nil {
				klog.Infof("Erro parsing target of sharepod %s...", shrName)
				return
			}
			targetRep = int32(math.Ceil(float64(invoke) / float64(tar)))
			klog.Infof("DEBUG:Target based with %d, and current %d invoke, need to upload %d", tar, invoke, targetRep)
			//shrCopy.Spec.Replicas = &targetRep
		} else {
			targetRep = int32(math.Ceil(float64(*shrCopy.Spec.Replicas) * 1.2))
		}

		if value, ok := shrCopy.Labels["com.openfaas.scale.max"]; ok {
			r, err := strconv.Atoi(value)
			if err == nil && r > 0 {
				if int32(r) < targetRep {
					klog.Infof("DEBUG: Overpass the maximum %i of SHR %s ...", r, shrCopy.Name)
					targetRep = int32(r)
					return
				}
			}
		}

		shrCopy.Spec.Replicas = &targetRep
		updatedShr, err := kube.KubeshareV1().SharePods(namepsace).Update(context.TODO(), shrCopy, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("Sharepod %s update error %v", shrName, err)
			return
		}
		if *updatedShr.Spec.Replicas == targetRep {
			klog.Infof("Sharepod %s replica updated to %v", shrName, targetRep)

		} else {
			klog.Infof("Sharepod %s with replica %i failed updated to %v replicas", updatedShr.Name, *updatedShr.Spec.Replicas, targetRep)
		}

	}

}

func (l *FunctionLookup) ScaleDown(funtionName string) {
	if podinfos, ok := l.ShareInfos[funtionName]; ok {
		podinfos.Lock.Lock()
		podinfos.ScaleDown = true
		podinfos.Lock.Unlock()
	}
}

func (l *FunctionLookup) Insert(shrName string, podName string, podIp string) {

	if sharepodInfo, ok := (l.ShareInfos)[shrName]; ok {

		sharepodInfo.Lock.Lock()
		defer sharepodInfo.Lock.Unlock()
		if podInfo, ok2 := (sharepodInfo.PodInfos)[podName]; ok2 {
			if podInfo.PodIp == "" {
				podInfo.PodIp = podIp
			}
		} else {
			sharepodInfo.PodInfos[podName] = PodInfo{PodName: podName, PodIp: podIp, ServiceName: shrName, TotalInvoke: 0, Rate: 0}
		}
	} else {
		podinfos := make(map[string]PodInfo)
		podinfos[podName] = PodInfo{PodName: podName, PodIp: podIp, ServiceName: shrName, TotalInvoke: 0, Rate: 0}
		(l.ShareInfos)[shrName] = &SharePodInfo{PodInfos: podinfos}
	}
}

func (l *FunctionLookup) deletepPodinfo(functionName string, podName string) {
	if sharepodInfo, ok := l.ShareInfos[functionName]; ok {
		sharepodInfo.Lock.Lock()
		defer sharepodInfo.Lock.Unlock()
		delete(sharepodInfo.PodInfos, podName)
	}
}

func (l *FunctionLookup) verifyNamespace(name string) error {
	if name != "kube-system" {
		return nil
	}
	// ToDo use global namepace parse and validation
	return fmt.Errorf("namespace not allowed")
}
