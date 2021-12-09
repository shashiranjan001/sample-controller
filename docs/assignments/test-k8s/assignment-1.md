## Assignment 1

### 1. Explain the yaml files present in `/master/artifacts/examples`:

A resource is a kubernetes object type that the kubernetes API Server understands. There are some known resources such as pods, deployments, jobs, etc. which are inbuilt in kubernetes. The `controller-manager` is responsible for the reconcilation of these resources.

In some situations the user wants to define business logic for declarative resources.<br/>
Let's take the example of VMs. Suppose the user wants to create a VM with 16GB memory, 4 vCPUs, 2 disks of size 100GB and attach one NIC connected to some network. The user specifies his intent as a Custom Resource (an resource of type VM). The prerequisite is that the kubernetes API server must first know about the type VM, which is basically the schema for VM resource type(analogous to the openAPI definition, or swagger, of an API endpoint).<br/>
Now that the intent is consolidated as a CR, we want someone to serve the user request. The kubernetes operators come into picture here.
A kubernetes operator watches for create, update and delete operations on the custom resources, and does the required operations to fulfil the request.
In typical scenarios, the reconciler has to construct and run some imperative commands, depending upon the current state of the resource, the outside world and as the desired state as specified by the user. The behavior has to be idempotent, i.e., an identical declarative intent should produce the same result every time.


i. `crd-status-subresource.yaml`

This yaml manifest contains the Custom Resource Definition for the Foo resource kind.
When we POST this maifest to the API server, a new CRD is registered with the kubernetes API.
Subsequently, we can create resources of Foo type on the same kubernetes cluster.

```yaml
# [resource: apiextensions.k8s.io]/[version: v1]
apiVersion: apiextensions.k8s.io/v1
# This type of this YAML manifest.
kind: CustomResourceDefinition
metadata:
  # The name of the Custom Resource Type.
  # [resource: foos].[group: samplecontroller.k8s.io]
  name: foos.samplecontroller.k8s.io
  annotations:
    # To make sure there are no conflicts between user-defined resources and kubernetes native resources in the parent group k8s.io, we need to provide this annotation.
    "api-approved.kubernetes.io": "unapproved, experimental-only; please get an approval from Kubernetes API reviewers if you're trying to develop a CRD in the *.k8s.io or *.kubernetes.io groups"
spec:
  # the group in which the CRD belongs to.
  group: samplecontroller.k8s.io
  # versions is the list of versions for our resource that the kubernetes API server knows of.
  versions:
    - name: v1alpha1
      # Whether this version will be served by the API server.
      served: true
      # Whether this version is stored in the etcd.
      storage: true
      schema:
        # schema used for validation.
        openAPIV3Schema:
          type: object
          properties:
            # The intent of the custom resource as specified by the user.
            spec:
              type: object
              properties:
                deploymentName:
                  type: string
                replicas:
                  type: integer
                  minimum: 1
                  maximum: 10
            # The current status of the custom resource.
            # The reconciler should work on making the status reflect the desired properties as mentioned in the spec.
            status:
              type: object
              properties:
                availableReplicas:
                  type: integer
      # Subresources are extra API endpoints provided by k8s.
      subresources:
        # enables the status subresource. We can perform CRUD directly on the following endpoint.
        # /apis/samplecontroller.k8s.io/v1alpha1/namespaces/<vm cr namespace>/vms/<vm cr name>/status
        # This is useful when we want to apply granular RBAC on the resources.
        status: {}
  names:
    # Kind is Foo
    kind: Foo
    # Resource is foos
    plural: foos
  # The resource is a namespaced resource.
  scope: Namespaced
```

ii. `crd.yaml`

This is similar to `crd-status-subresource.yaml`. The difference is that The status subresource is not enabled when this CRD is registered with the kubernetes API. We cannot perform CRUD (or RBAC) on just the status of a VM resource.

iii. `example-foo.yaml`

This is an example of a kubernetes resource of the Foo kind. 
After the Foo CRD is created, we can create a Foo resource in the cluster.<br/>
After the reconcilation of the following CR, a deployment named `example-foo` will be created:

- It will have 1 replica of pods.
- Each pod will have 1 container of image `nginx:latest`.
- The deployment will be owned by the `example-foo` CR. If there is any external changes made to the deployment (for example, if the deployment gets deleted or the number of replicas are updated externally), the controller will create an example-foo deployment again or update the existing deployment as needed.

```yaml
# The group and version for the resource.
# [group: samplecontroller.k8s.io]/[version: v1alpha1]
apiVersion: samplecontroller.k8s.io/v1alpha1
# The kind of the resource.
kind: Foo
metadata:
  # The name of the kubernetes resource.
  name: example-foo
spec:
  # The name of the deployment.
  deploymentName: example-foo
  # The number of replicas for the deployment.
  replicas: 1
```

### 2. We get the following error. Why? How to fix this?<br/>

`Error from server (NotFound): error when creating "artifacts/examples/example-foo.yaml":the server could not find the requested resource (post foos.samplecontroller.k8s.io)`

I tried running the given command. I error I got in response was slighlty different:<br/>
`error: unable to recognize "artifacts/examples/example-foo.yaml": no matches for kind "Foo" in version "samplecontroller.k8s.io/v1alpha1"`

The command we are trying to execute
```sh
kubectl apply -f artifacts/examples/example-foo.yaml &&\ # 1
kubectl apply -f artifacts/examples/crd.yaml # 2
```

What it means

