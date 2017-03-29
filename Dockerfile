FROM alpine:3.5

RUN apk --no-cache --update add ca-certificates git && update-ca-certificates

ENV GITHUB_TOKEN=""
ADD ./github-rebase-bot /

ENTRYPOINT ["/github-rebase-bot"]
