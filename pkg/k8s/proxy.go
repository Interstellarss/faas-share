// Copyright (c) Alex Ellis 2017. All rights reserved.
// Copyright 2020 OpenFaaS Author(s)
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package k8s

import (
	"errors"
	"fmt"
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
	Pods     []PodInfo
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
	name_i := s.Pods[i].podName

	name_j := s.Pods[j].podName

	//if the ip is unsigned then the unsigned one is smaller
	if s.Pods[i].podName != s.Pods[j].podName {
		return len((s.podInfos[name_i]).podIp) == 0
	}

	//rate smaller < larger rate
	return (s.podInfos[name_i]).rate < (s.podInfos[name_j]).rate

}

func NewFunctionLookup(ns string, podLister corelister.PodLister, faasLister faas.SharePodLister, sharepodInfos *map[string]SharePodInfo) *FunctionLookup {
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

	shareInfos *map[string]SharePodInfo
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

func (l *FunctionLookup) Resolve(name string) (url.URL, string, error) {
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

	//pods, err := l.podLister.Pods(namespace).List(selector)
	pInfos := ((*l.shareInfos)[functionName]).podInfos

	pods := make([]PodInfo, len(pInfos))

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

	sort.Sort(infos)

	podName := pods[0].podName
	serviceIP := pods[0].podIp

	klog.Infof("picking pod %s out of sharpeod %s with pod IP %s", pods[0].podName, pods[0].serviceName, pods[0].podIp)
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

	urlStr := fmt.Sprintf("http://%s:%d", serviceIP, watchdogPort)

	urlRes, err := url.Parse(urlStr)
	if err != nil {
		return url.URL{}, "", err
	}

	return *urlRes, podName, nil
}

func (l *FunctionLookup) deleteFunction(name string) {
	delete(*l.shareInfos, name)
	return
}

func (l *FunctionLookup) Update(duration time.Duration, functionName string, podName string) {
	//podinfo := *((*l.shareInfos)[functionName])
	var sharepodInfo SharePodInfo
	sharepodInfo = (*l.shareInfos)[functionName]

	var podInfo PodInfo
	podInfo = (sharepodInfo.podInfos)[podName]

	//podInfo.totalInvoke++
	//time.Duration()
	var invoke_pre = podInfo.totalInvoke
	var invoke_cur = podInfo.totalInvoke + 1

	podInfo.avgResponseTime = (podInfo.avgResponseTime*(time.Duration(invoke_pre)) + duration) / time.Duration(invoke_cur)

	podInfo.totalInvoke++

	podInfo.rate = float32(1000 / podInfo.avgResponseTime.Milliseconds())

	sharepodInfo.podInfos[podName] = podInfo

	(*l.shareInfos)[functionName] = sharepodInfo
	return
}

func (l *FunctionLookup) Insert(shrName string, podName string, podIp string) {
	if sharepodInfo, ok := (*l.shareInfos)[shrName]; ok {
		if podInfo, ok2 := (sharepodInfo.podInfos)[podName]; ok2 {
			if podInfo.podIp == "" {
				podInfo.podIp = podIp
			}
		}
	} else {
		podinfos := make(map[string]PodInfo)
		podinfos[podName] = PodInfo{podName: podName, podIp: podIp, serviceName: shrName, totalInvoke: 0, rate: 0}
		(*l.shareInfos)[shrName] = SharePodInfo{podInfos: podinfos}
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
