#!/bin/bash

# Define variables
DEBUG=false
HELP=false

numberOfPeers=10
maxNumberOfMessagesPerPeer=2
maxNumberPrivateMessagesPerPeer=1
UIPort=12345
minimumGossipPort=5000
maxNumberOfFiles=3
maxNumberOfChunksPerFile=10
RTimer=5
TimeToWait=20

TestRouting=false
TestPrivateMessages=false
TestFileIndexing=false
TestFileSharing=false
TestFile=false
AllowWarning=true

# Handle command-line arguments
while [[ $# -gt 0 ]]
do
    key="$1"

    case $key in
        -v|--verbose)
            echo "Mode debug"
            DEBUG=true
            ;;
        -h|--help)
            HELP=true
            ;;
        -p|--number-peers)
            shift
            numberOfPeers="$1"
            ;;
        -u|--ui-port)
            shift
            UIPort="$1"
            ;;
        -g|--gossip-port)
            shift
            minimumGossipPort="$1"
            ;;
        -r|--route-timer)
            shift
            RTimer="$1"
            TestRouting=true
            ;;
        --test-routing)
            TestRouting=true
            ;;
        -f|--file-sharing)
            TestFileSharing=true
            TestFile=true
            ;;
        -i|--file-indexing)
            TestFileIndexing=true
            TestFile=true
            ;;
        -m|--private-messages)
            TestPrivateMessages=true
            ;;
        -w|--disable-warnings)
            AllowWarning=false
            ;;
        -a|--all)
            TestRouting=true
            TestPrivateMessages=true
            TestFileIndexing=true
            TestFileSharing=true
            TestFile=true
            ;;
        -t|--wait-time)
            shift
            TimeToWait="$1"
            ;;
        *)
            # unknown option
            ;;
    esac
    shift
done

# Define colors for our outputs
BLACK='\033[0;30m'
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
GRAY='\033[0;37m'
WHITE='\033[0;97m'
NC='\033[0m'
WARNINGCOLOR=$YELLOW
if [[ $AllowWarning = false ]]
then
    WARNINGCOLOR=$RED
fi

failed=false
warning=false

check_failed() {
    if [[ $failed = true ]]
    then
        echo -e "${RED}***FAILED***${NC}"
    else
        echo -e "${GREEN}***PASSED***${NC}"
    fi
}

check_warning() {
    if [[ $warning = true ]]
    then
        if [[ $AllowWarning = true ]]
        then
            echo -e "${YELLOW}***   CHECK YOUR IMPLEMENTATION   ***${NC}"
            echo -e "${YELLOW}***SOMETHING MIGHT HAVE GONE WRONG***${NC}"
        else
            echo -e "${RED}***FAILED***${NC}"
        fi
    else
        echo -e "${GREEN}***PASSED***${NC}"
    fi
}

if [[ $HELP = true ]]
then
    echo "usage: ./test_generic.sh [ OPTIONS ]

      -h | --help               Display this help message
      -v | --verbose            Increase the details of the displayed messages
      -p | --number-peers       Specify the number of peers to launch. Default is 10.
      -u | --ui-port            Specify the first UI port to use. Default is 12345.
      -g | --gossip-port        Specify the first gossip port to use. Default is 5000.
      -r | --route-timer        Define the value for the -rtimer flag. Implies testing the routing.
                                Default is 0.
      -a | --all                Enable all the checks
      --test-routing            Test the routing part.
      -i | --file-indexing      Test the file indexing part.
      -f | --file-sharing       Test the file sharing part.
      -m | --private-messages   Test the private messages part.
      -w | --disable-warnings   Trigger failure instead of warning when a private message or a
                                file-related message doesn't reach its destination.
      -t | --wait-time          Time to wait for processes to communicate, before checking the output files.
    "
