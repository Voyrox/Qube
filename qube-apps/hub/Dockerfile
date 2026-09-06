FROM golang:1.24.5-alpine AS builder

WORKDIR /src

COPY hub/go.mod hub/go.sum ./hub/
WORKDIR /src/hub
RUN go mod download

WORKDIR /src
COPY hub ./hub
COPY Website ./Website

WORKDIR /src/hub
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/qubehub .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/qubehub /app/qubehub
COPY --from=builder /src/hub/static /app/static
COPY --from=builder /src/hub/templates /app/templates
COPY --from=builder /src/Website /app/Website

ENV ADDR=:32003

EXPOSE 32003

ENTRYPOINT ["/app/qubehub"]
