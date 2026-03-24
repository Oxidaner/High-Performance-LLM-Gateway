FROM golang:1.25 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/llm-gateway ./cmd/server

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /out/llm-gateway /app/llm-gateway
COPY configs /app/configs

EXPOSE 8080
ENTRYPOINT ["/app/llm-gateway"]
