FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY . .
RUN go build -o /out/forgedb ./cmd/forgedb

FROM alpine:3.20
WORKDIR /data
COPY --from=build /out/forgedb /usr/local/bin/forgedb
RUN chmod 0755 /usr/local/bin/forgedb
ENTRYPOINT ["forgedb"]
