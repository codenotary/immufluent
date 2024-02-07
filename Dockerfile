FROM golang:1.21-alpine as builder
LABEL maintainer="Simone Lazzaris <simone@codenotary.com>"
RUN apk add make
WORKDIR /app
COPY go.* .
RUN go mod download -x
COPY . .
RUN make

FROM scratch as runner
WORKDIR /app
COPY --from=builder /app/immufluent .
EXPOSE 8090
ENTRYPOINT ["/app/immufluent"]

