#!/bin/bash

#This script controls the execution of a single client machine

#Parameters:
#modelname: the name of the model the client will request from the server and train on
#dataset: the name of the dataset the client starts with
#clientnumber: each client is issued a unique number, this number is appended to it's id, and is appended to the name of it's dataset to determine the specific dataset that the client will user
#latency: artifical latenct to inject on each request
#adversary: True/False, if true the client will act as an adversary
#tor: True/False, if true the client will connect through tor, if not a regular tcp connection is opened
#serverip: The ip of the server, only used in the non tor case
#onionservice: name of the servers hidden onion service, used in the tor case
#diffpriv: the ammount of differential privacy the client wants

modelname=$1
dataset=$2
clientnumber=$3
latency=$4
adversary=$5
tor=$6
servername=$7
onionservice=$8
diffpriv=$9

#TODO check that each of these parameters are sane



truedatasetname=$dataset$clientnumber

if [ $adversart = true ];then
    truedatasetname=$truedatasetname_b
else
    truedatasetname=$truedatasetname_g
fi

if [ -e $truedatasetname ];then
    echo starting client with the $truedataset dataset
else
    echo $truedatasetname does not exist
    exit
fi

#export go paths
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin
export PATH=$PATH:/usr/local/go/bin

echo Resetting Tor
killall tor;
tor & sleep 15;
cd go/src/github.com/wantonsolutions/TorML/DistSys/

#run the client
go run torclient.go $HOSTNAME-$clientnumber $modelname $truedatasetname $tor $servername $onionservice $diffpriv
