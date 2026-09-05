FROM node:24-bookworm-slim AS web-build

WORKDIR /src

COPY package.json package-lock.json components.json ./
RUN npm ci

COPY web web
RUN npm run build:web

FROM golang:1.26-bookworm AS build

WORKDIR /src

COPY go.work go.work
COPY server/go.mod server/go.sum* ./server/
COPY cli/go.mod ./cli/
RUN cd server && go mod download

COPY server server
COPY cli cli
COPY --from=web-build /src/server/internal/web/dist ./server/internal/web/dist
RUN cd server && CGO_ENABLED=0 GOOS=linux go build -o /out/wrkflw ./cmd/wrkflw
RUN cd cli && CGO_ENABLED=0 GOOS=linux go build -o /out/wrkflw-cli ./cmd/wrkflw

FROM gcr.io/distroless/static-debian12

WORKDIR /app
COPY --from=build /out/wrkflw /app/wrkflw
COPY --from=build /out/wrkflw-cli /app/wrkflw-cli

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/app/wrkflw"]
CMD ["serve"]
