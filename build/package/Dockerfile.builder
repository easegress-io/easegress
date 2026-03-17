# Builder image for easegress
FROM golang:1.26.1-alpine
RUN apk --no-cache add make git
