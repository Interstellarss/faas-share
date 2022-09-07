package devicemanager

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	faasv1 "github.com/Interstellarss/faas-share/pkg/apis/faasshare/v1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/apis/core/helper"
	"k8s.io/kubernetes/pkg/features"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"

	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"k8s.io/utils/integer"

	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"

	"math/rand"
)

const (
	LabelMinReplicas = "com.openfaas.scale.min"
)

var (
	//KeyFunc           = cache.DeletionHandlingMetaNamespaceKeyFunc
	podPhaseToOrdinal = map[v1.PodPhase]int{v1.PodPending: 0, v1.PodUnknown: 1, v1.PodRunning: 2}
)

// ActivePodsWithRanks is a sortable list of pods and a list of corresponding
// ranks which will be considered during sorting.  The two lists must have equal
// length.  After sorting, the pods will be ordered as follows, applying each
// rule in turn until one matches:
//
// 1. If only one of the pods is assigned to a node, the pod that is not
//    assigned comes before the pod that is.
// 2. If the pods' phases differ, a pending pod comes before a pod whose phase
//    is unknown, and a pod whose phase is unknown comes before a running pod.
// 3. If exactly one of the pods is ready, the pod that is not ready comes
//    before the ready pod.
// 4. If controller.kubernetes.io/pod-deletion-cost annotation is set, then
//    the pod with the lower value will come first.
// 5. If the pods' ranks differ, the pod with greater rank comes before the pod
//    with lower rank.
// 6. If both pods are ready but have not been ready for the same amount of
//    time, the pod that has been ready for a shorter amount of time comes
//    before the pod that has been ready for longer.
// 7. If one pod has a container that has restarted more than any container in
//    the other pod, the pod with the container with more restarts comes
//    before the other pod.
// 8. If the pods' creation times differ, the pod that was created more recently
//    comes before the older pod.
//
// In 6 and 8, times are compared in a logarithmic scale. This allows a level
// of randomness among equivalent Pods when sorting. If two pods have the same
// logarithmic rank, they are sorted by UUID to provide a pseudorandom order.
//
// If none of these rules matches, the second pod comes before the first pod.
//
// The intention of this ordering is to put pods that should be preferred for
// deletion first in the list.
type ActivePodsWithRanks struct {
	Pods []*v1.Pod

	// Rank is a ranking of pods.  This ranking is used during sorting when
	// comparing two pods that are both scheduled, in the same phase, and
	// having the same ready status.
	Rank []int
	// Now is a reference timestamp for doing logarithmic timestamp comparisons.
	// If zero, comparison happens without scaling.
	Now metav1.Time
}

func (s ActivePodsWithRanks) Len() int {
	return len(s.Pods)
}

func (s ActivePodsWithRanks) Swap(i, j int) {
	s.Pods[i], s.Pods[j] = s.Pods[j], s.Pods[i]
	s.Rank[i], s.Rank[j] = s.Rank[j], s.Rank[i]
}

