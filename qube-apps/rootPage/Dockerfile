FROM golang:1.24.5-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/go-profile .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/go-profile /app/go-profile
COPY --from=builder /src/static /app/static
COPY --from=builder /src/public /app/public

ENV PORT=3000

EXPOSE 3000

ENTRYPOINT ["/app/go-profile"]
