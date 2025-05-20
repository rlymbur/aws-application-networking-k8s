# ServiceExport API Reference

## Introduction

In AWS Gateway API Controller, `ServiceExport` enables a Service for multi-cluster traffic setup.
Clusters can import the exported service with [`ServiceImport`](service-import.md) resource.

Internally, creating a ServiceExport creates a standalone VPC Lattice [target group](https://docs.aws.amazon.com/vpc-lattice/latest/ug/target-groups.html).
Even without ServiceImports, creating ServiceExports can be useful in case you only need the target groups created;
for example, using target groups in the VPC Lattice setup outside Kubernetes.

Note that ServiceExport is not the implementation of Kubernetes [Multicluster Service APIs](https://multicluster.sigs.k8s.io/concepts/multicluster-services-api/);
instead AWS Gateway API Controller uses its own version of the resource for the purpose of Gateway API integration.


### Limitations
* Limited to one ServiceExport per Service.

### Protocol Support
ServiceExport supports three route types:
* **HTTP**: Creates an HTTP target group with HTTP1 protocol version
* **GRPC**: Creates a GRPC target group with GRPC protocol version
* **TLS**: Creates a TCP target group for TLS passthrough

### Configuration

You can configure which ports to export and their route types in two ways:

1. Using the `exportedPorts` field in the spec (recommended):
   ```yaml
   spec:
     exportedPorts:
     - port: 80
       routeType: HTTP
     - port: 8081
       routeType: GRPC
     - port: 443
       routeType: TLS
   ```

2. Using the legacy port annotation (deprecated):
   ```yaml
   metadata:
     annotations:
       application-networking.k8s.aws/port: "80"
   ```
   When using the annotation, all ports will be exported as HTTP target groups for backward compatibility.

## Example Configurations

### Multi-Protocol Service Export
The following example exports a service with multiple protocols:
```yaml
apiVersion: application-networking.k8s.aws/v1alpha1
kind: ServiceExport
metadata:
  name: multi-protocol-service
spec:
  exportedPorts:
  - port: 80
    routeType: HTTP
  - port: 8081
    routeType: GRPC
  - port: 443
    routeType: TLS
```

### HTTP Service Export
For HTTP-only services:
```yaml
apiVersion: application-networking.k8s.aws/v1alpha1
kind: ServiceExport
metadata:
  name: http-service
spec:
  exportedPorts:
  - port: 80
    routeType: HTTP
```

### GRPC Service Export
For GRPC services:
```yaml
apiVersion: application-networking.k8s.aws/v1alpha1
kind: ServiceExport
metadata:
  name: grpc-service
spec:
  exportedPorts:
  - port: 50051
    routeType: GRPC
```

You can also configure health checks using a TargetGroupPolicy:
```yaml
apiVersion: application-networking.k8s.aws/v1alpha1
kind: TargetGroupPolicy
metadata:
  name: grpc-policy
spec:
  targetRef:
    group: application-networking.k8s.aws
    kind: ServiceExport
    name: grpc-service
  protocol: HTTP
  protocolVersion: GRPC
  healthCheck:
    enabled: true
    protocol: HTTP
    protocolVersion: GRPC
    port: 50051
```

For more detailed examples of GRPC service exports, see the [GRPC guide](../guides/grpc.md#exporting-grpc-services-with-serviceexport).
