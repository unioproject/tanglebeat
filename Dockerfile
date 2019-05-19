FROM golang:1.7.3
WORKDIR /
# RUN go get -d -v golang.org/x/net/html
COPY tanglebeat/ .
RUN go get -d github.com/unioproject/tanglebeat/tanglebeat
RUN go get -d github.com/unioproject/tanglebeat/tbsender
RUN go get -d github.com/unioproject/tanglebeat/examples/readnano
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o tanglebeat .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=0 tanglebeat .
CMD ["./tanglebeat"]
