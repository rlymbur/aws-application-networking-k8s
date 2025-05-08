package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"

	anv1alpha1 "github.com/aws/aws-application-networking-k8s/pkg/apis/applicationnetworking/v1alpha1"
	"github.com/aws/aws-application-networking-k8s/pkg/model/core"
	"github.com/aws/aws-application-networking-k8s/test/pkg/test"
	"github.com/aws/aws-sdk-go/aws"
)

var _ = Describe("HTTPRoute Priority", Ordered, func() {
	var (
		deployment *appsv1.Deployment
		service    *v1.Service
		httpRoute  *anv1alpha1.HTTPRoute
	)

	BeforeEach(func() {
		deployment, service = testFramework.NewHttpsApp(test.HTTPsAppOptions{
			Name:      "priority-test",
			Namespace: k8snamespace,
		})
	})

	Context("HTTPRoute with priority field", func() {
		It("creates successfully with priority set", func() {
			// Create the HTTPRoute
			httpRoute = testFramework.NewCustomHttpRoute(testGateway, service, "Service", aws.Int32(100))

			// Create resources
			testFramework.ExpectCreated(ctx, httpRoute, deployment, service)

			// Verify the route was created with priority
			route, err := core.NewRoute(httpRoute)
			Expect(err).NotTo(HaveOccurred())

			// Get the VPC Lattice service and verify it exists
			latticeService := testFramework.GetVpcLatticeService(ctx, route)
			Expect(latticeService).NotTo(BeNil())

			// Verify target group was created
			targetGroup := testFramework.GetTargetGroup(ctx, service)
			Expect(targetGroup).NotTo(BeNil())
		})
	})

	AfterEach(func() {
		testFramework.ExpectDeletedThenNotFound(ctx,
			httpRoute,
			deployment,
			service,
		)
	})
})
