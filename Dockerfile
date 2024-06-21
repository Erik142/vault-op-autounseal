FROM golang:alpine AS builder
RUN apk add --no-cache make
WORKDIR /root/
RUN go env -w GOMODCACHE=/root/.cache/go-build
COPY ./ ./
RUN --mount=type=cache,target=/root/.cache/go-build go mod download
RUN --mount=type=cache,target=/root/.cache/go-build go build -v -o vault-op-autounseal main.go

FROM alpine
WORKDIR /root/
COPY --from=builder /root/vault-op-autounseal ./
CMD ["./vault-op-autounseal"]
