###############
# build stage #
###############
FROM golang:alpine as build

# disable CGo
ENV CGO_ENABLED 0

# install build tools required for static build
RUN apk add --no-cache git build-base alpine-sdk

# create and cd into source code directory
WORKDIR /go/src/datayoinker

# copy over module files
COPY go.mod .
COPY go.sum .

# copy over everything
# this is done to have git info at build time for versioninfo
COPY . .

# install dependencies
RUN go get -d -v

# build static binary
#RUN go build -v -tags 'osusergo netgo static static_build' -ldflags '-linkmode external -extldflags "-static"' -o /go/bin/datayoinker
RUN go build -v -tags 'osusergo netgo static static_build' -o /go/bin/datayoinker


#############
# run stage #
#############
FROM alpine

# create and cd into app directory
WORKDIR /app

# create data directory
#NOTE: not used yet
RUN mkdir /data

# change ownership of data and app dirs to the to-be create non-root user
#NOTE: /data is not being used yet
RUN chown -R 3333:3333 /app
RUN chown -R 3333:3333 /data

# add and use non-root user
RUN adduser -D -u 3333 -g 5000 app
USER app:app

# copy binary from build stage
COPY --from=build /go/bin/datayoinker /app/datayoinker

# expose default port
EXPOSE 3333

# declare data directory as a volume
#NOTE: not used yet
VOLUME /data

# define launch command
CMD ["/app/datayoinker"]
