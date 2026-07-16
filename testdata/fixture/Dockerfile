FROM golang:1.25-alpine AS build
COPY . .
RUN go build -o /out/app .

FROM gcr.io/distroless/static AS app
COPY --from=build /out/app /app
ENTRYPOINT ["/app"]