// Less compares two pods with corresponding ranks and returns true if the first
// one should be preferred for deletion.
func (s ActivePodsWithRanks) Less(i, j int) bool {
	// 1. Unassigned < assigned
	// If only one of the pods is unassigned, the unassigned one is smaller
	if s.Pods[i].Spec.NodeName != s.Pods[j].Spec.NodeName && (len(s.Pods[i].Spec.NodeName) == 0 || len(s.Pods[j].Spec.NodeName) == 0) {
		return len(s.Pods[i].Spec.NodeName) == 0
	}
	// 2. PodPending < PodUnknown < PodRunning
	if podPhaseToOrdinal[s.Pods[i].Status.Phase] != podPhaseToOrdinal[s.Pods[j].Status.Phase] {
		return podPhaseToOrdinal[s.Pods[i].Status.Phase] < podPhaseToOrdinal[s.Pods[j].Status.Phase]
	}
	// 3. Not ready < ready
	// If only one of the pods is not ready, the not ready one is smaller
	if podutil.IsPodReady(s.Pods[i]) != podutil.IsPodReady(s.Pods[j]) {
		return !podutil.IsPodReady(s.Pods[i])
	}

	// 4. lower pod-deletion-cost < higher pod-deletion cost
	if utilfeature.DefaultFeatureGate.Enabled(features.PodDeletionCost) {
		pi, _ := helper.GetDeletionCostFromPodAnnotations(s.Pods[i].Annotations)
		pj, _ := helper.GetDeletionCostFromPodAnnotations(s.Pods[j].Annotations)
		if pi != pj {
			return pi < pj
		}
	}

	// 5. Doubled up < not doubled up
	// If one of the two pods is on the same node as one or more additional
	// ready pods that belong to the same replicaset, whichever pod has more
	// colocated ready pods is less
	if s.Rank[i] != s.Rank[j] {
		return s.Rank[i] > s.Rank[j]
	}
	// TODO: take availability into account when we push minReadySeconds information from deployment into pods,
	//       see https://github.com/kubernetes/kubernetes/issues/22065
	// 6. Been ready for empty time < less time < more time
	// If both pods are ready, the latest ready one is smaller
	if podutil.IsPodReady(s.Pods[i]) && podutil.IsPodReady(s.Pods[j]) {
		readyTime1 := podReadyTime(s.Pods[i])
		readyTime2 := podReadyTime(s.Pods[j])
		if !readyTime1.Equal(readyTime2) {
			if !utilfeature.DefaultFeatureGate.Enabled(features.LogarithmicScaleDown) {
				return afterOrZero(readyTime1, readyTime2)
			} else {
				if s.Now.IsZero() || readyTime1.IsZero() || readyTime2.IsZero() {
					return afterOrZero(readyTime1, readyTime2)
				}
				rankDiff := logarithmicRankDiff(*readyTime1, *readyTime2, s.Now)
				if rankDiff == 0 {
					return s.Pods[i].UID < s.Pods[j].UID
				}
				return rankDiff < 0
			}
		}
	}
	// 7. Pods with containers with higher restart counts < lower restart counts
	if maxContainerRestarts(s.Pods[i]) != maxContainerRestarts(s.Pods[j]) {
		return maxContainerRestarts(s.Pods[i]) > maxContainerRestarts(s.Pods[j])
	}
	// 8. Empty creation time pods < newer pods < older pods
	if !s.Pods[i].CreationTimestamp.Equal(&s.Pods[j].CreationTimestamp) {
		if !utilfeature.DefaultFeatureGate.Enabled(features.LogarithmicScaleDown) {
			return afterOrZero(&s.Pods[i].CreationTimestamp, &s.Pods[j].CreationTimestamp)
		} else {
			if s.Now.IsZero() || s.Pods[i].CreationTimestamp.IsZero() || s.Pods[j].CreationTimestamp.IsZero() {
				return afterOrZero(&s.Pods[i].CreationTimestamp, &s.Pods[j].CreationTimestamp)
			}
			rankDiff := logarithmicRankDiff(s.Pods[i].CreationTimestamp, s.Pods[j].CreationTimestamp, s.Now)
			if rankDiff == 0 {
				return s.Pods[i].UID < s.Pods[j].UID
			}
			return rankDiff < 0
		}
	}
	return false
}

func logarithmicRankDiff(t1, t2, now metav1.Time) int64 {
	d1 := now.Sub(t1.Time)
	d2 := now.Sub(t2.Time)
	r1 := int64(-1)
	r2 := int64(-1)
	if d1 > 0 {
		r1 = int64(math.Log2(float64(d1)))
	}
	if d2 > 0 {
		r2 = int64(math.Log2(float64(d2)))
	}
	return r1 - r2
}

func maxContainerRestarts(pod *v1.Pod) int {
	maxRestarts := 0
	for _, c := range pod.Status.ContainerStatuses {
		maxRestarts = integer.IntMax(maxRestarts, int(c.RestartCount))
	}
	return maxRestarts
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
	//minrep := *minReplicas

	//minrep = minrep - 1

	//minReplicas = &minrep
	return minReplicas
}

func int32p(i int32) *int32 {
	return &i
}

//more percise filter mechanism for pre-warm pod
func FilterActivePods(pods []*v1.Pod) []*v1.Pod {
	var result []*v1.Pod
	for _, p := range pods {
		if IsPodActive(p) {
			result = append(result, p)
		} else {
			klog.V(4).Infof("Ignoring inactive pod %v/%v in state %v, deletion time %v", p.Namespace, p.Name, p.Status.Phase, p.DeletionTimestamp)
		}
	}
	return result
}

func IsPodActive(p *v1.Pod) bool {
	return v1.PodSucceeded != p.Status.Phase && v1.PodFailed != p.Status.Phase && p.DeletionTimestamp == nil
}

func IsPodHot(p *v1.Pod) bool {
	return p.Annotations[FaasShareWarm] != "true"
}

//do we really need this?
func podReadyTime(pod *v1.Pod) *metav1.Time {
	if podutil.IsPodReady(pod) {
		for _, c := range pod.Status.Conditions {
			if c.Type == v1.PodReady && c.Status == v1.ConditionTrue {
				return &c.LastTransitionTime
			}
		}
	}
	return &metav1.Time{}
}

