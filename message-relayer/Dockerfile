# syntax = docker/dockerfile:experimental
# Build Container
FROM golang:1.17.1 as builder

ENV GO111MODULE on
ENV GOPRIVATE=bitbucket.org/latonaio

WORKDIR /go/src/bitbucket.org/latonaio

COPY . .

RUN git config --global url.git@bitbucket.org:.insteadOf https://bitbucket.org/
RUN mkdir /root/.ssh/ && touch /root/.ssh/known_hosts && ssh-keyscan -t rsa bitbucket.org >> /root/.ssh/known_hosts
RUN --mount=type=secret,id=ssh,target=/root/.ssh/id_rsa go mod download
RUN go mod tidy

RUN go build


# Runtime Container
FROM alpine:3.14

RUN apk add --no-cache libc6-compat

ENV SERVICE=message-relayer-golang \
    POSITION=Runtime \
    AION_HOME="/var/lib/aion" \
    APP_DIR="${AION_HOME}/${POSITION}/${SERVICE}"

WORKDIR ${AION_HOME}

COPY --from=builder /go/src/bitbucket.org/latonaio/message-relayer .

CMD ["./message-relayer"]