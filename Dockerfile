FROM golang:1.23-alpine as builder
RUN mkdir /build
ADD /lib /build/lib
ADD /go.mod /build/
ADD /go.sum /build/
ADD /go.work /build/
ADD *.go /build/
WORKDIR /build
RUN GOOS=linux GOARCH=amd64 go build -o pocketbase


# generate clean, final image for end users
FROM alpine:latest

RUN apk add --no-cache \
    unzip \
    ca-certificates

# download https://drive.google.com/file/d/1TgFbkVaHUqB3OvBVXB4ZMBNPN_Et-WHj/view?usp=sharing
# Create a directory for PocketBase
RUN mkdir -p /pb

COPY --from=builder /build/pocketbase /pb/pocketbase

# Make the binary executable
RUN chmod +x /pb/pocketbase

EXPOSE 8080

# start PocketBase
CMD ["/pb/pocketbase", "serve", "--http=0.0.0.0:8080"]