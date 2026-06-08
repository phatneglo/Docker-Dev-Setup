FROM node:22-alpine AS frontend
WORKDIR /web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd/ ./cmd/
COPY --from=frontend /web/dist ./cmd/server/web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/mail-admin ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache docker-cli ca-certificates
COPY --from=build /out/mail-admin /usr/local/bin/mail-admin
EXPOSE 8092
CMD ["mail-admin"]
