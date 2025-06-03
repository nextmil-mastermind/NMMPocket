FROM golang:1.24-alpine as builder
RUN mkdir /build
ADD /lib /build/lib
ADD /appform /build/appform
ADD /go.mod /build/
ADD /go.sum /build/
ADD /go.work /build/
ADD /authentication /build/authentication
ADD *.go /build/
ADD /zoomcon /build/zoomcon
WORKDIR /build
RUN GOOS=linux GOARCH=amd64 go build -o pocketbase


# generate clean, final image for end users
FROM alpine:latest

RUN apk add --no-cache \
    unzip \
    ca-certificates

# Create a directory for PocketBase
RUN mkdir -p /pb
RUN mkdir -p /pb/authhtml

COPY --from=builder /build/pocketbase /pb/pocketbase
COPY --from=builder /build/authentication/html/* /pb/authhtml/

# Make the binary executable
RUN chmod +x /pb/pocketbase

EXPOSE 8080

# start PocketBase
CMD ["/pb/pocketbase", "serve", "--http=0.0.0.0:8080"]