CRD_OPTIONS ?= "crd:trivialVersions=true,crdVersions=v1"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: vendor codegen manifests build

run:
	./sample-controller

vendor: tidy
	go mod vendor

tidy:
	go mod tidy

codegen:
	./hack/update-codegen.sh

manifests: controller-gen yq
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths=k8s.io/sample-controller/pkg/apis/samplecontroller/v1alpha1 rbac:roleName=sample-controller-manager-role crd:trivialVersions=true output:artifacts:config=helm/templates
	yq -i eval "(.metadata.annotations[\"api-approved.kubernetes.io\"] |= \"unapproved, experimental-only\")" helm/templates/samplecontroller.k8s.io_vms.yaml 

controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

yq:
ifeq (, $(shell which yq))
	@{GO111MODULE=on go install github.com/mikefarah/yq/v4@latest}
endif

build:
	go build
