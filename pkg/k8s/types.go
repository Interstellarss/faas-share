package k8s

import (
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
}

type SharePodInfo struct {
	podInfos map[string]PodInfo
}

//may not defined here
type ContainerPool struct {
	shrName     string
	containerId string
	podName     string
}
