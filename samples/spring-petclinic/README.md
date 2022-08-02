# Spring PetClinic with MySQL

[Spring PetClinic][petclinic] is a sample [Spring Boot][boot] web application that can be used with MySQL.

## Setup

If not already installed, [install the ServiceBinding CRD and controller][install].

## Deploy

Apply the PetClinic workload, MySQL service and connect them with a ServiceBinding:

```sh
kubectl apply -f ./samples/spring-petclinic
```

Wait for the workload (and database) to start and become healthy:

```sh
kubectl wait deployment spring-petclinic --for condition=available --timeout=2m
```

## Understand

Inspect the PetClinic workload as bound:

```sh
kubectl describe deployment spring-petclinic
```

If the ServiceBinding is working, a new environment variable (SERVICE_BINDING_ROOT), volume and volume mount (binding-49a23274b0590d5057aae1ae621be723716c4dd5) is added to the deployment.
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

We can see the effect of Spring Cloud Bindings by view the workload logs:

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
