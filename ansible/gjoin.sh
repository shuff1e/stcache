httpPort=$(cat run.hosts | awk -F '=' '/httpPort/ {print $2}')
echo "curl http://$1:$httpPort/gossipjoin/$2"
curl "http://$1:$httpPort/gossipjoin/$2"