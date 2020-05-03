FROM golang:1.14-alpine

RUN mkdir -p /go/src/github.com/hrntknr/bgpLogger/daemon

COPY . /go/src/github.com/hrntknr/bgpLogger/daemon

WORKDIR /go/src/github.com/hrntknr/bgpLogger/daemon

RUN go build

CMD [ "./daemon", "--config", "config.yml" ]
