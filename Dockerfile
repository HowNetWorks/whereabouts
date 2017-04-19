FROM golang:1.8-alpine

COPY . /go/src/whereabouts
RUN adduser -D app && chown -R app:app /go

USER app
WORKDIR /go/src/whereabouts
RUN go build

CMD ./whereabouts -host 0.0.0.0