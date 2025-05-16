package test

import (
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type GrpcAppOptions struct {
	AppName   string // Used by existing tests
	Name      string // Used by new tests
	Namespace string
	Replicas  *int32
	Port      *int32
}

func (o *GrpcAppOptions) GetName() string {
	if o.Name != "" {
		return o.Name
	}
	return o.AppName
}

func (o *GrpcAppOptions) GetPort() int32 {
	if o.Port == nil {
		return 50051 // Default GRPC port
	}
	return *o.Port
}

func (o *GrpcAppOptions) GetReplicas() *int32 {
	if o.Replicas == nil {
		return lo.ToPtr(int32(1))
	}
	return o.Replicas
}

// NewGrpcApp creates a new GRPC application using grpcbin image
func (env *Framework) NewGrpcApp(options GrpcAppOptions) (*appsv1.Deployment, *v1.Service) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      options.GetName(),
			Namespace: options.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: options.GetReplicas(),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":          options.GetName(),
					DiscoveryLabel: "true",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":          options.GetName(),
						DiscoveryLabel: "true",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "app",
							Image: "public.ecr.aws/a0j4q9e4/grpcbin:latest",
							Ports: []v1.ContainerPort{
								{
									Name:          "grpc",
									ContainerPort: options.GetPort(),
								},
							},
						},
					},
				},
			},
		},
	}

	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      options.GetName(),
			Namespace: options.Namespace,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:       "grpc",
					Port:       options.GetPort(),
					TargetPort: intstr.FromInt(int(options.GetPort())),
				},
			},
			Selector: map[string]string{
				"app": options.GetName(),
			},
		},
	}

	return deployment, service
}

// NewGrpcBin creates a new GRPC application using moul/grpcbin image
func (env *Framework) NewGrpcBin(options GrpcAppOptions) (*appsv1.Deployment, *v1.Service) {

	deployment := New(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      options.GetName(),
			Namespace: options.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: options.GetReplicas(),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": options.GetName(),
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":          options.GetName(),
						DiscoveryLabel: "true",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name:  options.GetName(),
						Image: "moul/grpcbin:latest",
					}},
				},
			},
		},
	})

	service := New(&v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind: "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      options.GetName(),
			Namespace: options.Namespace,
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"app": options.GetName(),
			},
			Ports: []v1.ServicePort{
				{
					Name:       "grpcbin-over-http",
					Protocol:   v1.ProtocolTCP,
					Port:       int32(19000),
					TargetPort: intstr.FromInt(9000),
				},
				{
					Name:       "grpcbin-over-https",
					Protocol:   v1.ProtocolTCP,
					Port:       int32(19001),
					TargetPort: intstr.FromInt(9001),
				},
			},
		},
	})
	return deployment, service
}
