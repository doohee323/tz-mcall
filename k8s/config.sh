#!/usr/bin/env bash

set -x

GIT_BRANCH=$1
STAGING=$2

export GIT_BRANCH=$(echo ${GIT_BRANCH} | sed 's/\//-/g')
GIT_BRANCH=$(echo ${GIT_BRANCH} | cut -b1-21)

rm -Rf etc/mcall.yaml
#echo aws s3 cp s3://${DOCKER_NAME}-${CLUSTER_NAME}/config/${STAGING}/mcall.yaml etc/mcall.yaml --profile ${CLUSTER_NAME}
#aws s3 cp s3://${DOCKER_NAME}-${CLUSTER_NAME}/config/${STAGING}/mcall.yaml etc/mcall.yaml --profile ${CLUSTER_NAME}
#sed -i -e "s|GIT_BRANCH|${GIT_BRANCH}|" etc/mcall.yaml

if [[ "${GIT_BRANCH}" == "block" ]]; then
  cp -Rf etc/block_access.yaml etc/mcall.yaml
elif [[ "${GIT_BRANCH}" == "access" ]]; then
  cp -Rf etc/allow_access.yaml etc/mcall.yaml
fi

cat etc/mcall.yaml