1. We are making a reqeust to the API server to create the Foo Custom Resource `example-foo`.
2. We are making a reqeust to the API server to create the Foo Custom Resource Definition `foos.samplecontroller.k8s.io`.

The issue with our command is that the order of execution is wrong. We must first register our CRD with the API Server. Only after that, will kubernetes have the knowledge of this custom resource kind. After the API werver learns about this new resource type, we are free to do CRUD operations on resources of kind Foo.

We can fix this by the following sequence of logical steps:

i. We make a POST request to API Server to create the CRD of Foo kind.
ii. We wait till the new resource type gets registered in the kubernetes API.
iii. We then make a POST request to create the CR of Foo kind.

The command is the following:

```sh
kubectl apply -f artifacts/examples/crd.yaml
until kubectl get foos --all-namespaces ; do date; sleep 1; echo ""; done
kubectl apply -f artifacts/examples/example-foo.yaml
```

### 3. Please give us explanation about "what this sample-controller do" and "how this sample-controller work" when user create Foo resource.

### What sample-controller does

`sample-controller` is a wrapper on top of a hardcoded deployment resource. The user can only specify the number of replicas (no. of pods) for the deployment and the name of the deployment using a custom resource of Foo kind. Each pod in the deployment will be runnning one container of image `nginx:latest`. We can later update the number of replicas in the Foo CR, by invoking a PATCH request to the same API endpoint to change `.spec.replicas`, and the number of replicas for the corresponding deployment will change accordingly. If the number of replicas gets changed or the deployment owned by the Foo resoruce gets deleted externally, the controller will invoke required kubernetes API Server calls to bring the current state to the desired state.

### How sample-controller works

Using the following image that is already included in the repository for reference.
<p align="center">
  <img src="../../images/client-go-controller-interaction.jpeg" height="400"/>
</p>

Let's start by looking into the data structures that the controller has access to:

```go
type Controller struct {
  kubeclientset kubernetes.Interface
  sampleclientset clientset.Interface

  deploymentsLister appslisters.DeploymentLister
  foosLister        listers.FooLister

  deploymentsSynced cache.InformerSynced
  foosSynced        cache.InformerSynced
  
  workqueue workqueue.RateLimitingInterface
  
  recorder record.EventRecorder
}
```

In the `NewController` function, we can see the following lines:

```go
fooInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
  AddFunc: ...,
  UpdateFunc: ...,
})

deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
  AddFunc: ...,
  UpdateFunc: ...,
  DeleteFunc: ...,
}
```

i. It has two clientSets:
- `kubeclientset` for invoking write calls (create/update) on native kubernetes resource (in our case, only Deployments).
- `sampleclientset` for invoking write calls (create/update) on resources in the `samplecontroller.k8s.io` group (Foo is the only resource kind in this group).

ii. It has two listers (indexers):
- `deploymentsLister` for reading (list/watch) the deployments in all namespaces.
- `foosLister` for reading (list/watch) the foos resources in all namespaces.

iii. It initializes two informers and has reference to the HasSynced method for each of these informers:
- `fooInformer` for getting events when a resource of Foo Kind is created/updated/deleted.
- `deploymentInformer` for getting events when a resource of Deployment Kind is created/updated/deleted.

We hook custom functions to `AddFunc`, `UpdateFunc` and `DeleteFunc` before the controller is started, so that the informers run the respective function provided by us when a Foo/Deployment object is modified on the cluster.<br/>
The HasSynced method is invoked in the controller before processing any Events generated by the informers, so that we don't get stale data from the indexer on querying.

**The whole reason caching and indexing is done for kubernetes API objects is because watching these objects directly from the API Server is quite expensive.**<br/>
**Instead we can use a shared watcher, create a common indexer for responding to GET/LIST API queries, and generate events once for all the consumers.**

iv. `workqueue` is a FIFO queue for storing just the `Namespaced Name` for the modified API objects of kind Foo.<br/>
In case a deployment gets modified, our custom logic searches the owner of the deleted deployment, and if it is owned by a foo resource, that foo resource is enqueued.

v. `recorder` is used for generating important Kubernetes Events. The events are persisted along with the API Objects (unlike logs which get deleted if the Pod is deleted), and can be helpful for analysis.

We now know all the significant helper objects/functions which the controller uses in the reconciler loop. The remaining thing is just defining our business logic. We need to implement the function which will be called when the workqueue is non-empty. This function must be idempotent, and should be able to figure out what action to take by observing the state of the world and the intent specified by the user in `spec`. It should then update the `status` which will reflect the current state as known by the reconciler. Note that typically the `spec` should be read-only and the `status` should be write-only for the controller. For the user, it should be the opposite; the user can only specify his intent in `spec` and know the current state of the object from the `status`.

The sequence of operations is as follows:

1. On Create/Update events of Foo kind or on Create/Update/Delete events on deployments owned by a foo resource, the API Obejct Key (NS/Name) of the corresponding foo resource is enqueud to the workqueue.
2. We wait till the cache is synced for both the Foo Informer and the Deployment Informer.
3. We process the object keys for foos from the workqueue one by one.
4. For each object key, we get the corresponding foo resource. Then we take required action.
    - If the corresponding deployment resource does not exist, we create it. 
    - If the numebr of replicas of the deployment resource does not match the value in `spec`, we update thedeployment.
    - In case of unexpected situations, we take the call. If the issue is intermittent, and is retryable, we requeue the Job. If the issue can't be solved by us, or is out of our scope, we fail and log the error and a corresponding event.
