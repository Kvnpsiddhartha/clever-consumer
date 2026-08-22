FROM golang:1.23-bookworm AS build

WORKDIR /src/backend

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/clever-consumer ./cmd/main

FROM node:22-bookworm-slim

RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates wget \
	&& npm install -g @brightdata/cli \
	&& rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=build /out/clever-consumer /usr/local/bin/clever-consumer
COPY backend/migrations ./migrations

EXPOSE 18080

CMD ["clever-consumer"]