else
    if [[ $numberOfPeers -le 1 ]]
    then
        echo "${RED}******** You need at least two processes to communicate ************"
        exit 0
    fi
    # Build server then client
    cd ..
    go build
    cd client
    go build
    cd ..
    # Interrupt all the processes, in case any of them is still running
    pkill -f Peerster
    rm ./tests/out/*.out

    outputFiles=()
    names=()
    UIPorts=()

    secondSender=2
    thirdSender=4
    messagesSentThird=0

    firstPrivateSender=1
    firstPrivateDest=5
    secondPrivateSender=7
    secondPrivateDest=0
    thirdPrivateSender=1
    thirdPrivateDest=0
    privateSentThird=0

    if [[ $numberOfPeers -lt 8 ]]
    then
        secondPrivateSender=2
        if [[ $numberOfPeers -lt 6 ]]
        then
            firstPrivateDest=2
            if [[ $numberOfPeers -lt 5 ]]
            then
                thirdSender=1
                if [[ $numberOfPeers -lt 3 ]]
                then
                    secondSender=1
                    thirdSender=0
                    messagesSentThird=2
                    maxNumberOfMessagesPerPeer=3

                    firstPrivateDest=0
                    secondPrivateSender=0
                    secondPrivateDest=1
                    privateSentThird=1
                    maxNumberPrivateMessagesPerPeer=2
                fi
            fi
        fi
    fi

    declare -A messages
    messages[0,0]=Weather_is_clear
    messages[0,1]=No_clouds_really
    messages[$secondSender,0]=Winter_is_coming
    messages[$secondSender,1]=Let\'s_go_skiing
    messages[$thirdSender,$messagesSendThird]=Is_anybody_here?

    if [[ $TestPrivateMessages = true ]]
    then
        declare -A private_messages
        private_messages[$firstPrivateSender,$firstPrivateDest,0]=Night_is_falling
        private_messages[$secondPrivateSender,$secondPrivateDest,0]=Sun_is_rising
        private_messages[$thirdPrivateSender,$thirdPrivateDest,$privateSentThird]=Go_to_bed
    fi

    if [[ $TestFile = true ]]
    then
        declare -A files
        files[0,1,0,0]="test"
        files[0,1,0,1]="f0ec718935d04e358e4235d3dd5fd5b69cb6bc1e6943310a6ea2312a7068bfb9"
    fi

    name='A'

    # Launch peers as a circle
    for i in `seq 0 $(($numberOfPeers - 1))`
    do
        outFileName="$name.out"
        peerPort=$(($(($i + 1)) % $numberOfPeers + $minimumGossipPort))
        gossipPort=$(($i + $minimumGossipPort))
        peer="127.0.0.1:$peerPort"
        gossipAddr="127.0.0.1:$gossipPort"
        if [[ $TestRouting = false ]]
        then
            if [[ $DEBUG = true ]]
            then
                echo "./Peerster -UIPort=$UIPort -gossipAddr=$gossipAddr -name=$name -peers=$peer > ./tests/out/$outFileName &"
            fi
            ./Peerster -UIPort=$UIPort -gossipAddr=$gossipAddr -name=$name -peers=$peer > "./tests/out/$outFileName" &
        else
            if [[ $DEBUG = true ]]
            then
                echo "./Peerster -UIPort=$UIPort -gossipAddr=$gossipAddr -name=$name -peers=$peer -rtimer=$RTimer > $outFileName &"
            fi
            ./Peerster -UIPort=$UIPort -gossipAddr=$gossipAddr -name=$name -peers=$peer -rtimer=$RTimer > "./tests/out/$outFileName" &
        fi
        outputFiles+=("./tests/out/$outFileName")
        names+=("$name")
        if [[ "$DEBUG" == "true" ]] ; then
            echo "$name running at UIPort $UIPort and gossipPort $gossipPort"
        fi
        UIPorts+=($UIPort)
        UIPort=$(($UIPort+1))
        name=$(echo "$name" | tr "A-Y" "B-Z")
    done

    sleep 3
    # Send messages
    ./client/client -UIPort=${UIPorts[0]} -msg="${messages[0,0]}"
    ./client/client -UIPort=${UIPorts[$secondSender]} -msg="${messages[$secondSender,0]}"
    sleep 5
    if [[ $TestPrivateMessages = true ]]
    then
        # Private message
        ./client/client -UIPort=${UIPorts[$firstPrivateSender]} -msg="${private_messages[$firstPrivateSender,$firstPrivateDest,0]}" -dest="${names[$firstPrivateDest]}"
    fi
    ./client/client -UIPort=${UIPorts[0]} -msg="${messages[0,1]}"
    sleep 3
    ./client/client -UIPort=${UIPorts[$secondSender]} -msg="${messages[$secondSender,1]}"
    ./client/client -UIPort=${UIPorts[$thirdSender]} -msg="${messages[$thirdSender,$messagesSentThird]}"
    sleep 3
    if [[ $TestPrivateMessages = true ]]
    then
        ./client/client -UIPort=${UIPorts[$secondPrivateSender]} -msg="${private_messages[$secondPrivateSender,$secondPrivateDest,0]}" -dest="${names[$secondPrivateDest]}"
        ./client/client -UIPort=${UIPorts[$thirdPrivateSender]} -msg="${private_messages[$thirdPrivateSender,$thirdPrivateDest,$privateSentThird]}" -dest="${names[$thirdPrivateDest]}"
    fi
    if [[ $TestFile = true ]]
    then
        # File
        ./client/client -UIPort=${UIPorts[0]} -file="${files[0,1,0,0]}"


        if [[ $TestFileSharing ]]
        then
            # Wait for the node to have indexed the file
            sleep 10
            # Request the file
            ./client/client -UIPort=${UIPorts[1]} -file="${files[0,1,0,0]}2" -request="${files[0,1,0,1]}" -dest="${names[0]}"
        fi
    fi

    # Wait for the nodes to communicate with one another
    sleep $TimeToWait
    # Interrupt all the processes
    pkill -f Peerster

    #######################
    # Testing correctness #
    #######################

    # Client messages
    failed=false
    if [[ $DEBUG = true ]]
    then
        echo -e "${BLUE}###CHECK that client messages arrived${NC}"
    fi
    for i in `seq 0 $(($numberOfPeers - 1))`
    do
        for j in `seq 0 $(($maxNumberOfMessagesPerPeer - 1))`
        do
            if [[ ${messages[$i,$j]} != "" ]] && !(grep -q "CLIENT MESSAGE ${messages[$i,$j]}" "${outputFiles[$i]}")
            then
                failed=true
                if [[ "$DEBUG" == "true" ]] ; then
                    echo -e "${RED}CLIENT MESSAGE ${messages[$i,$j]} not present in ${outputFiles[$i]}${NC}"
                fi
            fi
        done
    done
    check_failed
    failed=false

    # Client private messages
    if [[ $TestPrivateMessages = true ]]
    then
        if [[ $DEBUG = true ]]
        then
            echo -e "${BLUE}###CHECK that client private messages arrived${NC}"
        fi
        for i in `seq 0 $(($numberOfPeers - 1))`
        do
            for j in `seq 0 $(($numberOfPeers - 1))`
            do
                for k in `seq 0 $(($maxNumberPrivateMessagesPerPeer - 1))`
                do
                    line="SENDING PRIVATE MESSAGE ${private_messages[$i,$j,$k]} TO ${names[$j]}"
                    if [[ ${private_messages[$i,$j,$k]} != "" ]] && !(grep -q "$line" "${outputFiles[$i]}")
                    then
                        failed=true
                        if [[ "$DEBUG" == "true" ]] ; then
                            echo -e "${RED}$line not present in ${outputFiles[$i]}${NC}"
                        fi
                    fi
                done
            done
        done
        check_failed
        failed=false
    fi

    if [[ $TestFileIndexing = true ]]
    then
        # Client file indexing messages
        if [[ $DEBUG = true ]]
        then
            echo -e "${BLUE}###CHECK that client file-indexing messages arrived${NC}"
        fi
        for i in `seq 0 $(($numberOfPeers - 1))`
        do
            for j in `seq 0 $(($numberOfPeers - 1))`
            do
                for k in `seq 0 $(($maxNumberOfFiles - 1))`
                do
                    line="REQUESTING INDEXING filename ${files[$i,$j,$k,0]}"
                    if [[ ${files[$i,$j,$k,0]} != "" ]] && !(grep -q "$line" "${outputFiles[$i]}")
                    then
                        failed=true
                        if [[ "$DEBUG" == "true" ]] ; then
                            echo -e "${RED}$line not present in ${outputFiles[$i]}${NC}"
                        fi
                    fi
                done
            done
        done
        check_failed
        failed=false
    fi

    if [[ $TestFileSharing = true ]]
    then
        # Client file requesting messages
        if [[ $DEBUG = true ]]
        then
            echo -e "${BLUE}###CHECK that client file-requesting messages arrived${NC}"
        fi
        for i in `seq 0 $(($numberOfPeers - 1))`
        do
            for j in `seq 0 $(($numberOfPeers - 1))`
            do
                for k in `seq 0 $(($maxNumberOfFiles - 1))`
                do
                    if [[ ${files[$j,$i,$k,0]} != "" ]]
                    then
                        line="REQUESTING filename ${files[$j,$i,$k,0]}2 from ${names[$j]} hash ${files[$j,$i,$k,1]}"
                        if !(grep -q "$line" "${outputFiles[$i]}")
                        then
                            failed=true
                            if [[ "$DEBUG" == "true" ]] ; then
                                echo -e "${RED}$line not present in ${outputFiles[$i]}${NC}"
                            fi
                        fi
                    fi
                done
            done
        done
        check_failed
        failed=false
    fi

    # Rumor messages
    if [[ $DEBUG = true ]]
    then
        echo -e "${BLUE}###CHECK rumor messages ${NC}"
    fi
    for i in `seq 0 $(($numberOfPeers - 1))`
    do
        # for each peer, check that all messages have been seen as rumor
        for j in `seq 0 $(($numberOfPeers - 1))`
        do
            for k in `seq 0 $(($maxNumberOfMessagesPerPeer - 1))`
            do
                if [[ ${messages[$j,$k]} != "" ]] && [[ $i != $j ]]
                then
                    line="RUMOR origin ${names[$j]} from 127.0.0.1:[0-9]{4} ID [0-9]+ contents ${messages[$j,$k]}"
                    if !(grep -Eq "$line" "${outputFiles[$i]}") ; then
                        failed=true
                        if [[ $DEBUG = true ]] ; then
                            echo -e "${RED}$line not present in ${outputFiles[$i]}${NC}"
                        fi
                    fi
                fi
            done
        done
    done
    check_failed
    failed=false

    # Mongering
    if [[ $DEBUG = true ]]
    then
        echo -e "${BLUE}###CHECK mongering ${NC}"
    fi
    for i in `seq 0 $(($numberOfPeers - 1))`
    do
        # for each node, check that it has mongered with its two peers
        for j in "$(( $(($i-1+$numberOfPeers)) % $numberOfPeers + $minimumGossipPort ))" "$(( $(($i+1)) % $numberOfPeers + $minimumGossipPort ))"
        do
            if !(grep -q "MONGERING with 127.0.0.1:$j" "${outputFiles[$i]}")
            then
                failed=true
                if [[ $DEBUG = true ]]
                then
                    echo -e "${RED}Node ${names[$i]} did not monger with peer ${names[$(($j - $minimumGossipPort))]}${NC}"
                    echo -e "${RED}MONGERING with 127.0.0.1:$j not present in ${outputFiles[$i]}${NC}"
                fi
            fi
        done
    done
    check_failed
    failed=false

    # Check status messages
    if [[ $DEBUG = true ]]
    then
        echo -e "${BLUE}###CHECK status messages ${NC}"
    fi
    for i in `seq 0 $(($numberOfPeers - 1))`
    do
        patterns=()
        patterns+=("STATUS from 127.0.0.1:$(( $(($i-1+$numberOfPeers)) % $numberOfPeers + $minimumGossipPort ))"  "STATUS from 127.0.0.1:$(( $(($i+1)) % $numberOfPeers + $minimumGossipPort ))")
        for j in `seq 0 $(($numberOfPeers - 1))`
        do
            for k in `seq 1 $(($maxNumberOfMessagesPerPeer - 1))`
            do
                if [[ ${messages[$j,$k]} != "" ]]
                then
                    patterns+=("peer ${names[$j]} nextID $(($k+1))")
                fi
            done
        done
        for pattern in "${patterns[@]}"
        do
            if !(grep -q "$pattern" "${outputFiles[$i]}")
            then
            failed=true
            if [[ $DEBUG = true ]]
            then
                echo -e "${RED}$pattern not present in ${outputFiles[$i]}${NC}"
            fi
        fi
        done
    done
    check_failed
    failed=false

    # Check flipped coins
    if [[ $DEBUG = true ]]
    then
        echo -e "${BLUE}###CHECK flipped coin${NC}"
    fi
    for i in `seq 0 $(($numberOfPeers - 1))`
    do
        for j in "$(( $(($i-1+$numberOfPeers)) % $numberOfPeers + $minimumGossipPort ))" "$(( $(($i+1)) % $numberOfPeers + $minimumGossipPort ))"
        do
            if !(grep -q "FLIPPED COIN sending rumor to 127.0.0.1:$j" "${outputFiles[$i]}")
            then
                warning=true
                if [[ $DEBUG = true ]]
                then
                    echo -e "${WARNINGCOLOR}Node ${names[$i]} did not flip a coin and sent a rumor to peer ${names[$(($j - $minimumGossipPort))]}${NC}"
                fi
            fi
        done
    done
    check_warning
    warning=false

    # Check in sync
    if [[ $DEBUG = true ]]
    then
        echo -e "${BLUE}###CHECK in sync${NC}"
    fi
    for i in `seq 0 $(($numberOfPeers-1))`
    do
        for j in "$(( $(($i-1+$numberOfPeers)) % $numberOfPeers + $minimumGossipPort ))" "$(( $(($i+1)) % $numberOfPeers + $minimumGossipPort ))"
        do
            if !(grep -q "IN SYNC WITH 127.0.0.1:$j" "${outputFiles[$i]}")
            then
                failed=true
                if [[ $DEBUG = true ]]
                then
                    echo -e "${RED}Node ${names[$i]} not in sync with peer ${names[$(($j - $minimumGossipPort))]}${NC}"
                    echo -e "${RED}IN SYNC WITH 127.0.0.1:$j not present in ${outputFiles[$i]}${NC}"
                fi
            fi
        done
    done
    check_failed
    failed=false

    # Check correct peers
    if [[ $DEBUG = true ]]
    then
        echo -e "${BLUE}###CHECK correct peers${NC}"
    fi
    for i in `seq 0 $(($numberOfPeers-1))`
    do
        portBelow="$(( $(($i-1+$numberOfPeers)) % $numberOfPeers + $minimumGossipPort ))"
        portAbove="$(( $(($i+1)) % $numberOfPeers + $minimumGossipPort ))"
        if ([[ $numberOfPeers -gt 2 ]] && !(grep -q "127.0.0.1:$portBelow,127.0.0.1:$portAbove" "${outputFiles[$i]}") &&  !(grep -q "127.0.0.1:$portAbove,127.0.0.1:$portBelow" "${outputFiles[$i]}") ) || !(grep -q "127.0.0.1:$portBelow" "${outputFiles[$i]}")
        then
            failed=true
            if [[ $DEBUG = true ]]
            then
                echo -e "${RED}Node ${names[$i]} has not the right peers${NC}"
            fi
        fi
    done
    check_failed
    failed=false

    if [[ $TestRouting = true ]]
    then
        # Check update routing table
        if [[ $DEBUG = true ]]
        then
            echo -e "${BLUE}###CHECK update routing table${NC}"
        fi
        for i in `seq 0 $(($numberOfPeers-1))`
        do
            portBelow="$(( $(($i-1+$numberOfPeers)) % $numberOfPeers + $minimumGossipPort ))"
            portAbove="$(( $(($i+1)) % $numberOfPeers + $minimumGossipPort ))"
            for j in `seq 0 $(($numberOfPeers-1))`
            do
                if [[ ${messages[$j,0]} != "" ]] && [[ $j != $i ]]
                then
                    if !(grep -q "DSDV ${names[$j]} 127.0.0.1:$portBelow" "${outputFiles[$i]}") &&  !(grep -q "DSDV ${names[$j]} 127.0.0.1:$portAbove" "${outputFiles[$i]}")
                    then
                        failed=true
                        if [[ $DEBUG = true ]]
                        then
                            echo -e "${RED}Node ${names[$i]} does not update the routing table${NC}"
                            echo -e "${RED}'DSDV ${names[$j]} 127.0.0.1:$portBelow' and 'DSDV ${names[$j]} 127.0.0.1:$portAbove' not present in ${outputFiles[$i]}${NC}"
                        fi
                    fi
                fi
            done
        done
        check_failed
        failed=false
    fi

    if [[ $TestPrivateMessages = true ]]
    then
        # Check private messages
        if [[ $DEBUG = true ]]
        then
            echo -e "${BLUE}###CHECK private messages${NC}"
        fi
        for i in `seq 0 $(($numberOfPeers-1))`
        do
            for j in `seq 0 $(($numberOfPeers-1))`
            do
                for k in `seq 0 $(($maxNumberPrivateMessagesPerPeer - 1))`
                do
                    if [[ ${private_messages[$j,$i,$k]} != "" ]]
                    then
                        if !(grep -Eq "PRIVATE origin ${names[$j]} hop-limit [0-9]+ contents ${private_messages[$j,$i,$k]}" "${outputFiles[$i]}")
                        then
                            warning=true
                            if [[ $DEBUG = true ]]
                            then
                                echo -e "${WARNINGCOLOR}Node ${names[$i]} does not receive private messages${NC}"
                                echo -e "${WARNINGCOLOR}'PRIVATE origin ${names[$j]} hop-limit [0-9]+ contents ${private_messages[$j,$i,$k]}' not present in ${outputFiles[$i]}${NC}"
                            fi
                        fi
                    fi
                done
            done
        done
        check_warning
        warning=false
    fi

    if [[ $TestFileSharing = true ]]
    then
        # Check file downloading
        if [[ $DEBUG = true ]]
        then
            echo -e "${BLUE}###CHECK file downloading${NC}"
        fi
        for i in `seq 0 $(($numberOfPeers-1))`
        do
            for j in `seq 0 $(($numberOfPeers-1))`
            do
                for k in `seq 0 $(($maxNumberOfFiles-1))`
                do
                    # Check the start of a download
                    if [[ ${files[$j,$i,$k,0]} != "" ]]
                    then
                        if !(grep -q "DOWNLOADING metafile of ${files[$j,$i,$k,0]}2 from ${names[$j]}" "${outputFiles[$i]}")
                        then
                            warning=true
                            if [[ $DEBUG = true ]]
                            then
                                echo -e "${WARNINGCOLOR}Node ${names[$i]} does not receive metafile of ${files[$j,$i,$k,0]} from ${names[$j]}${NC}"
                            fi
                        fi

                        if [[ $warning != true ]]
                        then
                            # Check each chunk
                            for l in `seq 1 $maxNumberOfChunksPerFile`
                            do
                                if [[ ${files[$j,$i,$k,$l]} != "" ]] && !(grep -q "DOWNLOADING ${files[$j,$i,$k,0]}2 chunk $l from ${names[$j]}" "${outputFiles[$i]}")
                                then
                                    warning=true
                                    if [[ $DEBUG = true ]]
                                    then
                                        echo -e "${WARNINGCOLOR}Node ${names[$i]} does not receive chunk $l of ${files[$j,$i,$k,0]} from ${names[$j]}${NC}"
                                    fi
                                fi
                            done

                            # Check the full reception of the file
                            if [[ $warning != true ]] && !(grep -q "RECONSTRUCTED file ${files[$j,$i,$k,0]}2" "${outputFiles[$i]}")
                            then
                                warning=true
                                if [[ $DEBUG = true ]]
                                then
                                    echo -e "${WARNINGCOLOR}Node ${names[$i]} does not reconstruct file ${files[$j,$i,$k,0]} from ${names[$j]}${NC}"
                                fi
                            fi
                            if [[ $warning != true ]] && [[ $(diff "./client/_Downloads/${files[$j,$i,$k,0]}2" "./client/_SharedFiles/${files[$j,$i,$k,0]}") != "" ]]
                            then
                                warning=true
                                if [[ $DEBUG = true ]]
                                then
                                    echo -e "${WARNINGCOLOR}Reconstructed file ${files[$j,$i,$k,0]} does not match the original${NC}"
                                fi
                            fi
                        fi
                    fi
                done
            done
        done
        check_warning
        warning=false
    fi
fi
