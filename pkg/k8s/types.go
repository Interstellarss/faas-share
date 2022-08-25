package k8s

import (
	"sync"
	"time"
)

type PodInfo struct {
	podName          string
	podIp            string
	serviceName      string
	avgResponseTime  time.Duration
	lastResponseTime time.Duration
	totalInvoke      int64
	lastInvoke       time.Time
	rate             float32
	//
	rateChange ChangeType
}

type SharePodInfo struct {
	podInfos map[string]PodInfo

	//todo make thread safe
	lock sync.RWMutex
}

//may not defined here
type ContainerPool struct {
	shrName     string
	containerId string
	podName     string
}

type ChangeType int

const (
	Inc ChangeType = 2
	Dec ChangeType = 0
	Sta ChangeType = 1
)
