FROM golang:1.24.5-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/qube .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/qube /app/qube
COPY --from=builder /src/index.html /app/index.html
COPY --from=builder /src/footer.png /app/footer.png

EXPOSE 32002

ENTRYPOINT ["/app/qube"]
