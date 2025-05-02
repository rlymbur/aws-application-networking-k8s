package gateway

import (
	"context"
	"testing"

	mock_client "github.com/aws/aws-application-networking-k8s/mocks/controller-runtime/client"
	"github.com/aws/aws-application-networking-k8s/pkg/model/core"
	model "github.com/aws/aws-application-networking-k8s/pkg/model/lattice"
	"github.com/aws/aws-application-networking-k8s/pkg/utils/gwlog"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

func TestBuildRulesPriority(t *testing.T) {
	tests := []struct {
		name         string
		rules        []gwv1.HTTPRouteRule
		wantErr      bool
		errContains  string
		wantPriority []int64
	}{
		{
			name: "single rule without priority",
			rules: []gwv1.HTTPRouteRule{
				{
					BackendRefs: []gwv1.HTTPBackendRef{
						{
							BackendRef: gwv1.BackendRef{
								BackendObjectReference: gwv1.BackendObjectReference{
									Name: "service1",
								},
							},
						},
					},
				},
			},
			wantPriority: []int64{1},
		},
		{
			name: "multiple rules without priority",
			rules: []gwv1.HTTPRouteRule{
				{
					BackendRefs: []gwv1.HTTPBackendRef{
						{
							BackendRef: gwv1.BackendRef{
								BackendObjectReference: gwv1.BackendObjectReference{
									Name: "service1",
								},
							},
						},
					},
				},
				{
					BackendRefs: []gwv1.HTTPBackendRef{
						{
							BackendRef: gwv1.BackendRef{
								BackendObjectReference: gwv1.BackendObjectReference{
									Name: "service2",
								},
							},
						},
					},
				},
			},
			wantPriority: []int64{1, 2},
		},
		{
			name: "rules with explicit priorities",
			rules: []gwv1.HTTPRouteRule{
				{
					BackendRefs: []gwv1.HTTPBackendRef{
						{
							BackendRef: gwv1.BackendRef{
								BackendObjectReference: gwv1.BackendObjectReference{
									Name: "service1",
								},
							},
						},
					},
					Matches: []gwv1.HTTPRouteMatch{
						{
							Headers: []gwv1.HTTPHeaderMatch{
								{
									Name:  "x-lattice-rule-priority",
									Value: "100",
								},
							},
						},
					},
				},
				{
					BackendRefs: []gwv1.HTTPBackendRef{
						{
							BackendRef: gwv1.BackendRef{
								BackendObjectReference: gwv1.BackendObjectReference{
									Name: "service2",
								},
							},
						},
					},
					Matches: []gwv1.HTTPRouteMatch{
						{
							Headers: []gwv1.HTTPHeaderMatch{
								{
									Name:  "x-lattice-rule-priority",
									Value: "50",
								},
							},
						},
					},
				},
			},
			wantPriority: []int64{100, 50},
		},
		{
			name: "mixed rules with and without priorities",
			rules: []gwv1.HTTPRouteRule{
				{
					BackendRefs: []gwv1.HTTPBackendRef{
						{
							BackendRef: gwv1.BackendRef{
								BackendObjectReference: gwv1.BackendObjectReference{
									Name: "service1",
								},
							},
						},
					},
					Matches: []gwv1.HTTPRouteMatch{
						{
							Headers: []gwv1.HTTPHeaderMatch{
								{
									Name:  "x-lattice-rule-priority",
									Value: "100",
								},
							},
						},
					},
				},
				{
					BackendRefs: []gwv1.HTTPBackendRef{
						{
							BackendRef: gwv1.BackendRef{
								BackendObjectReference: gwv1.BackendObjectReference{
									Name: "service2",
								},
							},
						},
					},
				},
			},
			wantPriority: []int64{100, 101},
		},
		{
			name: "invalid priority value",
			rules: []gwv1.HTTPRouteRule{
				{
					BackendRefs: []gwv1.HTTPBackendRef{
						{
							BackendRef: gwv1.BackendRef{
								BackendObjectReference: gwv1.BackendObjectReference{
									Name: "service1",
								},
							},
						},
					},
					Matches: []gwv1.HTTPRouteMatch{
						{
							Headers: []gwv1.HTTPHeaderMatch{
								{
									Name:  "x-lattice-rule-priority",
									Value: "invalid",
								},
							},
						},
					},
				},
			},
			wantPriority: []int64{1},
		},
		{
			name: "priority out of range",
			rules: []gwv1.HTTPRouteRule{
				{
					BackendRefs: []gwv1.HTTPBackendRef{
						{
							BackendRef: gwv1.BackendRef{
								BackendObjectReference: gwv1.BackendObjectReference{
									Name: "service1",
								},
							},
						},
					},
					Matches: []gwv1.HTTPRouteMatch{
						{
							Headers: []gwv1.HTTPHeaderMatch{
								{
									Name:  "x-lattice-rule-priority",
									Value: "101",
								},
							},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "priority must be between 1 and 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			c := gomock.NewController(t)
			defer c.Finish()
			ctx := context.TODO()

			k8sScheme := runtime.NewScheme()
			clientgoscheme.AddToScheme(k8sScheme)
			gwv1.Install(k8sScheme)

			route := &gwv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-route",
					Namespace: "default",
				},
				Spec: gwv1.HTTPRouteSpec{
					Rules: tt.rules,
				},
			}

			stack := core.NewDefaultStack(core.StackID(types.NamespacedName{
				Name:      route.Name,
				Namespace: route.Namespace,
			}))
			mockClient := mock_client.NewMockClient(c)
			mockBrTgBuilder := NewMockBackendRefTargetGroupModelBuilder(c)

			task := &latticeServiceModelBuildTask{
				log:         gwlog.FallbackLogger,
				route:       core.NewHTTPRoute(*route),
				stack:       stack,
				client:      mockClient,
				brTgBuilder: mockBrTgBuilder,
			}

			// Setup mock expectations
			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockBrTgBuilder.EXPECT().Build(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(stack, nil, nil).AnyTimes()

			// Execute
			err := task.buildRules(ctx, "listener-id")

			// Verify
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			assert.NoError(t, err)

			var rules []*model.Rule
			stack.ListResources(&rules)
			assert.Equal(t, len(tt.wantPriority), len(rules))

			for i, rule := range rules {
				assert.Equal(t, tt.wantPriority[i], rule.Spec.Priority)
			}
		})
	}
}

func TestBuildRulesPriorityGRPC(t *testing.T) {
	tests := []struct {
		name         string
		rules        []gwv1.GRPCRouteRule
		wantErr      bool
		errContains  string
		wantPriority []int64
	}{
		{
			name: "single rule without priority",
			rules: []gwv1.GRPCRouteRule{
				{
					BackendRefs: []gwv1.GRPCBackendRef{
						{
							BackendRef: gwv1.BackendRef{
								BackendObjectReference: gwv1.BackendObjectReference{
									Name: "service1",
								},
							},
						},
					},
				},
			},
			wantPriority: []int64{1},
		},
		{
			name: "rules with explicit priorities",
			rules: []gwv1.GRPCRouteRule{
				{
					BackendRefs: []gwv1.GRPCBackendRef{
						{
							BackendRef: gwv1.BackendRef{
								BackendObjectReference: gwv1.BackendObjectReference{
									Name: "service1",
								},
							},
						},
					},
					Matches: []gwv1.GRPCRouteMatch{
						{
							Headers: []gwv1.GRPCHeaderMatch{
								{
									Name:  "x-lattice-rule-priority",
									Value: "100",
								},
							},
						},
					},
				},
			},
			wantPriority: []int64{100},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			c := gomock.NewController(t)
			defer c.Finish()
			ctx := context.TODO()

			k8sScheme := runtime.NewScheme()
			clientgoscheme.AddToScheme(k8sScheme)
			gwv1.Install(k8sScheme)

			route := &gwv1.GRPCRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-route",
					Namespace: "default",
				},
				Spec: gwv1.GRPCRouteSpec{
					Rules: tt.rules,
				},
			}

			stack := core.NewDefaultStack(core.StackID(types.NamespacedName{
				Name:      route.Name,
				Namespace: route.Namespace,
			}))
			mockClient := mock_client.NewMockClient(c)
			mockBrTgBuilder := NewMockBackendRefTargetGroupModelBuilder(c)

			task := &latticeServiceModelBuildTask{
				log:         gwlog.FallbackLogger,
				route:       core.NewGRPCRoute(*route),
				stack:       stack,
				client:      mockClient,
				brTgBuilder: mockBrTgBuilder,
			}

			// Setup mock expectations
			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockBrTgBuilder.EXPECT().Build(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(stack, nil, nil).AnyTimes()

			// Execute
			err := task.buildRules(ctx, "listener-id")

			// Verify
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			assert.NoError(t, err)

			var rules []*model.Rule
			stack.ListResources(&rules)
			assert.Equal(t, len(tt.wantPriority), len(rules))

			for i, rule := range rules {
				assert.Equal(t, tt.wantPriority[i], rule.Spec.Priority)
			}
		})
	}
}

func TestBuildRulesPriorityTLS(t *testing.T) {
	tests := []struct {
		name         string
		rules        []gwv1alpha2.TLSRouteRule
		wantErr      bool
		errContains  string
		wantPriority []int64
	}{
		{
			name: "single rule without priority",
			rules: []gwv1alpha2.TLSRouteRule{
				{
					BackendRefs: []gwv1.BackendRef{
						{
							BackendObjectReference: gwv1.BackendObjectReference{
								Name: "service1",
							},
						},
					},
				},
			},
			wantPriority: []int64{1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			c := gomock.NewController(t)
			defer c.Finish()
			ctx := context.TODO()

			k8sScheme := runtime.NewScheme()
			clientgoscheme.AddToScheme(k8sScheme)
			gwv1.Install(k8sScheme)

			route := &gwv1alpha2.TLSRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-route",
					Namespace: "default",
				},
				Spec: gwv1alpha2.TLSRouteSpec{
					Rules: tt.rules,
				},
			}

			stack := core.NewDefaultStack(core.StackID(types.NamespacedName{
				Name:      route.Name,
				Namespace: route.Namespace,
			}))
			mockClient := mock_client.NewMockClient(c)
			mockBrTgBuilder := NewMockBackendRefTargetGroupModelBuilder(c)

			task := &latticeServiceModelBuildTask{
				log:         gwlog.FallbackLogger,
				route:       core.NewTLSRoute(*route),
				stack:       stack,
				client:      mockClient,
				brTgBuilder: mockBrTgBuilder,
			}

			// Setup mock expectations
			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockBrTgBuilder.EXPECT().Build(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(stack, nil, nil).AnyTimes()

			// Execute
			err := task.buildRules(ctx, "listener-id")

			// Verify
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			assert.NoError(t, err)

			var rules []*model.Rule
			stack.ListResources(&rules)
			assert.Equal(t, len(tt.wantPriority), len(rules))

			for i, rule := range rules {
				assert.Equal(t, tt.wantPriority[i], rule.Spec.Priority)
			}
		})
	}
}
