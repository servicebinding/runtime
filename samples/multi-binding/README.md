# Multi-bindings

Often an workload needs to consume more than one service.
In that case, multiple service binding resources can each bind a distinct service to the same workload.

In this sample, we'll use a [Kubernetes Job][kubernetes-jobs] to dump the environment to the logs and exit.

## Setup

If not already installed, [install the ServiceBinding CRD and controller][install].

## Deploy

Like Pods, Kubernetes Jobs are immutable after they are created.
We need to make sure the `ServiceBinding`s are fully configured before the workload is created.

Apply the services and `ServiceBinding`s:

```sh
kubectl apply -f ./samples/multi-binding/service.yaml -f ./samples/multi-binding/service-binding.yaml
```

Check on the status of the `ServiceBinding`:

```sh
kubectl get servicebinding -l multi-binding=true -oyaml
```

For each service binding, the `ServiceAvailable` condition should be `True` and the `Ready` condition `False`.

```
...
    conditions:
    - lastTransitionTime: "2022-08-02T21:28:45Z"
      message: the workload was not found
      reason: WorkloadNotFound
      status: Unknown
      type: Ready
    - lastTransitionTime: "2022-08-02T21:28:45Z"
      message: ""
      reason: ResolvedBindingSecret
      status: "True"
      type: ServiceAvailable
```

Create the workload `Job`:

```sh
kubectl apply -f ./samples/multi-binding/workload.yaml
```

## Understand

Each `ServiceBinding` resource defines an environment variable that is projected into the workload in addition to the binding volume mount.

```sh
kubectl describe job multi-binding
```

```
...
Environment:
  SERVICE_BINDING_ROOT:  /bindings
  MULTI_BINDING_1:       <set to the key 'number' in secret 'multi-binding-1'>  Optional: false
  MULTI_BINDING_2:       <set to the key 'number' in secret 'multi-binding-2'>  Optional: false
...
```

The job dumps the environment to the log and then exits.
We should see our injected environment variable as well as other variable commonly found in Kubernetes containers.

Inspect the logs from the job:

```sh
kubectl logs -l job-name=multi-binding --tail 100
```

```
...
SERVICE_BINDING_ROOT=/bindings
MULTI_BINDING_1=1
MULTI_BINDING_2=2
...
```

## Play

Try adding yet another binding targeting the same Job.
Remember that Jobs are immutable after they are created, so you'll need to delete and recreate the Job to see the additional binding.

Alternatively, define a `Deployment` and update each binding to target the new Deployment.
Since Deployments are mutable, each service binding that is added or removed will be reflected on the Deployment and trigger the rollout of a new `ReplicaSet`.

[install]: ../../README.md#getting-started
[kubernetes-jobs]: https://kubernetes.io/docs/concepts/workloads/controllers/job/
