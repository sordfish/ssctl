FROM golang:1.20.4-alpine3.17 as build

WORKDIR /app
COPY . /app/
RUN go build -ldflags "-X ssctl/pkg/cli.Version=1.0.0" -o ssctl /app/cmd/cli/main.go

FROM alpine:3.17.0 as runtime

WORKDIR /app
COPY --from=build /app/ssctl /app/

CMD ["/app/ssctl"]