package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

type MetricsOptions struct {
	PodResponseTime *prometheus.Histogram
}

func ExecutionTimer() {

}
