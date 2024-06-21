FROM golang:latest
WORKDIR /go/src/app
COPY ./ ./
RUN make
CMD ["vault-op-autounseal"]
