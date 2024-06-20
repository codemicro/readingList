FROM golang:1 as builder

RUN mkdir /build
ADD . /build/
WORKDIR /build

RUN CGO_ENABLED=1 GOOS=linux go build -a -buildvcs=false -installsuffix cgo -ldflags "-extldflags '-static'" -o main git.tdpain.net/codemicro/readingList/cmd/readinglistd

FROM alpine

COPY --from=builder /build/main /main
WORKDIR /run

CMD ["/main"]
