package core

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type RouteType string

type Route interface {
	Spec() RouteSpec
	Status() RouteStatus
	Name() string
	Namespace() string
	DeletionTimestamp() *metav1.Time
	DeepCopy() Route
	K8sObject() client.Object
	GroupKind() metav1.GroupKind
}

func NewRoute(object client.Object) (Route, error) {
	switch obj := object.(type) {
	case *gwv1.HTTPRoute:
		return NewHTTPRoute(*obj), nil
	case *gwv1.GRPCRoute:
		return NewGRPCRoute(*obj), nil
	case *gwv1alpha2.TLSRoute:
		return NewTLSRoute((*obj)), nil
	default:
		return nil, fmt.Errorf("unexpected route type for object %+v", object)
	}
}

func ListAllRoutes(context context.Context, client client.Client) ([]Route, error) {
	httpRoutes, err := ListHTTPRoutes(context, client)
	if err != nil {
		return nil, err
	}

	grpcRoutes, err := ListGRPCRoutes(context, client)
	if err != nil {
		return nil, err
	}
	tlsRoutes, err := ListTLSRoutes(context, client)
	if err != nil {
		return nil, err
	}
	var routes []Route
	routes = append(routes, httpRoutes...)
	routes = append(routes, grpcRoutes...)
	routes = append(routes, tlsRoutes...)
	return routes, nil
}

type RouteSpec interface {
	ParentRefs() []gwv1.ParentReference
	Hostnames() []gwv1.Hostname
	Rules() []RouteRule
	Equals(routeSpec RouteSpec) bool
}

type RouteStatus interface {
	Parents() []gwv1.RouteParentStatus
	SetParents(parents []gwv1.RouteParentStatus)
	UpdateParentRefs(parent gwv1.ParentReference, controllerName gwv1.GatewayController)
	UpdateRouteCondition(parent gwv1.ParentReference, condition metav1.Condition)
}

type RouteRule interface {
	BackendRefs() []BackendRef
	Matches() []RouteMatch
	Equals(routeRule RouteRule) bool
}

type BackendRef interface {
	Weight() *int32
	Group() *gwv1.Group
	Kind() *gwv1.Kind
	Name() gwv1.ObjectName
	Namespace() *gwv1.Namespace
	Port() *gwv1.PortNumber
	Equals(backendRef BackendRef) bool
}

type RouteMatch interface {
	Headers() []HeaderMatch
	Equals(routeMatch RouteMatch) bool
}

type HeaderMatch interface {
	Type() *gwv1.HeaderMatchType
	Name() string
	Value() string
	Equals(headerMatch HeaderMatch) bool
}
