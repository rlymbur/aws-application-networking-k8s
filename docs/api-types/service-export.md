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
* Limited to one ServiceExport per Service. If you need multiple exports representing each port,
  you should create multiple Service-ServiceExport pairs.

### Protocol Support
* **HTTP Traffic**: Creates an HTTP target group with HTTP1 protocol version
* **GRPC Traffic**: Creates a GRPC target group with GRPC protocol version
* The exported service can be used in both HTTPRoutes and GRPCRoutes

### Annotations

* `application-networking.k8s.aws/port`  
  Represents which port of the exported Service will be used.
  When a comma-separated list of ports is provided, the traffic will be distributed to all ports in the list.

## Example Configurations

### HTTP Service Export
The following yaml will create a ServiceExport for an HTTP Service named `service-1`:
```yaml
apiVersion: application-networking.k8s.aws/v1alpha1
kind: ServiceExport
metadata:
  name: service-1
  annotations:
    application-networking.k8s.aws/port: "9200"
spec: {}
```

### GRPC Service Export
For GRPC services, you'll typically want to configure a TargetGroupPolicy along with the ServiceExport:

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
---
apiVersion: application-networking.k8s.aws/v1alpha1
kind: ServiceExport
metadata:
  name: grpc-service
  annotations:
    application-networking.k8s.aws/port: "50051"
spec: {}
```

For more detailed examples of GRPC service exports, see the [GRPC guide](../guides/grpc.md#exporting-grpc-services-with-serviceexport).
