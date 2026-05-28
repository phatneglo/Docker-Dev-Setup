FROM golang:1.24-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/postgres-wal-backup-api .

FROM postgres:17-bookworm

COPY --from=builder /out/postgres-wal-backup-api /usr/local/bin/postgres-wal-backup-api

EXPOSE 8080
ENTRYPOINT ["postgres-wal-backup-api"]
