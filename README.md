## extended sample-controller

The extended sample-controller implements an operator for a VM resource kind.

### Get started

#### 1. Run the mock cloud API server.

```sh
docker run -d --name cloud-server --restart unless-stopped --net host nascarsayan/cloud-server:latest
```

When the docker container starts, it will accept requests on port 9001.
Note down the IP of the host machine where we are running the container.
Let's assume the private cloud API endpoint is listening to requests at http://10.10.10.10:9001

#### 2. Install the helm package for the sample-controller

```sh
helm upgrade --install sample-controller helm/ --create-namespace --set cloudApiUrl="http://10.10.10.10:9001"
```

### Assignment discussions

The discussions for all the assignments can be found in Markdown format [here](./docs/assignments).

The PDFs have been generated from these markdown files using `md-to-pdf`

```sh
npm i -g md-to-pdf
md-to-pdf file.md # Creates file.pdf in the same directory where file.md is present.
```
