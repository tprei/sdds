FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY services/api ./services/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/mailsink ./services/api/cmd/mailsink

FROM alpine:3.22

RUN adduser -D -H -u 10001 sdds

COPY --from=build /out/mailsink /usr/local/bin/mailsink

USER sdds
EXPOSE 8090
ENV SDDS_MAILSINK_ADDR=:8090

ENTRYPOINT ["mailsink"]
