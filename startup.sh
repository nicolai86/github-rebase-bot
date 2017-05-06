#!/bin/sh

function get_public_dns() {
  curl -s -ik -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" https://kubernetes.default.svc.cluster.local/api/v1/namespaces/github/services/rebase-bot | grep "ip" | awk '{print $2}' | awk -F'"' '{print $2}'
}

# only determine k8s IP if not set explicitly
if [[ "z$PUBLIC_DNS" == "z" ]]; then
  PUBLIC_DNS=$(get_public_dns)
  while [[ "z$PUBLIC_DNS" == "z" ]]; do
    echo "Waiting for service to come up"
    sleep 2
    PUBLIC_DNS=$(get_public_dns)
  done
fi

/github-rebase-bot \
 -owner $GITHUB_OWNER \
 -repo $GITHUB_REPO \
 -public-dns http://$PUBLIC_DNS \
 -merge-label $GITHUB_MERGE_LABEL \
 -addr :8080 \
 -mainline $GITHUB_MAINLINE
