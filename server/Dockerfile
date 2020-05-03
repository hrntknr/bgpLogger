FROM golang:1.14-alpine

RUN mkdir -p /go/src/github.com/hrntknr/bgpLogger/server

COPY . /go/src/github.com/hrntknr/bgpLogger/server

WORKDIR /go/src/github.com/hrntknr/bgpLogger/server

RUN go build

CMD [ "./server", "--config", "config.yml" ]
