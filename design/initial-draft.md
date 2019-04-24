# GitLab-Controller

## Objective

Provide functionality to support GitLab provisioning in hosted Crossplane environment by means of _GitLabController_ 
and _GitLabCRD_

## Overview


### Workloads


Crossplane provides concept, definition and initial support for Portable Workloads. 

Crossplane workload definition enables the crossplane user to:

*   Define the payload definition. The initial version supports only:
    *   targetDeployment
    *   targetService
    *   targetNamespace
*   Define the target environment (in terms of cluster selector)

Crossplane workload functionality is responsible for:

*   Scheduling payload propagation to the target cluster
*   Propagation the payload
*   Propagated payload status report

The initial workload implementation is at the “Proof of Concept” level. 

Currently, there is an active effort to redesign Workload to enable payload support for a much broader spectrum of 
Kubernetes artifacts. At this time the working concepts replacing Workload are:

*   _KubernetesApplication_
*   _KubernetesApplicationResource_

### Real-world Application

The big step towards Crossplane maturity is for Crossplane provide support for real-world applications. 
While the initial “Word Press” workload example is a real-world application it lacks the complexity which is typically 
common in other applications. 

GitLab self-hosted service is a real-world and real-complex application. 
Supporting _GitLab_ as a workload(s) represents a great opportunity and even greater challenge and 
at the same time even greater validation of Crossplane maturity. 


## Terminology

*   _CrossplaneController_ Kubernetes Operator application responsible for provisioning instances of specific 
applications into the Crossplane environment
*   _CrossplaneControllerCRD_ - Schema definition for given _CrossplaneController_
*   _CrossplaneController_Instance_ - result of CR submission for a specific _CrossplaneController_
*   _KubernetesApplication_ - a collection _KubernetesApplicationResources_ and resource references that 
collectively define an application and propagated to the target Kubernetes cluster (formerly known as Workload)
*   _KubernetesApplicationController_ - a Kubernetes operator responsible for reconciliation 
(main intent: CRUD of application components and status report) of _KubernetesApplication_ instances
*   _KubernetesApplicationResource_ - provides a definition for application payload items 
(formerly known as Workload payload: targetNamespace, targetDeployment, targetService)
*   _KubernetesApplicationResourceController_ - a Kubernetes operator responsible for reconciliation 
(main intent: propagation / status report) of _KubernetesApplicationResource_ instances
*   _GitLab_ - Set of Kubernetes artifacts that collectively represent GitLab Service deployed onto Kubernetes 
cluster (official helm chart)
*   _GitLabController_ - _CrossplaneController_ for _GitLab_
*   _GitLabControllerCRD_ - Schema for _GitLabController_
*   _GitLabControllerInstance - CrossplaneControllerInstance_ for _GitLab_
*   _GitLabApplication_ - a collection _KubernetesApplicationResources_ and resources dedicated or required 
for deploying and operating _GitLap_ application on Kubernetes cluster

## Motivation

While it is possible to create a Workload in terms of authoring a `yaml` file, and while it is easy to do for simple applications (Wordpress), it poses serious challenges for more complicated applications. 

For example, _GitLab application_ roughly<sup>1</sup> consists of:

*   10 deployments
*   1 stateful set
*   3 jobs
*   4 horizontal pod autoscale[rs]
*   9 services
*   18 config maps
*   33 secrets 
*   2 certificates
*   7 pod disruption budgets
*   4 role bindings
*   4 roles
*   10 service accounts
*   2 persistent volume claims

Authoring _GitLab_ as workload.yaml could be a tedious and error-prone process. 
Moreover, to provision, another _GitLab_ instance, would require authoring yet another workload.yaml with identical complexity.

In an optimistic case (leveraging defaults), the user has to provide only a hand full of attributes specific to this given _GitLab_ application instance. 

_GitLabController_ purpose is to simplify the provisioning of _GitLab_ applications by:

*   Providing the type definition that represents the input required to provision a _GitLab_ instance
*   Providing functionality to:
    *   Process the input
    *   Create the necessary crossplane managed resources 
    *   Create Workload (Application and Components)
    *   Report application status

_GitLabController_ is portable, i.e. cloud provider agnostic and the same type definition will work on either of crossplane supported cloud providers.

In addition to simplifying and delivering more reliable user experience, _GitLabController_ provides an interface that will be leveraged by Hosted Crossplane, with goals of further simplification and automation, achieving a “Single Click Deployment” user experience.

## Scope

Design for _GitLabController_ and _GitLabControllerCRD_ (Type)

##### Not in scope

*   Hosted crossplane design
*   Crossplane Installer design
*   KubernetesApplication/ApplicationComponent design
*   _GitLabController_ registry packaging 

## Strategy

