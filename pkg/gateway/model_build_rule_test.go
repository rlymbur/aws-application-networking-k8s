package gateway

import (
	"context"
	"fmt"
	"testing"

	anv1alpha1 "github.com/aws/aws-application-networking-k8s/pkg/apis/applicationnetworking/v1alpha1"
	"github.com/aws/aws-application-networking-k8s/pkg/k8s"
	"github.com/aws/aws-application-networking-k8s/pkg/model/core"
	model "github.com/aws/aws-application-networking-k8s/pkg/model/lattice"
	"github.com/aws/aws-application-networking-k8s/pkg/utils/gwlog"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apimachineryv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type dummyTgBuilder struct {
	i int
}

func (d *dummyTgBuilder) Build(ctx context.Context, route core.Route, backendRef core.BackendRef, stack core.Stack) (core.Stack, *model.TargetGroup, error) {
	if backendRef.Name() == "invalid" {
		return stack, nil, &InvalidBackendRefError{
			BackendRef: backendRef,
			Reason:     "not valid",
		}
	}

	// just need to provide a TG with an ID
	id := fmt.Sprintf("tg-%d", d.i)
	d.i++

	tg := &model.TargetGroup{
		ResourceMeta: core.NewResourceMeta(stack, "AWS:VPCServiceNetwork::TargetGroup", id),
	}
	stack.AddResource(tg)
	return stack, tg, nil
}

func Test_RuleModelBuild(t *testing.T) {
	var httpSectionName gwv1.SectionName = "http"
	var serviceKind gwv1.Kind = "Service"
	var serviceImportKind gwv1.Kind = "ServiceImport"
	var weight1 = int32(10)
	var weight2 = int32(90)

	var backendRef1 = gwv1.BackendRef{
		BackendObjectReference: gwv1.BackendObjectReference{
			Name: "targetgroup1",
			Kind: &serviceKind,
		},
		Weight: &weight1,
	}
	var backendRef2 = gwv1.BackendRef{
		BackendObjectReference: gwv1.BackendObjectReference{
			Name: "targetgroup2",
			Kind: &serviceImportKind,
		},
		Weight: &weight2,
	}

	tests := []struct {
		name         string
		route        core.Route
		wantErrIsNil bool
		expectedSpec []model.RuleSpec
	}{
		{
			name:         "rule with explicit priority",
			wantErrIsNil: true,
			route: core.NewHTTPRoute(gwv1.HTTPRoute{
				ObjectMeta: apimachineryv1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: gwv1.HTTPRouteSpec{
					CommonRouteSpec: gwv1.CommonRouteSpec{
						ParentRefs: []gwv1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: convertToGatewayRules([]anv1alpha1.HTTPRouteRule{
						{
							HTTPRouteRule: gwv1.HTTPRouteRule{
								BackendRefs: []gwv1.HTTPBackendRef{
									{
										BackendRef: backendRef1,
									},
								},
							},
							Priority: intPtr(50),
						},
					}),
				},
			}),
			expectedSpec: []model.RuleSpec{
				{
					StackListenerId: "listener-id",
					PathMatchPrefix: true,
					PathMatchValue:  "/",
					Priority:        50,
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								StackTargetGroupId: "tg-0",
								Weight:             int64(weight1),
							},
						},
					},
				},
			},
		},
		{
			name:         "multiple rules with explicit priorities",
			wantErrIsNil: true,
			route: core.NewHTTPRoute(gwv1.HTTPRoute{
				ObjectMeta: apimachineryv1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: gwv1.HTTPRouteSpec{
					CommonRouteSpec: gwv1.CommonRouteSpec{
						ParentRefs: []gwv1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: convertToGatewayRules([]anv1alpha1.HTTPRouteRule{
						{
							HTTPRouteRule: gwv1.HTTPRouteRule{
								BackendRefs: []gwv1.HTTPBackendRef{
									{
										BackendRef: backendRef1,
									},
								},
							},
							Priority: intPtr(50),
						},
						{
							HTTPRouteRule: gwv1.HTTPRouteRule{
								BackendRefs: []gwv1.HTTPBackendRef{
									{
										BackendRef: backendRef2,
									},
								},
							},
							Priority: intPtr(1),
						},
					}),
				},
			}),
			expectedSpec: []model.RuleSpec{
				{
					StackListenerId: "listener-id",
					PathMatchPrefix: true,
					PathMatchValue:  "/",
					Priority:        50,
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								StackTargetGroupId: "tg-0",
								Weight:             int64(weight1),
							},
						},
					},
				},
				{
					StackListenerId: "listener-id",
					PathMatchPrefix: true,
					PathMatchValue:  "/",
					Priority:        1,
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								SvcImportTG: &model.SvcImportTargetGroup{
									K8SServiceName:      string(backendRef2.Name),
									K8SServiceNamespace: "default",
								},
								Weight: int64(weight2),
							},
						},
					},
				},
			},
		},
		{
			name:         "mixed rules with and without priorities",
			wantErrIsNil: true,
			route: core.NewHTTPRoute(gwv1.HTTPRoute{
				ObjectMeta: apimachineryv1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: gwv1.HTTPRouteSpec{
					CommonRouteSpec: gwv1.CommonRouteSpec{
						ParentRefs: []gwv1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: convertToGatewayRules([]anv1alpha1.HTTPRouteRule{
						{
							HTTPRouteRule: gwv1.HTTPRouteRule{
								BackendRefs: []gwv1.HTTPBackendRef{
									{
										BackendRef: backendRef1,
									},
								},
							},
							Priority: intPtr(100),
						},
						{
							HTTPRouteRule: gwv1.HTTPRouteRule{
								BackendRefs: []gwv1.HTTPBackendRef{
									{
										BackendRef: backendRef2,
									},
								},
							},
						},
					}),
				},
			}),
			expectedSpec: []model.RuleSpec{
				{
					StackListenerId: "listener-id",
					PathMatchPrefix: true,
					PathMatchValue:  "/",
					Priority:        100,
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								StackTargetGroupId: "tg-0",
								Weight:             int64(weight1),
							},
						},
					},
				},
				{
					StackListenerId: "listener-id",
					PathMatchPrefix: true,
					PathMatchValue:  "/",
					Priority:        101,
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								SvcImportTG: &model.SvcImportTargetGroup{
									K8SServiceName:      string(backendRef2.Name),
									K8SServiceNamespace: "default",
								},
								Weight: int64(weight2),
							},
						},
					},
				},
			},
		},
		{
			name:         "priority out of range",
			wantErrIsNil: false,
			route: core.NewHTTPRoute(gwv1.HTTPRoute{
				ObjectMeta: apimachineryv1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: gwv1.HTTPRouteSpec{
					CommonRouteSpec: gwv1.CommonRouteSpec{
						ParentRefs: []gwv1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: convertToGatewayRules([]anv1alpha1.HTTPRouteRule{
						{
							HTTPRouteRule: gwv1.HTTPRouteRule{
								BackendRefs: []gwv1.HTTPBackendRef{
									{
										BackendRef: backendRef1,
									},
								},
							},
							Priority: intPtr(101),
						},
					}),
				},
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := gomock.NewController(t)
			defer c.Finish()
			ctx := context.TODO()

			k8sSchema := runtime.NewScheme()
			k8sSchema.AddKnownTypes(anv1alpha1.SchemeGroupVersion, &anv1alpha1.ServiceImport{})
			clientgoscheme.AddToScheme(k8sSchema)
			k8sClient := testclient.NewClientBuilder().WithScheme(k8sSchema).Build()

			svc := corev1.Service{
				ObjectMeta: apimachineryv1.ObjectMeta{
					Name:      string(backendRef1.Name),
					Namespace: "default",
				},
				Status: corev1.ServiceStatus{},
			}
			assert.NoError(t, k8sClient.Create(ctx, svc.DeepCopy()))
			stack := core.NewDefaultStack(core.StackID(k8s.NamespacedName(tt.route.K8sObject())))

			task := &latticeServiceModelBuildTask{
				log:         gwlog.FallbackLogger,
				route:       tt.route,
				stack:       stack,
				client:      k8sClient,
				brTgBuilder: &dummyTgBuilder{},
			}

			err := task.buildRules(ctx, "listener-id")
			if tt.wantErrIsNil {
				assert.NoError(t, err)
			} else {
				assert.NotNil(t, err)
				return
			}

			var resRules []*model.Rule
			stack.ListResources(&resRules)

			validateEqual(t, tt.expectedSpec, resRules)
		})
	}
}

