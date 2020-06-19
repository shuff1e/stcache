#!/usr/bin/env bash

node_number=$1

function usage() {
    echo "Usage:"
    echo "   run.sh nodeNumber(1-5)"
}

if ! [[ "${node_number}" =~ ^[1-5]$ ]] ; then
    usage
    exit 1
fi

node="node${node_number}"
node_http=$((5999 + $node_number))
node_raft=$((6999 + $node_number))
node_gossip=$((7999 + $node_number))

args="${args} --node=${node}"
args="${args} --http=127.0.0.1:${node_http}"
args="${args} --raft=127.0.0.1:${node_raft}"
args="${args} --gossip=127.0.0.1:${node_gossip}"
args="${args} --verificationScript=./verify.sh"
args="${args} --notificationScript=./notify.sh"
args="${args} --eventInterval=60"

if [ "${node_number}" == 1 ] ; then
    args="${args} --bootstrap=true"
else
    args="${args} --join=127.0.0.1:6000 --gossipJoin=127.0.0.1:8000"
fi

echo${args}
../bin/CouchbaseHAService ${args}

