FROM golang:1.11 as builder
LABEL description="golang, gb ad a vim working environment"
MAINTAINER mowings@turbosquid.com
RUN go get github.com/constabulary/gb/...
COPY . /app
ENV GOPATH=/go:/app:/app/vendor
RUN cd /app/src/loghog  && go build
FROM ubuntu
RUN mkdir /app
COPY --from=builder /app/src/loghog/loghog /app
# Set up stuff for logstash consumption
# certs need to be provided externally
COPY logstash/log.sh /app/log.sh
COPY logstash/logstash-forwarder.gz /app/
RUN gzip -d /app/logstash-forwarder.gz
CMD  ["/app/loghog"]
