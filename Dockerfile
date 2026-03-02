FROM golang AS build

COPY . .
RUN go build

FROM debian
COPY --from=build /go/sdp-plugin sdp-plugin
ENTRYPOINT ["/sdp-plugin"]