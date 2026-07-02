FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY services/api ./services/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./services/api/cmd/api

FROM alpine:3.22

RUN adduser -D -H -u 10001 sdds

COPY --from=build /out/api /usr/local/bin/api

USER sdds
EXPOSE 8080
ENV SDDS_HTTP_ADDR=:8080

ENTRYPOINT ["api"]
