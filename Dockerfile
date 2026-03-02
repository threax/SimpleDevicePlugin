FROM golang AS build

COPY . .
RUN go build

FROM debian
COPY --from=build /go/fpga-plugin fpga-plugin
ENTRYPOINT ["/fpga-plugin"]