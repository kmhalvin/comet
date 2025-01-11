FROM golang:1.22-alpine as build
WORKDIR /
COPY go.* .
RUN go mod download
COPY . .
RUN go build -o app github.com/kmhalvin/comet/cmd/ssh

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /app /usr/local/bin/app
ENTRYPOINT ["/usr/local/bin/app"]
