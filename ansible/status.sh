httpPort=$(cat run.hosts | awk -F '=' '/httpPort/ {print $2}')

keyArray=(status detectordata gossipstatus leader gossipnodes raftnodes membersconsensus)

declare -A keyMap
keyMap["s"]="status"
keyMap["d"]="detectordata"
keyMap["g"]="gossipstatus"
keyMap["l"]="leader"
keyMap["gn"]="gossipnodes"
keyMap["rn"]="raftnodes"
keyMap["mc"]="membersconsensus"

if [[ $# == 1 ]];then
    case $1 in
        0|1|2|3|4|5|6)  uri=${keyArray[$1]}
        ;;
        s|d|g|l|gn|rn|mc)  uri=${keyMap[$1]}
        ;;
        *)
        echo 'unknown mode'
        exit -1
        ;;
    esac
elif [[ $# == 2 ]];then
    case $2 in
        0|1|2)  uri=${keyArray[$2]}/$1
        ;;
        s|d|g)  uri=${keyMap[$2]}/$1
        ;;
        *)
        echo 'unknown mode'
        exit -1
        ;;
    esac
fi

for host in `cat run.hosts`;do
    if [[ $host == 10* ]];then
        echo $host
        if [ "$uri" == "leader" -o "$uri" == "membersconsensus" ];then
            curl "http://$host:$httpPort/$uri" -s -S
        else
            curl "http://$host:$httpPort/$uri" -s -S | python -mjson.tool
        fi
    fi
done
