# Multi-Stage Reproducible Build for BETTERGOV Layer 1 Node (bettergovd)
FROM golang:1.24-alpine AS builder
WORKDIR /src/chain
COPY chain/go.mod ./
COPY chain/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/bettergovd ./cmd/bettergovd

FROM alpine:3.20
RUN adduser -D -u 10001 bettergov
USER bettergov
WORKDIR /home/bettergov
COPY --from=builder /out/bettergovd /usr/local/bin/bettergovd
COPY genesis/devnet/genesis.json /home/bettergov/genesis.json
EXPOSE 8545 26656
ENTRYPOINT ["/usr/local/bin/bettergovd"]
CMD ["start", "--rpc-bind", "0.0.0.0:8545", "--admin-bind", "127.0.0.1:26657"]

