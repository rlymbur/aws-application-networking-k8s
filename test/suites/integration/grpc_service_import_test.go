package integration

import (
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/vpclattice"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/aws-application-networking-k8s/pkg/k8s"
	"github.com/aws/aws-application-networking-k8s/test/pkg/test"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	anv1alpha1 "github.com/aws/aws-application-networking-k8s/pkg/apis/applicationnetworking/v1alpha1"
	model "github.com/aws/aws-application-networking-k8s/pkg/model/lattice"
)

var _ = Describe("GRPC Service Import Test", Ordered, func() {
	var (
		grpcDeployment1 *appsv1.Deployment
		grpcSvc1        *corev1.Service
		grpcDeployment2 *appsv1.Deployment
		grpcSvc2        *corev1.Service
		grpcRoute       *gwv1.GRPCRoute
		serviceExport   *anv1alpha1.ServiceExport
		serviceImport   *anv1alpha1.ServiceImport
		policy1         *anv1alpha1.TargetGroupPolicy
		policy2         *anv1alpha1.TargetGroupPolicy
	)

	It("Create k8s resources", func() {
		// Create GRPC services and deployments
		grpcDeployment1, grpcSvc1 = testFramework.NewGrpcApp(test.GrpcAppOptions{Name: "grpc-service-01", Namespace: k8snamespace})
		grpcDeployment2, grpcSvc2 = testFramework.NewGrpcApp(test.GrpcAppOptions{Name: "grpc-service-02", Namespace: k8snamespace})

		// Create Kubernetes API Objects
		testFramework.ExpectCreated(ctx,
			grpcSvc1,
			grpcDeployment1,
			grpcSvc2,
			grpcDeployment2,
		)

		// Create ServiceExport and ServiceImport
		serviceExport = &anv1alpha1.ServiceExport{
			ObjectMeta: metav1.ObjectMeta{
				Name:      grpcSvc2.Name,
				Namespace: grpcSvc2.Namespace,
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

		// Create ServiceImport
		serviceImport = testFramework.CreateServiceImport(grpcSvc2)
		testFramework.ExpectCreated(ctx, serviceImport)

		// Create TargetGroupPolicies
		policy1 = test.CreateGRPCTargetGroupPolicy(grpcSvc1)
		testFramework.ExpectCreated(ctx, policy1)
		policy2 = test.CreateGRPCTargetGroupPolicy(grpcSvc2)
		testFramework.ExpectCreated(ctx, policy2)

		// Create GRPCRoute with weighted distribution
		grpcRoute = &gwv1.GRPCRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "grpc-service-import",
				Namespace: k8snamespace,
			},
			Spec: gwv1.GRPCRouteSpec{
				CommonRouteSpec: gwv1.CommonRouteSpec{
					ParentRefs: []gwv1.ParentReference{
						{
							Name:        gwv1.ObjectName(testGateway.Name),
							SectionName: lo.ToPtr(gwv1.SectionName("https")),
						},
					},
				},
				Rules: []gwv1.GRPCRouteRule{
					{
						BackendRefs: []gwv1.GRPCBackendRef{
							{
								BackendRef: gwv1.BackendRef{
									BackendObjectReference: gwv1.BackendObjectReference{
										Name: gwv1.ObjectName(grpcSvc1.Name),
										Port: lo.ToPtr(gwv1.PortNumber(80)),
									},
									Weight: lo.ToPtr(int32(50)),
								},
							},
							{
								BackendRef: gwv1.BackendRef{
									BackendObjectReference: gwv1.BackendObjectReference{
										Name: gwv1.ObjectName(grpcSvc2.Name),
										Kind: lo.ToPtr(gwv1.Kind("ServiceImport")),
										Port: lo.ToPtr(gwv1.PortNumber(80)),
									},
									Weight: lo.ToPtr(int32(50)),
								},
							},
						},
					},
				},
			},
		}

		// Create GRPCRoute
		testFramework.ExpectCreated(ctx, grpcRoute)
	})

	It("Verify lattice resources", func() {
		// Get GRPC target groups for both services
		localTG := testFramework.GetTargetGroupWithProtocol(ctx, grpcSvc1, vpclattice.TargetGroupProtocolHttp, vpclattice.TargetGroupProtocolVersionGrpc)
		importedTG := testFramework.GetTargetGroupWithProtocol(ctx, grpcSvc2, vpclattice.TargetGroupProtocolHttp, vpclattice.TargetGroupProtocolVersionGrpc)

		// Verify local service target group
		localTGDetails, err := testFramework.LatticeClient.GetTargetGroup(&vpclattice.GetTargetGroupInput{
			TargetGroupIdentifier: aws.String(*localTG.Id),
		})
		Expect(localTGDetails).To(Not(BeNil()))
		Expect(err).To(BeNil())
		Expect(*localTG.VpcIdentifier).To(Equal(os.Getenv("CLUSTER_VPC_ID")))
		Expect(*localTG.Protocol).To(Equal(vpclattice.TargetGroupProtocolHttp))

		// Get local target group tags to verify protocol version
		localTags, err := testFramework.LatticeClient.ListTagsForResource(&vpclattice.ListTagsForResourceInput{
			ResourceArn: localTG.Arn,
		})
		Expect(err).To(BeNil())
		Expect(localTags.Tags[model.K8SProtocolVersionKey]).To(Equal(aws.String(vpclattice.TargetGroupProtocolVersionGrpc)))

		// Verify imported service target group
		importedTGDetails, err := testFramework.LatticeClient.GetTargetGroup(&vpclattice.GetTargetGroupInput{
			TargetGroupIdentifier: aws.String(*importedTG.Id),
		})
		Expect(importedTGDetails).To(Not(BeNil()))
		Expect(err).To(BeNil())
		Expect(*importedTG.VpcIdentifier).To(Equal(os.Getenv("CLUSTER_VPC_ID")))
		Expect(*importedTG.Protocol).To(Equal(vpclattice.TargetGroupProtocolHttp))

		// Get imported target group tags to verify protocol version
		importedTags, err := testFramework.LatticeClient.ListTagsForResource(&vpclattice.ListTagsForResourceInput{
			ResourceArn: importedTG.Arn,
		})
		Expect(err).To(BeNil())
		Expect(importedTags.Tags[model.K8SProtocolVersionKey]).To(Equal(aws.String(vpclattice.TargetGroupProtocolVersionGrpc)))

		// Verify targets are healthy
		Eventually(func(g Gomega) {
			// Check local service targets
			localTargets := testFramework.GetTargets(ctx, localTG, grpcDeployment1)
			for _, target := range localTargets {
				g.Expect(*target.Port).To(BeEquivalentTo(grpcSvc1.Spec.Ports[0].TargetPort.IntVal))
				g.Expect(*target.Status).To(Equal(vpclattice.TargetStatusHealthy))
			}

			// Check imported service targets
			importedTargets := testFramework.GetTargets(ctx, importedTG, grpcDeployment2)
			for _, target := range importedTargets {
				g.Expect(*target.Port).To(BeEquivalentTo(grpcSvc2.Spec.Ports[0].TargetPort.IntVal))
				g.Expect(*target.Status).To(Equal(vpclattice.TargetStatusHealthy))
			}
		})
	})

	AfterAll(func() {
		testFramework.ExpectDeletedThenNotFound(ctx,
			grpcRoute,
			grpcDeployment1,
			grpcSvc1,
			grpcDeployment2,
			grpcSvc2,
			serviceImport,
			serviceExport,
			policy1,
			policy2,
		)
	})
})
