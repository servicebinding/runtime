# Spring PetClinic with MySQL

[Spring PetClinic][petclinic] is a sample [Spring Boot][boot] web application that can be used with MySQL.

## Setup

If not already installed, [install the ServiceBinding CRD and controller][install].

This sample uses [Carvel Kapp][kapp-install], rather than `kubectl` to install and watch the sample become ready.

## Deploy

Apply the PetClinic workload, MySQL service and connect them with a ServiceBinding:

```sh
kapp deploy --app servicebinding-sample-spring-petclinic -f samples/spring-petclinic
```

When you are done with this sample, all resources in the deploy can be removed by running:

```sh
kapp delete --app servicebinding-sample-spring-petclinic
```

When prompted, you can review the resource about to be created (updated or deleted) and approve them, or add `--yes` to the above commands. Resources are [tracked between deploys](https://carvel.dev/kapp/docs/latest/diff/), if a resource is removed from the file it will be removed from the cluster on the next deploy.

Kapp will monitor the [health of the resources](https://carvel.dev/kapp/docs/latest/apply-waiting/) it creates and exit when they become ready, or fail to become ready. The startup [logs from our workload](https://carvel.dev/kapp/docs/latest/apply/#kappk14siodeploy-logs) will also be displayed.

## Understand

Inspect the PetClinic workload as bound:

```sh
kubectl describe deployment spring-petclinic
```

If the ServiceBinding is working, a new environment variable (SERVICE_BINDING_ROOT), volume and volume mount (e.g. servicebinding-54dec81e-49f6-467d-934f-36029f2dfd26) is added to the deployment.
The describe output will contain:

```txt
...
  Containers:
   workload:
    ...
    Environment:
      SPRING_PROFILES_ACTIVE:  mysql
      SERVICE_BINDING_ROOT:    /bindings
    Mounts:
      /bindings/spring-petclinic-db from servicebinding-54dec81e-49f6-467d-934f-36029f2dfd26 (ro)
  Volumes:
   servicebinding-54dec81e-49f6-467d-934f-36029f2dfd26:
    Type:                Projected (a volume that contains injected data from multiple sources)
    SecretName:          spring-petclinic-db
    SecretOptionalName:  <nil>
...
```

The workload uses [Spring Cloud Bindings][scb], which discovers the bound MySQL service by type and reconfigures Spring Boot to consume the service.
Spring Cloud Bindings is automatically added to Spring applications built by Paketo buildpacks.

We can see the effect of Spring Cloud Bindings by view the workload logs. While the startup logs are shown as part of the deploy, we can fetch them directly as well:

```sh
kubectl logs -l app=spring-petclinic -c workload --tail 1000
```

The logs should contain:

```txt
...
Spring Cloud Bindings Enabled
...


              |\      _,,,--,,_
             /,`.-'`'   ._  \-;;,_
  _______ __|,4-  ) )_   .;.(__`'-'__     ___ __    _ ___ _______
 |       | '---''(_/._)-'(_\_)   |   |   |   |  |  | |   |       |
 |    _  |    ___|_     _|       |   |   |   |   |_| |   |       | __ _ _
 |   |_| |   |___  |   | |       |   |   |   |       |   |       | \ \ \ \
 |    ___|    ___| |   | |      _|   |___|   |  _    |   |      _|  \ \ \ \
 |   |   |   |___  |   | |     |_|       |   | | |   |   |     |_    ) ) ) )
 |___|   |_______| |___| |_______|_______|___|_|  |__|___|_______|  / / / /
 ==================================================================/_/_/_/

:: Built with Spring Boot :: 2.3.3.RELEASE


2022-08-02 21:35:27.236  INFO 1 --- [           main] o.s.s.petclinic.PetClinicApplication     : Starting PetClinicApplication v2.3.0.BUILD-SNAPSHOT on spring-petclinic-5f868997f6-9q8jm with PID 1 (/workspace/BOOT-INF/classes started by cnb in /workspace)
2022-08-02 21:35:27.243  INFO 1 --- [           main] o.s.s.petclinic.PetClinicApplication     : The following profiles are active: mysql
2022-08-02 21:35:27.350  INFO 1 --- [           main] .BindingSpecificEnvironmentPostProcessor : Creating binding-specific PropertySource from Kubernetes Service Bindings
...
```

`kapp` uses config file defined in [kapp-config.yaml](./kapp-config.yaml) to teach it understand the specifics for the `ServiceBinding` resource. We instruct kapp to wait for the `Ready` condition to be `True` to indicate the binding is successful, or `False` to indicate an error the requires attention. We also tell it to wait for the `ServiceAvailable` condition to be `True` before creating any resources that depend on the `ServiceBinding`.

Annotations (`kapp.k14s.io/...`) on individual resources are used to further configure kapp. In this case we define the [order resources are applied](https://carvel.dev/kapp/docs/latest/apply-ordering/) with change groups for sets of resources (`service`, `workload` and `binding`), and change rules to define the order that resources are created or deleted. On the `ServiceBinding` we define a rule `upsert before upserting workload`, this tells kapp to create/update the ServiceBinding resource before create/update the resource in the workload change group. This way, we can intercept the create request for the workload Deployment, and project the binding in at creation time, rather than either needing to manually order the creation of resources, or initially rolling out a ReplicaSet without the binding that we know will fail only to shortly have the correct config with the projection applied.

## Play

To connect to the workload, forward a local port into the cluster:

```sh
kubectl port-forward service/spring-petclinic 8080:80
```

Then open `http://localhost:8080` in a browser.


[petclinic]: https://github.com/spring-projects/spring-petclinic
[boot]: https://spring.io/projects/spring-boot
[paketo]: https://paketo.io
[install]: ../../README.md#getting-started
[scb]: https://github.com/spring-cloud/spring-cloud-bindings
