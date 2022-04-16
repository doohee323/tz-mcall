#!/usr/bin/env bash

GIT_BRANCH=$1
STAGING=$2

export GIT_BRANCH=$(echo ${GIT_BRANCH} | sed 's/\//-/g')
GIT_BRANCH=$(echo ${GIT_BRANCH} | cut -b1-21)

rm -Rf ./mcall.yaml
echo aws s3 cp s3://${DOCKER_NAME}-${CLUSTER_NAME}/config/${STAGING}/mcall.yaml ./mcall.yaml --profile ${CLUSTER_NAME}
aws s3 cp s3://${DOCKER_NAME}-${CLUSTER_NAME}/config/${STAGING}/mcall.yaml ./mcall.yaml --profile ${CLUSTER_NAME}

sed -i -e "s|GIT_BRANCH|${GIT_BRANCH}|" ./mcall.yaml

cat ./mcall.yaml
