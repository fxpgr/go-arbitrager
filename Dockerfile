FROM golang:1.9

ENV GOBIN /go/bin
RUN mkdir -p /go/src/github.com/fxpgr/go-arbitrager
WORKDIR /go/src/github.com/fxpgr/go-arbitrager

COPY ./ /go/src/github.com/fxpgr/go-arbitrager
RUN go get -u github.com/golang/dep/...
RUN dep ensure
RUN go install ./...
