FROM golang:1.26 AS build
WORKDIR /src
COPY gobao-pkg gobao-pkg
COPY gobao-proto gobao-proto
COPY gobao-order gobao-order
ENV GOWORK=off
RUN cd /src/gobao-order && go build -o /out/server ./cmd/server

FROM gcr.io/distroless/base-debian12
COPY --from=build /out/server /server
EXPOSE 8080 9090
ENTRYPOINT ["/server"]
