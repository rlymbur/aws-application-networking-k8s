package utils

import (
	"fmt"

	anv1alpha1 "github.com/aws/aws-application-networking-k8s/pkg/apis/applicationnetworking/v1alpha1"
	"github.com/aws/aws-application-networking-k8s/pkg/model/core"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// HasPriorityHeader checks if any rule in the given core.Route has a priority header
func HasPriorityHeader(route core.Route) bool {
	if route == nil {
		return false
	}

	for _, rule := range route.Spec().Rules() {
		if httpRule, ok := rule.(*core.HTTPRouteRule); ok {
			for _, match := range httpRule.Matches() {
				for _, header := range match.Headers() {
					if header.Name() == "x-lattice-rule-priority" {
						return true
					}
				}
			}
		}
	}
	return false
}

// IsCustomHTTPRoute checks if the given object is either an anv1alpha1.HTTPRoute or core.HTTPRoute
func IsCustomHTTPRoute(obj interface{}) bool {
	switch obj.(type) {
	case *anv1alpha1.HTTPRoute, anv1alpha1.HTTPRoute:
		return true
	case *core.HTTPRoute:
		return true
	default:
		return false
	}
}

// ConvertHTTPRoute converts between core.HTTPRoute and anv1alpha1.HTTPRoute types.
// If the input is a core.HTTPRoute, it converts to anv1alpha1.HTTPRoute.
// If the input is an anv1alpha1.HTTPRoute, it converts to core.HTTPRoute.
func ConvertHTTPRoute(route interface{}) (interface{}, error) {
	if !IsCustomHTTPRoute(route) {
		return nil, fmt.Errorf("input is not a valid HTTPRoute type")
	}

	switch r := route.(type) {
	case *core.HTTPRoute:
		return convertCoreToV1Alpha1(r), nil
	case *anv1alpha1.HTTPRoute:
		return convertV1Alpha1ToCore(r), nil
	case anv1alpha1.HTTPRoute:
		return convertV1Alpha1ToCore(&r), nil
	default:
		return nil, fmt.Errorf("unsupported HTTPRoute type")
	}
}

// convertCoreToV1Alpha1 converts a core.HTTPRoute to anv1alpha1.HTTPRoute
func convertCoreToV1Alpha1(coreRoute *core.HTTPRoute) *anv1alpha1.HTTPRoute {
	v1alpha1Route := &anv1alpha1.HTTPRoute{
		TypeMeta:   coreRoute.Inner().TypeMeta,
		ObjectMeta: coreRoute.Inner().ObjectMeta,
		Status:     coreRoute.Inner().Status,
	}

	// Convert spec fields
	v1alpha1Route.Spec.ParentRefs = coreRoute.Spec().ParentRefs()
	v1alpha1Route.Spec.Hostnames = coreRoute.Spec().Hostnames()

	// Convert rules, preserving priority
	coreRules := coreRoute.Spec().Rules()
	v1alpha1Route.Spec.Rules = make([]anv1alpha1.HTTPRouteRule, len(coreRules))

	for i, coreRule := range coreRules {
		httpRule, ok := coreRule.(*core.HTTPRouteRule)
		if !ok {
			continue
		}

		v1alpha1Route.Spec.Rules[i] = anv1alpha1.HTTPRouteRule{
			HTTPRouteRule: gwv1.HTTPRouteRule{
				BackendRefs: func() []gwv1.HTTPBackendRef {
					var refs []gwv1.HTTPBackendRef
					for _, ref := range httpRule.BackendRefs() {
						if httpRef, ok := ref.(*core.HTTPBackendRef); ok {
							refs = append(refs, gwv1.HTTPBackendRef{
								BackendRef: gwv1.BackendRef{
									BackendObjectReference: gwv1.BackendObjectReference{
										Group:     httpRef.Group(),
										Kind:      httpRef.Kind(),
										Name:      httpRef.Name(),
										Namespace: httpRef.Namespace(),
										Port:      httpRef.Port(),
									},
									Weight: httpRef.Weight(),
								},
							})
						}
					}
					return refs
				}(),
				Matches: func() []gwv1.HTTPRouteMatch {
					var matches []gwv1.HTTPRouteMatch
					for _, match := range httpRule.Matches() {
						if httpMatch, ok := match.(*core.HTTPRouteMatch); ok {
							matches = append(matches, gwv1.HTTPRouteMatch{
								Path: httpMatch.Path(),
								Headers: func() []gwv1.HTTPHeaderMatch {
									var headers []gwv1.HTTPHeaderMatch
									for _, h := range httpMatch.Headers() {
										headers = append(headers, gwv1.HTTPHeaderMatch{
											Type:  h.Type(),
											Name:  gwv1.HTTPHeaderName(h.Name()),
											Value: h.Value(),
										})
									}
									return headers
								}(),
								QueryParams: httpMatch.QueryParams(),
								Method:      httpMatch.Method(),
							})
						}
					}
					return matches
				}(),
			},
			Priority: httpRule.Priority(),
		}
	}

	return v1alpha1Route
}

// convertV1Alpha1ToCore converts an anv1alpha1.HTTPRoute to core.HTTPRoute
func convertV1Alpha1ToCore(v1alpha1Route *anv1alpha1.HTTPRoute) *core.HTTPRoute {
	route := gwv1.HTTPRoute{
		TypeMeta:   v1alpha1Route.TypeMeta,
		ObjectMeta: v1alpha1Route.ObjectMeta,
		Status:     v1alpha1Route.Status,
		Spec: gwv1.HTTPRouteSpec{
			CommonRouteSpec: gwv1.CommonRouteSpec{
				ParentRefs: v1alpha1Route.Spec.ParentRefs,
			},
			Hostnames: v1alpha1Route.Spec.Hostnames,
			Rules:     make([]gwv1.HTTPRouteRule, len(v1alpha1Route.Spec.Rules)),
		},
	}

	for i, rule := range v1alpha1Route.Spec.Rules {
		route.Spec.Rules[i] = rule.HTTPRouteRule

		// If priority is set, add it as a header
		if rule.Priority != nil {
			headerType := gwv1.HeaderMatchExact
			priorityHeader := gwv1.HTTPHeaderMatch{
				Type:  &headerType,
				Name:  "x-lattice-rule-priority",
				Value: fmt.Sprintf("%d", *rule.Priority),
			}

			// If there are no matches, create one with just the priority header
			if len(route.Spec.Rules[i].Matches) == 0 {
				route.Spec.Rules[i].Matches = []gwv1.HTTPRouteMatch{
					{
						Headers: []gwv1.HTTPHeaderMatch{priorityHeader},
					},
				}
			} else {
				// Add priority header to each match
				for j := range route.Spec.Rules[i].Matches {
					route.Spec.Rules[i].Matches[j].Headers = append(
						route.Spec.Rules[i].Matches[j].Headers,
						priorityHeader,
					)
				}
			}
		}
	}

	return core.NewHTTPRoute(route)
}
