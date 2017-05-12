FROM alpine:3.5

RUN apk --no-cache --update add ca-certificates git curl && update-ca-certificates

ENV GITHUB_TOKEN="" \
    GITHUB_OWNER="" \
    GITHUB_REPOS="" \
    GITHUB_MERGE_LABEL="LGTM" \
    PUBLIC_DNS=""

ADD github-rebase-bot startup.sh /

ENTRYPOINT ["/startup.sh"]
