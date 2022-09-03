package k8s

import (
	"sync"
	"time"
)

type PodInfo struct {
	PodName          string
	PodIp            string
	ServiceName      string
	AvgResponseTime  time.Duration
	LastResponseTime time.Duration
	TotalInvoke      int32
	LastInvoke       time.Time
	Rate             float32
	//
	RateChange ChangeType
}

type SharePodInfo struct {
	PodInfos map[string]PodInfo

	ScaleDown bool
	//todo make thread safe
	//Lock sync.RWMutex
	Lock sync.Mutex
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
