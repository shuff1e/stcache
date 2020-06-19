httpPort=$(cat run.hosts | awk -F '=' '/httpPort/ {print $2}')
master=$(cat run.hosts | awk '/\[master\]/ {getline; print}')
master=$(curl -s -S http://$master:$httpPort/leader)
master=$(echo $master | awk -F : '{print $1}')
echo "curl http://$master:$httpPort/set?key=$1&value=$2"
curl "http://$master:$httpPort/set?key=$1&value=$2"