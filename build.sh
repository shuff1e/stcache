if [ "Linux" != "$(uname -s)" ];then
	printf "\\nWARNING !!! MacOs doesn't supply to use this script\\n"
#    exit 1
fi

output=./bin
#rm -rf ${output}/*
rm -rf ${output}/CouchbaseHAService

#gcc -Wall -O3 scripts/hypervisor.c -o ${output}/hypervisor -lpthread

export GOPATH=`pwd` && \
cd src/couchbase-ha-service && \
export GOOS=linux && go build -v -o "../../bin/CouchbaseHAService"
