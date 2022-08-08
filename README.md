<img src="https://avatars.githubusercontent.com/u/64096231" align="right" />

# Service Binding Runtime <!-- omit in toc -->

[![CI](https://github.com/servicebinding/runtime/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/servicebinding/runtime/actions/workflows/ci.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/servicebinding/runtime)](https://goreportcard.com/report/github.com/servicebinding/runtime)
[![Go Reference](https://pkg.go.dev/badge/github.com/servicebinding/runtime.svg)](https://pkg.go.dev/github.com/servicebinding/runtime)
[![codecov](https://codecov.io/gh/servicebinding/runtime/branch/main/graph/badge.svg?token=D2Hs4MIXBZ)](https://codecov.io/gh/servicebinding/runtime)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

Reference implementation of the [ServiceBinding.io](https://servicebinding.io) [1.0 spec](https://servicebinding.io/spec/core/1.0.0/). The full specification is implemented, please open an issue for any discrepancies.

- [Getting Started](#getting-started)
  - [Deploy a released build](#deploy-a-released-build)
  - [Build from source](#build-from-source)
    - [Undeploy controller](#undeploy-controller)
- [Samples](#samples)
- [Supported Services](#supported-services)
- [Supported Workloads](#supported-workloads)
- [Architecture](#architecture)
  - [Controller](#controller)
  - [Webhooks](#webhooks)
- [Contributing](#contributing)
  - [Test It Out](#test-it-out)
  - [Modifying the API definitions](#modifying-the-api-definitions)
- [Community, discussion, contribution, and support](#community-discussion-contribution-and-support)
  - [Code of conduct](#code-of-conduct)

## Getting Started

You’ll need a Kubernetes cluster to run against. You can use [kind](https://kind.sigs.k8s.io) to get a local cluster for testing, or run against a remote cluster.

After the controller is deployed, try out the [samples](#samples).

### Deploy a released build

The easiest way to get started is by deploying the [latest release](https://github.com/servicebinding/runtime/releases). Alternatively, you can [build the runtime from source](#build-from-source).

### Build from source

1. Define where to publish images:

   ```sh
   export KO_DOCKER_REPO=<a-repository-you-can-write-to>
   ```

   For kind, a registry is not required:

   ```sh
   export KO_DOCKER_REPO=kind.local
   ```
	
1. Build and deploy the controller to the cluster:

   Note: The cluster must have the [cert-manager](https://cert-manager.io) deployed.  There is a `make deploy-cert-manager` target to deploy the cert-manager.

   ```sh
   make deploy
   ```

#### Undeploy controller
Undeploy the controller to the cluster:

```sh
make undeploy
```

## Samples

Samples are located in the [samples directory](./samples), including:

- [Spring PetClinic with MySQL](./samples/spring-petclinic)
- [Controlled Resource](./samples/controlled-resource)
- [Overridden Type and Provider](./samples/overridden-type-provider)
- [Multiple Bindings](./samples/multi-binding)

## Supported Services

Kubernetes defines no provisioned services by default, however, `Secret`s may be [directly referenced](https://servicebinding.io/spec/core/1.0.0/#direct-secret-reference).

Additional services can be supported dynamically by [defining a `ClusterRole`](https://servicebinding.io/spec/core/1.0.0/#considerations-for-role-based-access-control-rbac).

## Supported Workloads

Support for the built-in k8s workload resource is pre-configured including:
- apps `DaemonSet`
- apps `Deployment`
- apps `ReplicaSet`
- apps `StatefulSet`
- batch `CronJob` (also includes a `ClusterResourceMapping`)
- batch `Job` (since Jobs are immutable, the ServiceBinding must be defined and service resolved before the job is created)
- core `ReplicationController`

Additional workloads can be supported dynamically by [defining a `ClusterRole`](https://servicebinding.io/spec/core/1.0.0/#considerations-for-role-based-access-control-rbac-1) and if not PodSpecable, a [`ClusterWorkloadResourceMapping`](https://servicebinding.io/spec/core/1.0.0/#workload-resource-mapping).

## Architecture

The [Service Binding for Kubernetes Specification](https://servicebinding.io/spec/core/1.0.0/) defines the shape of [Provisioned Services](https://servicebinding.io/spec/core/1.0.0/#provisioned-service), and how the `Secret` is [projected into a workload](https://servicebinding.io/spec/core/1.0.0/#workload-projection). The spec says less (intentionally) about how this happens.

Both a controller and mutating admission webhook are used to project a `Secret` defined by the service referenced by the `ServiceBinding` resource into the workloads referenced. The controller is used to process `ServiceBinding`s by resolving services, projecting workloads and updating the status. The webhook is used to prevent removal of the workload projection, projecting workload on create, and a notification trigger for `ServiceBinding`s the controller should process.

The apis, resolver and projector packages are defined by the reference implementation and reused here with slight modifications. The bulk of the work to bind a service to a workload is encapsulated with these packages. The output from the projector is deterministic and idempotent. The order that service bindings are applied to, or removed from, a workload does not matter. If a workload is bound and then unbound, the only trace will be the `SERVICE_BINDING_ROOT` environment variable.

There are a limited number of resources that maintain an informer cache within the manager:
- `ServiceBinding`
- `ClusterWorkloadResourceMapping`
- `MutatingWebhookConfiguration`
- `ValidatingWebhookConfiguration`

### Controller

When a `ServiceBinding` is created, updated or deleted the controller processes the resource. It will:
- resolve the referenced service resource, looking at it's `.spec.binding.name` for the name of the Secret to bind
- reflect the discovered `Secret` name onto the `ServiceBinding`'s `.status.binding.name`
- the `ServiceAvailable` condition is updated on the `ServiceBinding`
- the referenced workloads are resolved (either by name or selector)
- a `ClusterWorkloadResourceMapping` is resolved for the apiVersion/kind of the workload (or a default value for a PodSpecable workload is used)
- the resolved `Secret` name is projected into the workload
- the `Ready` condition is updated on the `ServiceBinding`

### Webhooks

In addition to that main flow, a `MutatingWebhookConfiguration` and `ValidationWebhookConfiguration` are updated:
- all `ServiceBinding`s in the cluster are resolved
- the rules for a `MutatingWebhookConfiguration` are updated based on the set of all workload group-kinds referenced
- the rules for a `ValidatingWebhookConfiguration` are updated based on the set of all workload and service group-kinds referenced

The `MutatingWebhookConfiguration` is used to intercept create and update requests for workloads:
- all `ServiceBinding`s targeting the workload are resolved
- a `ClusterWorkloadResourceMapping` is resolved for the apiVersion/kind of the workload (or a default value for a PodSpecable workload is used)
- for each `ServiceBinding` the resolved `Secret` name is projected into the workload
- the delta between the original resource and the projected resource is returned with the webhook response as a patch

The `ValidationWebhookConfiguration` is used as an alternative to watching the API Server directly for these types and keeping an informer cache. When a webhook request is received, the `ServiceBinding`s that reference that resource as a workload or service are resolved and enqueued for the controller to process.

No blocking work is performed within the webhooks.

## Contributing

### Test It Out

Run the unit tests:

```sh
make test
```

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## Community, discussion, contribution, and support

The Service Binding Controller project is a community lead effort.
A bi-weekly [working group call][working-group] is open to the public.
Discussions occur here on GitHub and on the [#bindings-discuss channel in the Kubernetes Slack][slack].

If you catch an error in the implementation, please let us know by opening an issue at our
[GitHub repository][repo].

### Code of conduct

Participation in the Service Binding community is governed by the [Contributor Covenant][code-of-conduct].

[working-group]: https://docs.google.com/document/d/1rR0qLpsjU38nRXxeich7F5QUy73RHJ90hnZiFIQ-JJ8/edit#heading=h.ar8ibc31ux6f
[slack]: https://kubernetes.slack.com/archives/C012F2GPMTQ
[repo]: https://github.com/servicebinding/runtime
[code-of-conduct]: ./CODE_OF_CONDUCT.md
