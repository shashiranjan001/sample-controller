FROM golang:1.17 as builder

WORKDIR /workspace/

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY main.go main.go
COPY internal/ internal/
COPY pkg/ pkg/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o sample-controller main.go

FROM alpine
WORKDIR /
COPY --from=builder /workspace/sample-controller .

EXPOSE 8000
ENTRYPOINT ["/sample-controller"]
