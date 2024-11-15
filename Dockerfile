FROM golang:1.23.1-alpine3.20 AS builder
RUN apk update && apk add --no-cache git

LABEL authors="alturino"

WORKDIR /usr/app/ecommerce/

COPY ["go.mod", "go.sum", "./"]
RUN go mod download

COPY main.go  ./
COPY ./internal/ ./internal/
COPY ./cmd/ ./cmd/
COPY ./user/ ./user/

RUN go build main.go

FROM alpine:3.20.3 AS production
RUN apk add --no-cache dumb-init

WORKDIR /usr/app/ecommerce/

RUN addgroup --system go && adduser -S -s /bin/false -G go go

COPY --chown=go:go --from=builder /usr/app/ecommerce/main ./ecommerce
COPY --chown=go:go ./env/ ./env/
COPY --chown=go:go ./migrations/ ./migrations/

RUN mkdir -p /var/log/ && chown -R go:go /var/log/

RUN touch /var/log/ecommerce.log && chown -R go:go /var/log/ecommerce.log
RUN touch /var/log/order-service.log && chown -R go:go /var/log/order-service.log
RUN touch /var/log/user-service.log && chown -R go:go /var/log/user-service.log
RUN touch /var/log/notification-service.log && chown -R go:go /var/log/notification-service.log
RUN touch /var/log/shop-service.log && chown -R go:go /var/log/shop-service.log
RUN touch /var/log/product-service.log && chown -R go:go /var/log/product-service.log

USER go
CMD [ "dumb-init", "./ecommerce" ]