Use GitLab Helm deployments as a base for GitLab application design.

*   Render helm chart into gitlab.yaml file
*   Split Cluster level components into separate stand-alone _CrossplaneControllers_:
    *   Lets Encrypt
    *   External DNS
    *   Prometheus
*   Create MinIO Gateway _KubernetesApplication_ (or possibly CrossplaneController)
*   Create _GitLabKubernetesApplication_

## Requirements

### Portability

GitLab application must produce portable workloads, i.e. the same workload must run on either support cloud providers without any modifications.

### Environment

GitLabController has set of expectation for the crossplane environment. These expectations must be satisfied before creating _GitLabApplication_ instance

## Risks

_GitLabController_ depends on few components outside of its scope:

*   _KubernetesApplication_ and _KubernetesApplicationResource_(s)
*   Upbound Registry

While the _GitLabController_ Design currently being proposed it operates on terms like ‘CrossplaneController’ with the assumption that there will be additional applications solutions architected with this design. 

### Portability

#### GitLab

Helm installation is initially not portable, i.e. not cloud provider agnostic in the bucket credentials area:

*   Connection secret
*   s3cmd.config

~~MinIO Gateway could be used as a portable layer, however, we need to have MinIO Gateway crossplane application deployed into the same target Kubernetes cluster.~~

#### MinIO Gateway

MinIO Gateway itself is not portable and requires a different configuration set between different supported cloud providers.

*   AWS S3 example:

```
export MINIO_ACCESS_KEY=aws_s3_access_key
export MINIO_SECRET_KEY=aws_s3_secret_key
minio gateway s3
```

*   GCP GCS example:

```
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/credentials.json
export MINIO_ACCESS_KEY=minioaccesskey
export MINIO_SECRET_KEY=miniosecretkey
minio gateway gcs yourprojectid
```

### Crossplane (hosted)

A running crossplane application is required for GitLab Application installation. The crossplane must be configured to support the following crossplane artifacts:

*   Cloud Providers
    *   Cloud provider must be created and deployed to Crossplane.
    *   Cloud provider name will be passed as an input (Global)
*   Resource Classes 
    *   Resource classes must be created and deployed to Crossplane.
    *   Resource classes dependencies (secrets) must be created and deployed to Crossplane.
    *   Resource classes names:
        *   Bucket
        *   Kubernetes 
        *   Postgresql
        *   Redis
    *   Question: Crossplane [does not support “default” resource classes](https://github.com/crossplaneio/crossplane/issues/151) yet. What is the mechanism for the GitLab controller to discover resource classes and match them to resource claims?
*   Other: leaving TBD, to be updated or removed

### Crossplane ApplicationControllers

GitLabApplication will be limited to deploying only GitLab components. Any/All cluster level components will be (should be) removed and refactored into separate CrossplaneControllers.

Those CrossplaneControllers will submit respective KubernetesApplications, which are expected to be deployed into the same target Kubernetes cluster as the GitLab application. In the future, this case will be represented by Controller dependency, where GitLabController depends on IngressControllerController, LetsEncryptController, etc.

The list of controllers/applications can be refined as we go through the process. Currently, the dependency list as follows:

*   Lets Encrypt
*   External DNS
*   ~~Ingress Controller~~
    * Ingress Controller relies on GitLab specific functionality (TCP) and must be part of the gitlab application
*   Prometheus

## GitLabController CR

_GitLabController_ is “very” opinionated about what _GitLab _application should look like, in terms of GitLab components and resource dependencies. Taking into consideration that GitLabControllerCRD will be submitted from the UpboundCloud (front-end), there is a heavy emphasis to keep this schema as concise as possible. 


```yaml
apiVersion: controller.gitlab.com/v1alpha1
kind: GitLab
metadata:
  name: gitlab-demo
  namespace: default
spec:
  # cluster selector where this gitlab will be propagated to
  clusterSelector:
    matchLabels:
      app: gitlab-demo-gke
  # cluster namespace where to deploy gitlab components. defaults to "default"
  clusterNamespace: gitlab
  # domain (required) is used for DNS records
  domain: upbound.io
  # email (required) is used in certmanager-issuer
  email: joe.shmoe@gitlab.com
  # hostSuffix (optional) is used in dns names: 'host-hostSuffix.domain'
  # if not provided results in 'host.domain'
  hostSuffix: demo
  # providerRef (required)
  providerRef:
    name: demo-gcp
  # reclaimPolicy - if not provided defaults to Retain
  reclaimPolicy: Delete
```

That’s all there is to it :)

The rest of the required GitLab required information will be statically provided by _GitLabController_ (via code or controller configuration). 


## Notes

1: POC GitLab Helm chart deployment into GKE w/ managed crossplane resources
