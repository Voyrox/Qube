FROM golang:1.24.5-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/qubehub .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/qubehub /app/qubehub
COPY --from=builder /src/static /app/static
COPY --from=builder /src/templates /app/templates

ENV ADDR=:32003

EXPOSE 32003

ENTRYPOINT ["/app/qubehub"]
