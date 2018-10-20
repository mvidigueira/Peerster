#!/usr/bin/env bash

cd ..
go build
cd client
go build
cd ..

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'
DEBUG="true"

outputFiles=()
message_c1_1=Weather_is_clear
message_c2_1=Winter_is_coming
message_c1_2=No_clouds_really
message_c2_2=Let\'s_go_skiing
message_c3=Is_anybody_here?


UIPort=12345
gossipPort=5000
name='A'

# General peerster (gossiper) command
#./Peerster -UIPort=12345 -gossipAddr=127.0.0.1:5001 -name=A -peers=127.0.0.1:5002 > A.out &

for i in `seq 1 10`;
do
	outFileName="$name.out"
	peerPort=$((($gossipPort+1)%10+5000))
	peer="127.0.0.1:$peerPort"
	gossipAddr="127.0.0.1:$gossipPort"
	./Peerster -UIPort=$UIPort -gossipAddr=$gossipAddr -name=$name -peers=$peer > "./tests/out/$outFileName" &
	outputFiles+=("./tests/out/$outFileName")
	if [[ "$DEBUG" == "true" ]] ; then
		echo "$name running at UIPort $UIPort and gossipPort $gossipPort"
	fi
	UIPort=$(($UIPort+1))
	gossipPort=$(($gossipPort+1))
	name=$(echo "$name" | tr "A-Y" "B-Z")
done

sleep 1

pkill -f Peerster

#test
failed="F"
echo -e "${RED}###CHECK sent route messages ${NC}"

gossipPort=5000
for i in `seq 0 9`;
do
	relayPort=$(($gossipPort-1))
	if [[ "$relayPort" == 4999 ]] ; then
		relayPort=5009
	fi
	nextPort=$((($gossipPort+1)%10+5000))
    msgLine1="MONGERING with 127.0.0.1:\($relayPort\|$nextPort\)"

	if !(grep -Eq "$msgLine1" "${outputFiles[$i]}") ; then
        failed="T"
        if [[ "$DEBUG" == "true" ]] ; then
		    echo -e "${RED}Missing at ${outputFiles[$i]} ${NC}"
	    fi
    fi

	gossipPort=$(($gossipPort+1))
done

if [[ "$failed" == "T" ]] ; then
    echo -e "${RED}***FAILED***${NC}"
else
    echo -e "${GREEN}***PASSED***${NC}"
fi


failed="F"
echo -e "${RED}###CHECK dsdv messages ${NC}"
gossipPort=5000
count=0
for i in `seq 0 9`;
do
    relayPort=$(($gossipPort-1))
    if [[ "$relayPort" == 4999 ]] ; then
        relayPort=5009
    fi
    nextPort=$((($gossipPort+1)%10+5000))

	msgLine1="DSDV \[A-J\] 127.0.0.1:\($relayPort\|$nextPort\)"

    count = $count + $(grep -c "$msgLine1" "${outputFiles[$i]}")

	gossipPort=$(($gossipPort+1))
done

if [[ $count != 10 ]] ; then
    echo -e "${RED}***FAILED***${NC}"
    if [[ "$DEBUG" == "true" ]] ; then
		echo -e "${RED} Count was $count < 10 ${NC}"
	fi
else
    echo -e "${GREEN}***PASSED***${NC}"
fi