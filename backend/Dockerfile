FROM golang:1 AS builder
WORKDIR /go
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /go/app

FROM golang:1 AS runner
COPY ./.env /opt/app/.env
COPY ./migrations /opt/app/migrations
COPY --from=builder /go/app /opt/app/app
EXPOSE 2727
WORKDIR /opt/app
CMD ["./app"]
