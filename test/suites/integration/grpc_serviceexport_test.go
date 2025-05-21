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

	anv1alpha1 "github.com/aws/aws-application-networking-k8s/pkg/apis/applicationnetworking/v1alpha1"
	"github.com/aws/aws-application-networking-k8s/pkg/k8s"
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

		// Create Kubernetes API Objects
		testFramework.ExpectCreated(ctx,
			grpcSvc,
			grpcDeployment,
		)

		// Create ServiceExport
		serviceExport = &anv1alpha1.ServiceExport{
			ObjectMeta: metav1.ObjectMeta{
				Name:      grpcSvc.Name,
				Namespace: grpcSvc.Namespace,
				Annotations: map[string]string{
					"application-networking.k8s.aws/federation": "amazon-vpc-lattice",
				},
			},
			Spec: anv1alpha1.ServiceExportSpec{
				ExportedPorts: []anv1alpha1.ExportedPort{
					{
						Port:      80,
						RouteType: "GRPC",
					},
				},
			},
		}
		testFramework.ExpectCreated(ctx, serviceExport)

		// Wait for ServiceExport to be reconciled
		Eventually(func(g Gomega) {
			err := testFramework.Get(ctx, k8s.NamespacedName(serviceExport), serviceExport)
			g.Expect(err).To(BeNil())
			g.Expect(serviceExport.Status.Conditions).ToNot(BeEmpty())
		}).Should(Succeed())

		// Create TargetGroupPolicy
		policy = test.CreateGRPCTargetGroupPolicy(grpcSvc)
		testFramework.ExpectCreated(ctx, policy)
	})

	It("Verify lattice resources", func() {
		// Get GRPC target group
		grpcTG := testFramework.GetTargetGroupWithProtocol(ctx, grpcSvc, vpclattice.TargetGroupProtocolHttp, vpclattice.TargetGroupProtocolVersionGrpc)

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

		// Verify targets are healthy
		Eventually(func(g Gomega) {
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
