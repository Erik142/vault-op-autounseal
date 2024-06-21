FROM golang:alpine AS builder
RUN apk add --no-cache make
WORKDIR /go/src/app
COPY ./ ./
RUN make

FROM alpine
WORKDIR /root/
COPY --from=builder /go/src/app ./app
CMD ["./app/vault-op-autounseal"]
