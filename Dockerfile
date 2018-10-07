FROM golang:alpine
MAINTAINER "Aviv Laufer <aviv.laufer@gmail.com>"
RUN apk add --no-cache git build-base && \
    mkdir -p "$GOPATH/src/github.com/doitintl/kuberbs"

ADD . "$GOPATH/src/github.com/doitintl/kuberbs"

RUN go get github.com/golang/dep/cmd/dep && \
    cd "$GOPATH/src/github.com/doitintl/kuberbs" && \
    $GOPATH/bin/dep ensure && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a --installsuffix cgo --ldflags="-s -w"  -ldflags "-X main.version=$(git log | head -n 1 | cut  -f 2 -d ' ') -X main.builddate=$(date +%Y-%m-%d\-%H:%M)" -o /kuberbs
RUN strip /kuberbs

FROM alpine:3.4
RUN apk add --no-cache ca-certificates

COPY --from=0 /kuberbs /bin/kuberbs

ENTRYPOINT ["/bin/kuberbs"]
