package utils

import (
	"testing"

	anv1alpha1 "github.com/aws/aws-application-networking-k8s/pkg/apis/applicationnetworking/v1alpha1"
	"github.com/aws/aws-application-networking-k8s/pkg/model/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestHasPriorityHeader(t *testing.T) {
	headerType := gwv1.HeaderMatchExact

	tests := []struct {
		name     string
		route    core.Route
		expected bool
	}{
		{
			name: "http route with priority header",
			route: core.NewHTTPRoute(gwv1.HTTPRoute{
				Spec: gwv1.HTTPRouteSpec{
					Rules: []gwv1.HTTPRouteRule{
						{
							Matches: []gwv1.HTTPRouteMatch{
								{
									Headers: []gwv1.HTTPHeaderMatch{
										{
											Type:  &headerType,
											Name:  "x-lattice-rule-priority",
											Value: "1",
										},
									},
								},
							},
						},
					},
				},
			}),
			expected: true,
		},
		{
			name: "route with different header",
			route: core.NewHTTPRoute(gwv1.HTTPRoute{
				Spec: gwv1.HTTPRouteSpec{
					Rules: []gwv1.HTTPRouteRule{
						{
							Matches: []gwv1.HTTPRouteMatch{
								{
									Headers: []gwv1.HTTPHeaderMatch{
										{
											Type:  &headerType,
											Name:  "other-header",
											Value: "value",
										},
									},
								},
							},
						},
					},
				},
			}),
			expected: false,
		},
		{
			name: "route with no headers",
			route: core.NewHTTPRoute(gwv1.HTTPRoute{
				Spec: gwv1.HTTPRouteSpec{
					Rules: []gwv1.HTTPRouteRule{
						{
							Matches: []gwv1.HTTPRouteMatch{
								{},
							},
						},
					},
				},
			}),
			expected: false,
		},
		{
			name: "route with no matches",
			route: core.NewHTTPRoute(gwv1.HTTPRoute{
				Spec: gwv1.HTTPRouteSpec{
					Rules: []gwv1.HTTPRouteRule{
						{},
					},
				},
			}),
			expected: false,
		},
		{
			name: "route with no rules",
			route: core.NewHTTPRoute(gwv1.HTTPRoute{
				Spec: gwv1.HTTPRouteSpec{},
			}),
			expected: false,
		},
		{
			name:     "nil route",
			route:    nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasPriorityHeader(tt.route)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsCustomHTTPRoute(t *testing.T) {
	tests := []struct {
		name     string
		obj      interface{}
		expected bool
	}{
		{
			name: "anv1alpha1.HTTPRoute pointer",
			obj: &anv1alpha1.HTTPRoute{
				TypeMeta: metav1.TypeMeta{
					Kind: "HTTPRoute",
				},
			},
			expected: true,
		},
		{
			name: "anv1alpha1.HTTPRoute value",
			obj: anv1alpha1.HTTPRoute{
				TypeMeta: metav1.TypeMeta{
					Kind: "HTTPRoute",
				},
			},
			expected: true,
		},
		{
			name: "core.HTTPRoute pointer",
			obj: core.NewHTTPRoute(gwv1.HTTPRoute{
				TypeMeta: metav1.TypeMeta{
					Kind: "HTTPRoute",
				},
			}),
			expected: true,
		},
		{
			name:     "nil input",
			obj:      nil,
			expected: false,
		},
		{
			name: "different type",
			obj: &gwv1.HTTPRoute{
				TypeMeta: metav1.TypeMeta{
					Kind: "HTTPRoute",
				},
			},
			expected: false,
		},
		{
			name:     "string type",
			obj:      "not a route",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCustomHTTPRoute(tt.obj)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertHTTPRoute(t *testing.T) {
	weight := int32(50)
	port := gwv1.PortNumber(8080)
	pathType := gwv1.PathMatchExact
	pathValue := "/test"
	headerType := gwv1.HeaderMatchExact
	method := gwv1.HTTPMethod("GET")
	kind := gwv1.Kind("Service")
	priority := int32(1)

	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name: "core.HTTPRoute to anv1alpha1.HTTPRoute",
			input: core.NewHTTPRoute(gwv1.HTTPRoute{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HTTPRoute",
					APIVersion: "gateway.networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-route",
					Namespace: "default",
				},
				Spec: gwv1.HTTPRouteSpec{
					CommonRouteSpec: gwv1.CommonRouteSpec{
						ParentRefs: []gwv1.ParentReference{
							{
								Name: "test-gateway",
							},
						},
					},
					Hostnames: []gwv1.Hostname{"test.example.com"},
					Rules: []gwv1.HTTPRouteRule{
						{
							BackendRefs: []gwv1.HTTPBackendRef{
								{
									BackendRef: gwv1.BackendRef{
										BackendObjectReference: gwv1.BackendObjectReference{
											Kind: &kind,
											Name: "test-service",
											Port: &port,
										},
										Weight: &weight,
									},
								},
							},
							Matches: []gwv1.HTTPRouteMatch{
								{
									Path: &gwv1.HTTPPathMatch{
										Type:  &pathType,
										Value: &pathValue,
									},
									Headers: []gwv1.HTTPHeaderMatch{
										{
											Type:  &headerType,
											Name:  "x-lattice-rule-priority",
											Value: "1",
										},
									},
									Method: &method,
								},
							},
						},
					},
				},
			}),
			expected: &anv1alpha1.HTTPRoute{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HTTPRoute",
					APIVersion: "gateway.networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-route",
					Namespace: "default",
				},
				Spec: anv1alpha1.HTTPRouteSpec{
					ParentRefs: []gwv1.ParentReference{
						{
							Name: "test-gateway",
						},
					},
					Hostnames: []gwv1.Hostname{"test.example.com"},
					Rules: []anv1alpha1.HTTPRouteRule{
						{
							HTTPRouteRule: gwv1.HTTPRouteRule{
								BackendRefs: []gwv1.HTTPBackendRef{
									{
										BackendRef: gwv1.BackendRef{
											BackendObjectReference: gwv1.BackendObjectReference{
												Kind: &kind,
												Name: "test-service",
												Port: &port,
											},
											Weight: &weight,
										},
									},
								},
								Matches: []gwv1.HTTPRouteMatch{
									{
										Path: &gwv1.HTTPPathMatch{
											Type:  &pathType,
											Value: &pathValue,
										},
										Headers: []gwv1.HTTPHeaderMatch{
											{
												Type:  &headerType,
												Name:  "x-lattice-rule-priority",
												Value: "1",
											},
										},
										Method: &method,
									},
								},
							},
							Priority: &priority,
						},
					},
				},
			},
		},
		{
			name: "anv1alpha1.HTTPRoute to core.HTTPRoute",
			input: &anv1alpha1.HTTPRoute{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HTTPRoute",
					APIVersion: "gateway.networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-route",
					Namespace: "default",
				},
				Spec: anv1alpha1.HTTPRouteSpec{
					ParentRefs: []gwv1.ParentReference{
						{
							Name: "test-gateway",
						},
					},
					Hostnames: []gwv1.Hostname{"test.example.com"},
					Rules: []anv1alpha1.HTTPRouteRule{
						{
							HTTPRouteRule: gwv1.HTTPRouteRule{
								BackendRefs: []gwv1.HTTPBackendRef{
									{
										BackendRef: gwv1.BackendRef{
											BackendObjectReference: gwv1.BackendObjectReference{
												Kind: &kind,
												Name: "test-service",
												Port: &port,
											},
											Weight: &weight,
										},
									},
								},
								Matches: []gwv1.HTTPRouteMatch{
									{
										Path: &gwv1.HTTPPathMatch{
											Type:  &pathType,
											Value: &pathValue,
										},
										Method: &method,
									},
								},
							},
							Priority: &priority,
						},
					},
				},
			},
			expected: core.NewHTTPRoute(gwv1.HTTPRoute{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HTTPRoute",
					APIVersion: "gateway.networking.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-route",
					Namespace: "default",
				},
				Spec: gwv1.HTTPRouteSpec{
					CommonRouteSpec: gwv1.CommonRouteSpec{
						ParentRefs: []gwv1.ParentReference{
							{
								Name: "test-gateway",
							},
						},
					},
					Hostnames: []gwv1.Hostname{"test.example.com"},
					Rules: []gwv1.HTTPRouteRule{
						{
							BackendRefs: []gwv1.HTTPBackendRef{
								{
									BackendRef: gwv1.BackendRef{
										BackendObjectReference: gwv1.BackendObjectReference{
											Kind: &kind,
											Name: "test-service",
											Port: &port,
										},
										Weight: &weight,
									},
								},
							},
							Matches: []gwv1.HTTPRouteMatch{
								{
									Path: &gwv1.HTTPPathMatch{
										Type:  &pathType,
										Value: &pathValue,
									},
									Headers: []gwv1.HTTPHeaderMatch{
										{
											Type:  &headerType,
											Name:  "x-lattice-rule-priority",
											Value: "1",
										},
									},
									Method: &method,
								},
							},
						},
					},
				},
			}),
		},
		{
			name: "invalid input type",
			input: &gwv1.HTTPRoute{
				TypeMeta: metav1.TypeMeta{
					Kind: "HTTPRoute",
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertHTTPRoute(tt.input)
			if tt.expected == nil {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			switch expected := tt.expected.(type) {
			case *anv1alpha1.HTTPRoute:
				actual, ok := result.(*anv1alpha1.HTTPRoute)
				require.True(t, ok)
				assert.Equal(t, expected.TypeMeta, actual.TypeMeta)
				assert.Equal(t, expected.ObjectMeta, actual.ObjectMeta)
				assert.Equal(t, expected.Spec.ParentRefs, actual.Spec.ParentRefs)
				assert.Equal(t, expected.Spec.Hostnames, actual.Spec.Hostnames)
				assert.Equal(t, len(expected.Spec.Rules), len(actual.Spec.Rules))
				for i := range expected.Spec.Rules {
					assert.Equal(t, expected.Spec.Rules[i].Priority, actual.Spec.Rules[i].Priority)
					assert.Equal(t, expected.Spec.Rules[i].BackendRefs, actual.Spec.Rules[i].BackendRefs)
					assert.Equal(t, expected.Spec.Rules[i].Matches, actual.Spec.Rules[i].Matches)
				}
			case *core.HTTPRoute:
				actual, ok := result.(*core.HTTPRoute)
				require.True(t, ok)
				assert.Equal(t, expected.Inner().TypeMeta, actual.Inner().TypeMeta)
				assert.Equal(t, expected.Inner().ObjectMeta, actual.Inner().ObjectMeta)
				assert.Equal(t, expected.Spec().ParentRefs(), actual.Spec().ParentRefs())
				assert.Equal(t, expected.Spec().Hostnames(), actual.Spec().Hostnames())
				assert.Equal(t, len(expected.Spec().Rules()), len(actual.Spec().Rules()))
				for i := range expected.Inner().Spec.Rules {
					assert.Equal(t, expected.Inner().Spec.Rules[i].BackendRefs, actual.Inner().Spec.Rules[i].BackendRefs)
					assert.Equal(t, expected.Inner().Spec.Rules[i].Matches, actual.Inner().Spec.Rules[i].Matches)
				}
			default:
				t.Fatalf("unexpected type %T", tt.expected)
			}
		})
	}
}
