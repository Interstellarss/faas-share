package controller

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"

	faasv1 "github.com/Interstellarss/faas-share/pkg/apis/faasshare/v1"
)

// newService creates a new ClusterIP Service for a Function resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Function resource that 'owns' it.
func newService(sharepod *faasv1.SharePod) *corev1.Service {
        funcname, found := sharepod.Labels["faas_function"]
        if !found {
                funcname = sharepod.ObjectMeta.Name
        }
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      funcname,
			Namespace: sharepod.Namespace,
			//todo here: do we need to modify annotaions also through sharepod
			Annotations: map[string]string{"prometheus.io.scrape": "false"},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sharepod, schema.GroupVersionKind{
					Group:   faasv1.SchemeGroupVersion.Group,
					Version: faasv1.SchemeGroupVersion.Version,
					Kind:    faasKind,
				}),
			},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"faas_function": funcname},
			Ports: []corev1.ServicePort{
				{
					Name:     "http",
					Protocol: corev1.ProtocolTCP,
					Port:     functionPort,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: int32(functionPort),
					},
				},
			},
		},
	}
}
