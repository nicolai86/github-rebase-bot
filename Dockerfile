FROM golang:1.8.3-alpine3.6
RUN apk --no-cache --update add git
RUN go get github.com/nicolai86/github-rebase-bot

FROM alpine:3.6

RUN apk --no-cache --update add ca-certificates git curl && update-ca-certificates

ENV GITHUB_TOKEN="" \
    GITHUB_OWNER="" \
    GITHUB_REPOS="" \
    GITHUB_MERGE_LABEL="LGTM" \
    PUBLIC_DNS=""

COPY --from=0 /go/bin/github-rebase-bot /
ADD startup.sh /

ENTRYPOINT ["/startup.sh"]
