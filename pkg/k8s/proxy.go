// Copyright (c) Alex Ellis 2017. All rights reserved.
// Copyright 2020 OpenFaaS Author(s)
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package k8s

import (
	"errors"
	"fmt"
	"github.com/Interstellarss/faas-share/pkg/devicemanager"
	v1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
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
	if s.Pods[i].Spec.NodeName != s.Pods[j].Spec.NodeName && (len(s.Pods[i].Spec.NodeName) == 0 || len(s.Pods[j].Spec.NodeName) == 0) {
		return len(s.Pods[i].Spec.NodeName) == 0
	}

	//rate smaller < larger rate
	if s.podInfos[name_i].rateChange == s.podInfos[name_j].rateChange {
		return s.podInfos[name_i].rate < s.podInfos[name_j].rate
	}

	return s.podInfos[name_i].rateChange < s.podInfos[name_j].rateChange
}

func NewFunctionLookup(ns string, podLister corelister.PodLister, faasLister faas.SharePodLister, sharepodInfos map[string]*SharePodInfo) *FunctionLookup {
	return &FunctionLookup{
		DefaultNamespace: ns,
		//EndpointLister:   lister,
		faasLister: faasLister,
		podLister:  podLister,
		Listers:    map[string]shareLister{},
		shareInfos: sharepodInfos,
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

	shareInfos map[string]*SharePodInfo
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

	filteredPods := devicemanager.FilterActivePods(pods)

	pInfos := (l.shareInfos[functionName]).podInfos

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

	podsWithinfo := PodsWithInfos{
		Pods:     filteredPods,
		podInfos: pInfos,
		Now:      metav1.Now(),
	}

	sort.Sort(podsWithinfo)

	podName := pods[0].Name
	serviceIP := pods[0].Status.PodIP

	klog.Infof("picking pod %s out of sharpeod %s with pod IP %s", podName, pInfos[podName].serviceName, serviceIP)
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
	if suffix == "" {
		urlStr = fmt.Sprintf("http://%s:%d", serviceIP, watchdogPort)
	} else {
		urlStr = fmt.Sprintf("http://%s:%d/%s", serviceIP, watchdogPort, suffix)
	}

	urlRes, err := url.Parse(urlStr)
	if err != nil {
		return url.URL{}, "", err
	}

	return *urlRes, podName, nil
}

func (l *FunctionLookup) DeleteFunction(name string) {
	//l.lock
	delete(l.shareInfos, name)
	return
}

func (l *FunctionLookup) AddFunc(funcname string) {
	(l.shareInfos)[funcname] = &SharePodInfo{podInfos: make(map[string]PodInfo), lock: sync.RWMutex{}}
	klog.Infof("Info of Sharepod %s initialized...", funcname)
	/*
		if sharepodinfo, ok := (*l.shareInfos)[funcname]; !ok {
			sharepodinfo = SharePodInfo{podInfos: make(map[string]PodInfo), lock: sync.RWMutex{}}
			(*l.shareInfos)[funcname] = sharepodinfo
			klog.Infof("Info of Sharepod %s initialized...", funcname)
		} else {
			if &sharepodinfo.podInfos == nil {
				sharepodinfo.podInfos = make(map[string]PodInfo)
			}

		}
	*/
}

func (l *FunctionLookup) Update(duration time.Duration, functionName string, podName string) {
	//podinfo := *((*l.shareInfos)[functionName])
	//var sharepodInfo SharePodInfo
	//sharepodInfo = (*l.shareInfos)[functionName]
	//TODO:
	if l.shareInfos[functionName] == nil {

		podinfos := make(map[string]PodInfo)

		podinfos[podName] = PodInfo{podName: podName, serviceName: functionName, avgResponseTime: duration, lastResponseTime: duration, rate: float32(1000 / duration.Milliseconds()), totalInvoke: 1, rateChange: Inc}

		l.shareInfos[functionName] = &SharePodInfo{
			podInfos: podinfos,
			lock:     sync.RWMutex{},
		}
		klog.Infof("initializing, pod info %s", l.shareInfos[functionName].podInfos)
		return
	}

	l.shareInfos[functionName].lock.Lock()
	//test.lock.Lock()
	defer l.shareInfos[functionName].lock.Unlock()

	if podInfo, ok := l.shareInfos[functionName].podInfos[podName]; ok {
		//podInfo.totalInvoke++
		//time.Duration()
		var invoke_pre = podInfo.totalInvoke
		var invoke_cur = podInfo.totalInvoke + 1

		podInfo.avgResponseTime = (podInfo.avgResponseTime*(time.Duration(invoke_pre)) + duration) / time.Duration(invoke_cur)

		podInfo.totalInvoke++

		oldRate := podInfo.rate

		podInfo.rate = float32(1000 / podInfo.avgResponseTime.Milliseconds())

		if podInfo.rate/oldRate > 1.1 {
			podInfo.rateChange = ChangeType(Inc)
		} else if podInfo.rate/oldRate < 0.9 {
			podInfo.rateChange = ChangeType(Dec)
		} else {
			podInfo.rateChange = ChangeType(Sta)
		}

		//return

	} else {
		klog.Infof("Sharepod %s with Pod %s 's info nil...", functionName, podName)
		l.shareInfos[functionName].podInfos[podName] = PodInfo{podName: podName, serviceName: functionName, avgResponseTime: duration, totalInvoke: 1,
			lastResponseTime: duration, rateChange: Inc, rate: float32(1000 / duration.Milliseconds())}

		//return
	}

	newReplica := true

	for _, podinfo := range l.shareInfos[functionName].podInfos {
		if podinfo.rateChange == Inc || podinfo.rateChange == Sta {
			newReplica = false
		}
	}
	if newReplica {

	}

	//return
	//var testPod *PodInfo = (*l.shareInfos)[functionName].podInfos[podName]
	//return
}

func (l *FunctionLookup) Insert(shrName string, podName string, podIp string) {

	if sharepodInfo, ok := (l.shareInfos)[shrName]; ok {

		sharepodInfo.lock.Lock()
		defer sharepodInfo.lock.Unlock()
		if podInfo, ok2 := (sharepodInfo.podInfos)[podName]; ok2 {
			if podInfo.podIp == "" {
				podInfo.podIp = podIp
			}
		} else {
			sharepodInfo.podInfos[podName] = PodInfo{podName: podName, podIp: podIp, serviceName: shrName, totalInvoke: 0, rate: 0}
		}
	} else {
		podinfos := make(map[string]PodInfo)
		podinfos[podName] = PodInfo{podName: podName, podIp: podIp, serviceName: shrName, totalInvoke: 0, rate: 0}
		(l.shareInfos)[shrName] = &SharePodInfo{podInfos: podinfos}
		return
	}
}

func (l *FunctionLookup) verifyNamespace(name string) error {
	if name != "kube-system" {
		return nil
	}
	// ToDo use global namepace parse and validation
	return fmt.Errorf("namespace not allowed")
}