//Idea is to delete pods on the node that is minimum across all filtered pods
func getPodsToDelete(filteredPods []*v1.Pod, diff int) []*v1.Pod {

	if diff < len(filteredPods) {
		podsWithRanks := getPodsRankedByRelatedPodsOnSameNode(filteredPods)
		sort.Sort(podsWithRanks)
		//

	}
	return filteredPods[:diff]
}

func afterOrZero(t1, t2 *metav1.Time) bool {
	if t1.Time.IsZero() || t2.Time.IsZero() {
		return t1.Time.IsZero()
	}
	return t1.After(t2.Time)
}

func getPodsRankedByRelatedPodsOnSameNode(podsToRank []*v1.Pod) ActivePodsWithRanks {
	podsOnNode := make(map[string]int)
	for _, pod := range podsToRank {
		if IsPodActive(pod) {
			podsOnNode[pod.Spec.NodeName]++
		}
	}

	ranks := make([]int, len(podsToRank))

	for i, pod := range podsToRank {
		ranks[i] = podsOnNode[pod.Spec.NodeName]
	}
	return ActivePodsWithRanks{Pods: podsToRank, Rank: ranks, Now: metav1.Now()}
}

// PodKey returns a key unique to the given pod within a cluster.
// It's used so we consistently use the same key scheme in this module.
// It does exactly what cache.MetaNamespaceKeyFunc would have done
// except there's not possibility for error since we know the exact type.
func PodKey(pod *v1.Pod) string {
	return fmt.Sprintf("%v/%v", pod.Namespace, pod.Name)
}

func calculateStatus(shr *faasv1.SharePod, filteredPods []*v1.Pod, manageReplicasErr error) faasv1.SharePodStatus {
	newStatus := shr.Status

	readyReplicasCount := 0
	availableReplicasCount := 0

	for _, pod := range filteredPods {
		if podutil.IsPodReady(pod) {
			readyReplicasCount++
			//TODO is pod is available
			if podutil.IsPodAvailable(pod, 1, metav1.Now()) {
				availableReplicasCount++
			}
		}
	}
	/*
		if port != 0 {
			(*sharepodCopy.Status.PodManagerPort)[pod.Name] = port
		}
	*/

	//conditions missing?
	if manageReplicasErr != nil {
		var reason string
		if diff := len(filteredPods) - int(*(shr.Spec.Replicas)); diff < 0 {
			reason = "FailedCreate"

		} else if diff > 0 {
			reason = "FailedDelete"
		}
		klog.Info(reason)
		//Set condition
	}

	newStatus.Replicas = int32(len(filteredPods))
	newStatus.AvailableReplicas = int32(availableReplicasCount)
	newStatus.ReadyReplicas = int32(readyReplicasCount)

	return newStatus
}

type RealPodControl struct {
	KubeClient clientset.Interface
	Recorder   record.EventRecorder
}

func makeLabels(sharepod *faasv1.SharePod) map[string]string {
	labels := map[string]string{
		"faas_function": sharepod.Name,
		"app":           sharepod.Name,
		"controller":    sharepod.Name,
	}
	if sharepod.Labels != nil {
		for k, v := range sharepod.Labels {
			labels[k] = v
		}
	}

	return labels
}

func RandStr(length int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyz"
	bytes := []byte(str)
	result := []byte{}
	rand.Seed(time.Now().UnixNano() + int64(rand.Intn(100)))
	for i := 0; i < length; i++ {
		result = append(result, bytes[rand.Intn(len(bytes))])
	}
	return string(result)
}

type SharepodProbes struct {
	Liveness  *v1.Probe
	Readiness *v1.Probe
}

func makeSimpleProbes() (*SharepodProbes, error) {
	var handler v1.ProbeHandler
	path := filepath.Join("/tmp/", ".lock")
	handler = v1.ProbeHandler{
		Exec: &v1.ExecAction{
			Command: []string{"cat", path},
		},
	}
	probes := SharepodProbes{}
	probes.Readiness = &v1.Probe{
		ProbeHandler:        handler,
		InitialDelaySeconds: int32(2),
		TimeoutSeconds:      int32(1),
		PeriodSeconds:       int32(10),
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}

	probes.Liveness = &v1.Probe{
		ProbeHandler:        handler,
		InitialDelaySeconds: int32(2),
		TimeoutSeconds:      int32(1),
		PeriodSeconds:       int32(10),
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}

	return &probes, nil
}
