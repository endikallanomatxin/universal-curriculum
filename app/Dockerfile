FROM public.ecr.aws/docker/library/golang:1.26.2 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/migrate ./cmd/migrate

FROM docker.io/library/debian:bookworm-slim

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
RUN groupadd -r appuser && useradd -r -g appuser appuser

WORKDIR /app

COPY --from=builder /out/server /usr/local/bin/server
COPY --from=builder /out/migrate /usr/local/bin/migrate
COPY web ./web
RUN mkdir -p /app/uploads && chown -R appuser:appuser /app/uploads

EXPOSE 8080

USER appuser

CMD ["/usr/local/bin/server"]
