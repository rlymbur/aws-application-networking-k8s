package test

import (
	"github.com/aws/aws-sdk-go/aws"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	anv1alpha1 "github.com/aws/aws-application-networking-k8s/pkg/apis/applicationnetworking/v1alpha1"
)

func CreateGRPCTargetGroupPolicy(
	service *corev1.Service,
) *anv1alpha1.TargetGroupPolicy {
	healthCheckProtocol := anv1alpha1.HealthCheckProtocol("HTTP")
	healthCheckProtocolVersion := anv1alpha1.HealthCheckProtocolVersion("HTTP1")
	return &anv1alpha1.TargetGroupPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind: "TargetGroupPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: service.Namespace,
			Name:      service.Name + "-policy",
		},
		Spec: anv1alpha1.TargetGroupPolicySpec{
			TargetRef: &v1alpha2.NamespacedPolicyTargetReference{
				Group: "",
				Kind:  gwv1.Kind("Service"),
				Name:  gwv1.ObjectName(service.Name),
			},
			Protocol:        aws.String("HTTP"),
			ProtocolVersion: aws.String("GRPC"),
			HealthCheck: &anv1alpha1.HealthCheckConfig{
				Enabled:         aws.Bool(true),
				Protocol:        &healthCheckProtocol,
				ProtocolVersion: &healthCheckProtocolVersion,
				Port:            aws.Int64(50051),
			},
		},
	}
}
