FROM golang:alpine AS builder
RUN apk add --no-cache make
WORKDIR /root/
RUN go env -w GOMODCACHE=/root/.cache/go-build
COPY ./ ./
RUN --mount=type=cache,target=/root/.cache/go-build go mod download
RUN --mount=type=cache,target=/root/.cache/go-build go build -v -o vault-onepassword-controller cmd/autounseal.go

FROM alpine
WORKDIR /root/
COPY --from=builder /root/vault-onepassword-controller ./
CMD ["./vault-onepassword-controller"]
