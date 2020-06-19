key=$1
value=$2
if [ ${#key} -gt ${#value} ];then echo UP; else echo DOWN; fi
