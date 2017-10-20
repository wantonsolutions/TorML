#!/bin/bash

function killtor {
    killall torserver
    killall torclient
}

if [ "$1" == "-k" ];
then
    killtor
    exit
fi

killtor

CLIENTS=2


go run torserver.go &

sleep 3

go run torcurator.go c1 models


#for (( i=1; i<CLIENTS+1; i++ ))
#do
#    go run torclient.go h$i models credit$i &
#done
