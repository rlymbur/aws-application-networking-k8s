package integration

import (
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/vpclattice"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/aws-application-networking-k8s/test/pkg/test"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	anv1alpha1 "github.com/aws/aws-application-networking-k8s/pkg/apis/applicationnetworking/v1alpha1"
	model "github.com/aws/aws-application-networking-k8s/pkg/model/lattice"
)

var _ = Describe("GRPC ServiceExport Test", Ordered, func() {
	var (
		grpcDeployment *appsv1.Deployment
		grpcSvc        *corev1.Service
		serviceExport  *anv1alpha1.ServiceExport
		policy         *anv1alpha1.TargetGroupPolicy
	)

	It("Create k8s resources", func() {
		// Create GRPC service and deployment
		grpcDeployment, grpcSvc = testFramework.NewGrpcApp(test.GrpcAppOptions{Name: "my-grpc-1", Namespace: k8snamespace})
		policy = createGRPCTargetGroupPolicy(grpcSvc)
		testFramework.ExpectCreated(ctx, policy)
		serviceExport = testFramework.CreateServiceExport(grpcSvc)
		testFramework.ExpectCreated(ctx, serviceExport)

		// Create Kubernetes API Objects
		testFramework.ExpectCreated(ctx,
			grpcSvc,
			grpcDeployment,
		)
	})

	It("Verify lattice resources", func() {
		// Get both HTTP and GRPC target groups
		httpTG := testFramework.GetTargetGroupWithProtocol(ctx, grpcSvc, vpclattice.TargetGroupProtocolHttp, vpclattice.TargetGroupProtocolVersionHttp1)
		grpcTG := testFramework.GetTargetGroupWithProtocol(ctx, grpcSvc, vpclattice.TargetGroupProtocolHttp, vpclattice.TargetGroupProtocolVersionGrpc)

		// Verify HTTP target group
		httpTGDetails, err := testFramework.LatticeClient.GetTargetGroup(&vpclattice.GetTargetGroupInput{
			TargetGroupIdentifier: aws.String(*httpTG.Id),
		})
		Expect(httpTGDetails).To(Not(BeNil()))
		Expect(err).To(BeNil())
		Expect(*httpTG.VpcIdentifier).To(Equal(os.Getenv("CLUSTER_VPC_ID")))
		Expect(*httpTG.Protocol).To(Equal(vpclattice.TargetGroupProtocolHttp))

		// Get HTTP target group tags to verify protocol version
		httpTags, err := testFramework.LatticeClient.ListTagsForResource(&vpclattice.ListTagsForResourceInput{
			ResourceArn: httpTG.Arn,
		})
		Expect(err).To(BeNil())
		Expect(httpTags.Tags[model.K8SProtocolVersionKey]).To(Equal(aws.String(vpclattice.TargetGroupProtocolVersionHttp1)))

		// Verify GRPC target group
		grpcTGDetails, err := testFramework.LatticeClient.GetTargetGroup(&vpclattice.GetTargetGroupInput{
			TargetGroupIdentifier: aws.String(*grpcTG.Id),
		})
		Expect(grpcTGDetails).To(Not(BeNil()))
		Expect(err).To(BeNil())
		Expect(*grpcTG.VpcIdentifier).To(Equal(os.Getenv("CLUSTER_VPC_ID")))
		Expect(*grpcTG.Protocol).To(Equal(vpclattice.TargetGroupProtocolHttp))

		// Get GRPC target group tags to verify protocol version
		grpcTags, err := testFramework.LatticeClient.ListTagsForResource(&vpclattice.ListTagsForResourceInput{
			ResourceArn: grpcTG.Arn,
		})
		Expect(err).To(BeNil())
		Expect(grpcTags.Tags[model.K8SProtocolVersionKey]).To(Equal(aws.String(vpclattice.TargetGroupProtocolVersionGrpc)))

		// Verify targets are healthy for both target groups
		Eventually(func(g Gomega) {
			// Check HTTP target group targets
			httpTargets := testFramework.GetTargets(ctx, httpTG, grpcDeployment)
			for _, target := range httpTargets {
				g.Expect(*target.Port).To(BeEquivalentTo(grpcSvc.Spec.Ports[0].TargetPort.IntVal))
				g.Expect(*target.Status).To(Equal(vpclattice.TargetStatusHealthy))
			}

			// Check GRPC target group targets
			grpcTargets := testFramework.GetTargets(ctx, grpcTG, grpcDeployment)
			for _, target := range grpcTargets {
				g.Expect(*target.Port).To(BeEquivalentTo(grpcSvc.Spec.Ports[0].TargetPort.IntVal))
				g.Expect(*target.Status).To(Equal(vpclattice.TargetStatusHealthy))
			}
		})
	})

	AfterAll(func() {
		testFramework.ExpectDeletedThenNotFound(ctx,
			grpcDeployment,
			grpcSvc,
			serviceExport,
			policy,
		)
	})
})

func createGRPCTargetGroupPolicy(
	service *corev1.Service,
) *anv1alpha1.TargetGroupPolicy {
	healthCheckProtocol := anv1alpha1.HealthCheckProtocol("HTTP")
	healthCheckProtocolVersion := anv1alpha1.HealthCheckProtocolVersion("HTTP2")
	return &anv1alpha1.TargetGroupPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind: "TargetGroupPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: service.Namespace,
			Name:      "grpc-policy",
		},
		Spec: anv1alpha1.TargetGroupPolicySpec{
			TargetRef: &v1alpha2.NamespacedPolicyTargetReference{
				Group: "application-networking.k8s.aws",
				Kind:  gwv1.Kind("ServiceExport"),
				Name:  gwv1.ObjectName(service.Name),
			},
			Protocol:        aws.String("HTTP"),
			ProtocolVersion: aws.String("HTTP2"),
			HealthCheck: &anv1alpha1.HealthCheckConfig{
				Enabled:         aws.Bool(true),
				Protocol:        &healthCheckProtocol,
				ProtocolVersion: &healthCheckProtocolVersion,
				Port:            aws.Int64(50051),
			},
		},
	}
}
