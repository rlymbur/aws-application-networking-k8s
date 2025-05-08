package integration

import (
	"fmt"
	"github.com/aws/aws-application-networking-k8s/pkg/gateway/utils"
	"github.com/aws/aws-sdk-go/service/vpclattice"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"os"

	anv1alpha1 "github.com/aws/aws-application-networking-k8s/pkg/apis/applicationnetworking/v1alpha1"
	"github.com/aws/aws-application-networking-k8s/test/pkg/test"
	"github.com/aws/aws-sdk-go/aws"
)

var _ = Describe("HTTPRoute Priority", Ordered, func() {
	var (
		deployment *appsv1.Deployment
		service    *v1.Service
		httpRoute  *anv1alpha1.HTTPRoute
	)

	It("Set up k8s resource", func() {
		deployment, service = testFramework.NewHttpsApp(test.HTTPsAppOptions{
			Name:      "priority-test",
			Namespace: k8snamespace,
		})

		httpRoute = testFramework.NewCustomHttpRoute(testGateway, service, "Service", aws.Int32(100))
		testFramework.ExpectCreated(ctx, httpRoute, deployment, service)
	})

	It("Verify Lattice resource", func() {
		route := utils.ConvertV1Alpha1ToCore(httpRoute)
		vpcLatticeService := testFramework.GetVpcLatticeService(ctx, route)
		fmt.Printf("vpcLatticeService: %v \n", vpcLatticeService)
		tgSummary := testFramework.GetTargetGroup(ctx, service)
		tg, err := testFramework.LatticeClient.GetTargetGroup(&vpclattice.GetTargetGroupInput{
			TargetGroupIdentifier: aws.String(*tgSummary.Id),
		})
		Expect(err).To(BeNil())
		Expect(tg).NotTo(BeNil())
		Expect(*tgSummary.VpcIdentifier).To(Equal(os.Getenv("CLUSTER_VPC_ID")))
		Expect(*tgSummary.Protocol).To(Equal("TCP"))
		if tg.Config.HealthCheck != nil {
			Expect(*tg.Config.HealthCheck.Enabled).To(BeFalse())
		}
		targets := testFramework.GetTargets(ctx, tgSummary, deployment)
		for _, target := range targets {
			Expect(*target.Port).To(BeEquivalentTo(service.Spec.Ports[0].TargetPort.IntVal))
			Expect(*target.Status).To(Equal(vpclattice.TargetStatusUnavailable))
		}
	})

	AfterAll(func() {
		testFramework.ExpectDeletedThenNotFound(ctx,
			httpRoute,
			deployment,
			service,
		)
	})
})
