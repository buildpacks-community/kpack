FROM golang:1.16-alpine3.14

RUN apk add gcc pkgconfig libc-dev make
RUN apk add --no-cache libgit2-dev~=1.1

# Use the GitHub Actions uid:gid combination for proper fs permissions
RUN addgroup -g 116 -S test && adduser -u 1001 -S -g test test
USER test

ENTRYPOINT ["/bin/sh", "-c"]