func validateEqual(t *testing.T, expectedRules []model.RuleSpec, actualRules []*model.Rule) {
	assert.Equal(t, len(expectedRules), len(actualRules))
	assert.Equal(t, len(expectedRules), len(actualRules))

	for i, expectedSpec := range expectedRules {
		actualRule := actualRules[i]

		assert.Equal(t, expectedSpec.StackListenerId, actualRule.Spec.StackListenerId)
		assert.Equal(t, expectedSpec.PathMatchValue, actualRule.Spec.PathMatchValue)
		assert.Equal(t, expectedSpec.PathMatchPrefix, actualRule.Spec.PathMatchPrefix)
		assert.Equal(t, expectedSpec.PathMatchExact, actualRule.Spec.PathMatchExact)
		assert.Equal(t, expectedSpec.Method, actualRule.Spec.Method)
		assert.Equal(t, expectedSpec.Priority, actualRule.Spec.Priority)

		assert.Equal(t, len(expectedSpec.Action.TargetGroups), len(actualRule.Spec.Action.TargetGroups))
		for j, etg := range expectedSpec.Action.TargetGroups {
			atg := actualRule.Spec.Action.TargetGroups[j]

			assert.Equal(t, etg.Weight, atg.Weight)
			assert.Equal(t, etg.StackTargetGroupId, atg.StackTargetGroupId)
			assert.Equal(t, etg.SvcImportTG, etg.SvcImportTG)
		}
	}
}

func intPtr(i int32) *int32 {
	return &i
}

func convertToGatewayRules(rules []anv1alpha1.HTTPRouteRule) []gwv1.HTTPRouteRule {
	result := make([]gwv1.HTTPRouteRule, len(rules))
	for i, rule := range rules {
		result[i] = rule.HTTPRouteRule
		if rule.Priority != nil {
			headerType := gwv1.HeaderMatchExact
			priorityStr := fmt.Sprintf("%d", *rule.Priority)
			result[i].Matches = append(result[i].Matches, gwv1.HTTPRouteMatch{
				Headers: []gwv1.HTTPHeaderMatch{
					{
						Type:  &headerType,
						Name:  "x-lattice-rule-priority",
						Value: priorityStr,
					},
				},
			})
		}
	}
	return result
}
