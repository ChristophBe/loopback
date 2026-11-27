FROM golang:1.26 AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download
COPY backend ./backend
RUN cd backend && CGO_ENABLED=0 go build -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -o /out/import ./cmd/import

FROM golang:1.26 AS dev
RUN go install github.com/air-verse/air@latest
WORKDIR /src/backend
CMD ["air", "-c", ".air.toml"]

FROM gcr.io/distroless/static-debian12 AS prod
COPY --from=build /out/api /out/import /
COPY api/gamification.yaml /api/gamification.yaml
ENV GAMIFICATION_FILE=/api/gamification.yaml
ENTRYPOINT ["/api"]
