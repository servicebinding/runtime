# Controlled Resource

Sometimes a the workload resource you create is not PodSpec-able, but it creates a child resource that is. In cases like this, we can inject into the child resource. Normally, it is not possible to mutate a controlled resource, as the controller should see the mutation and undo the change, however, `ServiceBinding`s are able to inject into controlled resources and keep the injected values in sync.

This behavior may not be portable across other service binding implementations.

## Setup

If not already installed, [install the ServiceBinding CRD and controller][install].

For this sample, we'll also need [Knative Serving][knative-install].

## Deploy

Apply the custom workload, service and connect them with a `ServiceBinding`:

```sh
kubectl apply -f ./samples/controlled-resource
```

## Understand

The workload.yaml defines a Knative `Service`, which in turn controls a Knative `Configuration`.
The `ServiceBinding`'s workload reference targets the `Configuration`, instead of the `Service`.

> Note: the Knative `Service` and `Configuration` resources are both PodSpec-able, and can both be the target of a service binding. Typically, a binding would target the service instead of the configuration resource. We're targeting the configuration in this sample to demonstrate targeting a controlled resource.

The workload reference is using a label selector to match the target configuration. Label selectors are useful when a binding needs to target multiple resources, or the name of the target resource is not known. Controllers may generate multiple child resources, or use a generated name which will not be known in advance.

We can see the binding applied to the Knative `Configuration`.

```sh
kubectl describe configurations.serving.knative.dev -l serving.knative.dev/service=controlled-resource
```

It will contain an environment variable `TARGET` provided by the binding.

```
...
        Env:
          Name:   SERVICE_BINDING_ROOT
          Value:  /bindings
          Name:   TARGET
          Value From:
            Secret Key Ref:
              Key:   target
              Name:  controlled-resource
...
```

Try manually editing the configuration to add a new environment variable.

```sh
kubectl edit configurations.serving.knative.dev controlled-resource
```

The new value will be removed by the Knative controller.
This is normal for controlled resources.
The service binding is able to inject into controlled resources because it hooks directly into the cluster's API server via a mutating webhook.
The webhook is able to intercept requests from the Knative controller and transparently reapplies the binding, preventing the controller from removing the injected values.

Knative resources are not exposed by default for binding, This sample includes a ClusterRole that grants the controller access to project the service into Knative Serving resources. ClusterRoles that include the `servicebinding.io/controller: "true"` label are automatically available to the Service Binding Runtime.

## Play

Try invoking the Knative Service to view the message injected by the service binding

```sh
kubectl get ksvc controlled-resource --output=custom-columns=NAME:.metadata.name,URL:.status.url
```

Assuming ingress is properly configured, an invocable URL will be returned for the controlled-resource service.

```
NAME                   URL
controlled-resource   http://controlled-resource.default.{YOUR_CLUSTERS_HOSTNAME}
```

Make a request to the service.

```sh
curl http://controlled-resource.default.{YOUR_CLUSTERS_HOSTNAME}
Hello service binding!
```

Going further, try changing the message in the controlled-resource `Secret`.
While values in the volume are updated in a running container, an environment variable will only be updated when a new Pod is created.
Because Knative will auto-scale workloads based on requests, new Pods will be created over time, but exactly when is highly dependant on the load.


[knative-install]: https://knative.dev/docs/install/yaml-install/serving/install-serving-with-yaml/
[install]: ../../README.md#getting-started
