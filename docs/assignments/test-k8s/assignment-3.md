## Assignment 3

### 1. What kind of problem, risk we have if we run multiple extended sample-controllers?

If we run multiple replicas of extended sample-controllers, we will be facing a serious issue of duplicate work done by the replicas.<br/>
Since there are multiple instances of our controller running on the cluster, each instance will have its own workqueue.
Each instance will be adding Add and Update hooks to the informer.
Once a VM CR is modified (created/updated/deleted) on the cluster, the Informer will trigger the `AddFunc` or `UpdateFunc` for each instance.
Each instance will retrieve the desired spec and will start working on fulfilling the request.
There will be duplicate API calls to the private cloud, resulting in muliple create requests.
Overall, the system will be in a race condition. The first reconciler to hit the private cloud API is likely to get a success message, others will get failure messges.
We could face much more damage in case multiple VMs could be created with the same name, or if we support VM reconfiguration.

### 2. How to fix this issue? How does it work?

The issue we are facing here is similar to a train/flight ticket booking problem. It can be solved by making sure the work for one write is fulfilled by a single replica.<br/>
I could think of the following possible ways to make sure this happens:

#### 1. Replicated StatefulSet / Sharding
We could use something like a database sharding. Each replica will have its own set of responsibilities.<br/>

The following article does a similar replicated load balancing for MySQL:<br/>
https://kubernetes.io/docs/tasks/run-application/run-replicated-stateful-application

We could create a simple hash function that converts a string to a number in the range `[0 - (number_of_repicas - 1)]`:

```py
f(x) = sum([ascii(char) for char in string]) % number_of_repicas
```

i. We need some way to number Pods. StatefulSets can serve this purpose. We change the type of worload for our controller from Deployment to a StatefulSet. We can obtain the Pod Number and set it to an environment variable `POD_NUM`. This can be done using a simple bash script which is provided in the command for the container of the pod in the template.

ii. For an workqueue item, we hash the namespace using `f(x)`. If the hash value matches the `POD_NUM`, we are responsible for fulfilling the request. If it does not match, we forget the workqueue item.

Here, we have "sharded" the workload using the Namespace of the workqueue item. We could have used other strategies to group the workitems by Pod Number.

#### 2. Distributed Consensus

We use distributed consensus. We could have a leader among a set of replicas, who will be doing all the work. The other replicas will just be in standby mode.<br/>
The replicas will first start polling to decide who will become a leader. The leader has to communicate with the other replicas every `X` interval of time.
If the leader does not ping back after some retries, a new leader is elected. The Raft algorithm can be used for this purpose.
Kubernetes already has this algorithm implemented as a package in client-go, and we can reuse this algorithm.
We still need an id for each Pod while creating the lock. We can simply use the hostname of the Pod for this purpose.

I've used this method in the code to support running multiple replicas of the Pod.
