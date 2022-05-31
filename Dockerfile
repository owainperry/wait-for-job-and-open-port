FROM golang:1.17-alpine AS builder
WORKDIR /app
COPY . ./
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o ./wait-for-job-and-open-port
ENTRYPOINT [ "/wait-for-job-and-open-port" ]

FROM alpine:latest  
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/wait-for-job-and-open-port ./
CMD ["./wait-for-job-and-open-port"]  