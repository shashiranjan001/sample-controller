## extended sample-controller

The extended sample-controller implements an operator for a VM resource kind.

### Running 

##### 1. Run the mock cloud API server.

```sh
docker run -d --name cloud-server --restart unless-stopped --net host nascarsayan/cloud-server:latest
```

When the docker container starts, it will accept requests on port 9001.
Note down the IP of the host machine where we are running the container.
Let's assume the private cloud API endpoint is listening to requests at http://10.10.10.10:9001

##### 2. Install the helm package for the sample-controller

```sh
helm upgrade --install sample-controller helm/ --create-namespace --set cloudApiUrl="http://10.10.10.10:9001"
```

This simple script can be used to create 10 VMs. yq should be installed on the system.

```sh
for i in $(seq -f "%04g" 1 10); do yq e "(.metadata.name |= \"vm-sample-$i\") | (.spec.vmName |= \"vm-sample-$i\")" artifacts/examples/example-vm.yaml | ka -; done
```

### Assignment discussions

The discussions for all the assignments can be found in Markdown format [here](./docs/assignments).

The PDFs have been generated from these markdown files using `md-to-pdf`

```sh
npm i -g md-to-pdf
md-to-pdf file.md # Creates file.pdf in the same directory where file.md is present.
```

### Development

The root directory should be named `sample-controller` and placed under a directory with name `k8s.io`,
so that the module name `k8s.io/sample-controller` gets resolved into the correct directory by `codegen`.

Run `make`. It will download the go modules (GO111MODULE should be on by default in go1.17),
generate the informers, listers and clientset for our custom k8s resources, and create the binary `sample-controller`

You can run `sample-controller` locally, but please copy `.env.sample` to `.env` and set the environment variables correctly.
The default values for those environment variables should work except for one: `CLOUD_API_URL`.
`CLOUD_API_URL` should be set to the private cloud API URL endpoint.

### Instrumentation

Currently only very basic golang metrics are exposed in the `/metrics` endpoint.
We should add **workqueue related metrics**, **reflector related metrics** and **k8s client related metrics** metrics too.<br/>
All these metrics are exposed by default in [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime/tree/master/pkg/metrics). A kubernetes operator scaffolded using kubebuilder will have all these metrics exposed.

To keep the sample-controller simple all these metrics were not added. Just the following metrics related to our controller have been added:
- **Max Concurrent Reconcilers**: The number of worker threads for the controller.
- **Workqueue Retries Total**: The number of times a workitem was added to the workqueue again for retrying after it failed due to transient errors.

The latency for the REST requests have been added to the instrumentation.
This helps us to categorize the latency of the controller, i.e., how much time the controller took in executing the code paths vs how much time was spent by the controller to wait for response from the private cloud server.
It gives us more clarity on the performance bottlenecks, so that we can work on improvement of the service as a whole.
- **Cloud API Request Latency** was added for this purpose.

A sample snapshot from the '/metrics' endpoint has been captured in this file:
[Sample Metrics](./docs/assignments/test-k8s/metrics.prom.log)
