httpPort=$(cat run.hosts | awk -F '=' '/httpPort/ {print $2}')
raftPort=$(cat run.hosts | awk -F '=' '/raftPort/ {print $2}')

echo "curl http://$1:$httpPort/leave?peerAddress=$2:$raftPort"
curl "http://$1:$httpPort/leave?peerAddress=$2:$raftPort"