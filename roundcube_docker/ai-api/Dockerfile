# ITBS PNP Mail - Go AI API
# OpenAI-compatible AI proxy used by the Roundcube AI plugin.
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/itbs-ai-api ./cmd/server

FROM alpine:3.20
RUN adduser -D -H -u 10001 appuser
COPY --from=build /out/itbs-ai-api /usr/local/bin/itbs-ai-api
EXPOSE 8090
HEALTHCHECK --interval=30s --timeout=5s --retries=5 CMD wget -qO- http://127.0.0.1:8090/healthz || exit 1
USER appuser
CMD ["itbs-ai-api"]
