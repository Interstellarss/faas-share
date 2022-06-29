package devicemanager

import (
	"strconv"

	faasv1 "github.com/Interstellarss/faas-share/pkg/apis/faas_share/v1"
)

const (
	LabelMinReplicas = "com.openfaas.scale.min"
)

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
