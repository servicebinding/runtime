# External Secrets

[External Secrets][eso] is an operator to fetch content from secure stores and synthesize that state into a Kubernetes `Secret`.

This sample uses a fake `SecretStore` (to avoid external infrastructure) and `ExternalSecret` to create a binding Secret. The [Spring PetClinic][petclinic] workload and MySQL database are independently bound to the `ExternalSecret`.

## Setup

If not already installed, [install the ServiceBinding CRD and controller][install], and the [External Secrets Operator][eso-install].

> NOTE: Provisioned Service support was added to External Secrets in the v0.8.2 release, prior versions are not compatible.

This sample uses [Carvel Kapp][kapp-install], rather than `kubectl` to install and watch the sample become ready.

## Deploy

Apply the `ExternalSecret`, `SecretStore`, PetClinic workload, MySQL service and connect them with `ServiceBinding`s:

```sh
kapp deploy --app servicebinding-sample-external-secrets -f samples/external-secrets
```

When you are done with this sample, all resources in the deploy can be removed by running:

```sh
kapp delete --app servicebinding-sample-external-secrets
```

When prompted, you can review the resource about to be created (updated or deleted) and approve them, or add `--yes` to the above commands. Resources are [tracked between deploys](https://carvel.dev/kapp/docs/latest/diff/), if a resource is removed from the file it will be removed from the cluster on the next deploy.

Kapp will monitor the [health of the resources](https://carvel.dev/kapp/docs/latest/apply-waiting/) it creates and exit when they become ready, or fail to become ready. The startup [logs from our workload](https://carvel.dev/kapp/docs/latest/apply/#kappk14siodeploy-logs) will also be displayed.

## Understand

Inspect the `ExternalSecret`:

```sh
kubectl describe externalsecrets.external-secrets.io eso-example
```

If the `ExternalSecret` is working, a new `Secret` is created and the name of that `Secret` is reflected on the status.
The describe output will contain:

```txt
...
Status:
  Binding:
    Name:  eso-example-db
...
```

The `ServiceBinding` for the PetClinic workload references the `ExternalSecret` resource to discover the binding `Secret` exposed. The [Spring PetClinic sample](../spring-petclinic/) goes into deeper detail for how a Spring Boot workload can consume service bindings at runtime.

```sh
kubectl describe servicebindings.servicebinding.io eso-example
```

```txt
Spec:
  Service:
    API Version:  external-secrets.io/v1beta1
    Kind:         ExternalSecret
    Name:         eso-example-db
```

Additional, the `ServiceBinding` for the MySQL database projects environment variables since the MySQL image is not natively aware of service bindings.

```sh
kubectl describe servicebindings.servicebinding.io eso-example-db
```

```txt
...
Spec:
  Env:
  - Key:   username
    Name:  MYSQL_USER
  - Key:   password
    Name:  MYSQL_PASSWORD
  - Key:   database
    Name:  MYSQL_DATABASE
...
```

## Play

A key advantage of referencing a Provisioned Service resources over directly referencing the binding `Secret`, is that the name of the `Secret` referenced can be updated at any time, bound workloads will automatically receive the updated `Secret`.

Try updating the `ExternalSecret` to manage the `Secret` under a different name.

```sh
kubectl patch externalsecrets.external-secrets.io eso-example-db --type json --patch '[{"op": "replace", "path": "/spec/target/name", "value":"my-new-super-duper-secret"}]'
```

Look at the `ExternalSecret` status and the `Deployment`s to see that they have been updated to use the `my-new-super-duper-secret` secret.

[eso]: https://external-secrets.io/
[eso-install]: https://external-secrets.io/v0.8.2/introduction/getting-started/
[install]: ../../README.md#getting-started
[kapp-install]: https://carvel.dev/kapp/
[petclinic]: https://github.com/spring-projects/spring-petclinic
