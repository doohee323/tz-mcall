##
## Build
##
FROM golang:1.16-buster AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./
COPY etc/mcall.yaml ./

RUN go build

##
## Deploy
##
FROM ubuntu:18.04

WORKDIR /app

COPY --from=build /app/tz-mcall /app/tz-mcall
COPY --from=build /app/mcall.yaml /app/mcall.yaml

RUN export PATH=/app:.:$PATH
RUN mkdir -p /app/log
RUN /bin/chmod -Rf 777 /app

#RUN apt update
#RUN apt install curl -y

CMD ["/app/tz-mcall", "-c=/app/mcall.yaml"]
#CMD ["/bin/sh", "-c", "while true; do sleep 24h; done"]
