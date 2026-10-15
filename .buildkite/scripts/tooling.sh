#!/bin/bash

retry() {
    local retries=$1
    shift

    local count=0
    until "$@"; do
        exit=$?
        wait=$((2 ** count))
        count=$((count + 1))
        if [ $count -lt "$retries" ]; then
            >&2 echo "Retry $count/$retries exited $exit, retrying in $wait seconds..."
            sleep $wait
        else
            >&2 echo "Retry $count/$retries exited $exit, no more retries left."
            return $exit
        fi
    done
    return 0
}

unset_secrets () {
  for var in $(printenv | sed 's;=.*;;' | sort); do
    # vault-docker-login plugin (>= v0.8.1) needs VAULT_TOKEN in its pre-exit hook
    if [[ "$var" == "VAULT_TOKEN" ]]; then
      continue
    fi
    if [[ "$var" == *_SECRET || "$var" == *_TOKEN ]]; then
      unset "$var"
    fi
  done
}

running_on_buildkite() {
    if [[ "${BUILDKITE:-"false"}" == "true" ]]; then
        return 0
    fi
    return 1
}
