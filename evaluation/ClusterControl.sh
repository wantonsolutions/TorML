#!/bin/bash -ex

declare -A vms

username='stewbertgrant'

bootstrap='rm TorMentorAzureInstall.sh;
           wget https://raw.githubusercontent.com/wantonsolutions/TorML/master/evaluation/TorMentorAzureInstall.sh;
           chmod 755 TorMentorAzureInstall.sh;
           sudo sh -c "yes | ./TorMentorAzureInstall.sh"'

firstcommand='echo hello $HOSTNAME'
pingcommand='ping -c 1 198.162.52.147 | tail -1 | cut -d / -f 5 > $HOSTNAME.ping'
pinglocation="/home/stewbertgrant/*ping"
permission="sudo chown -R stewbertgrant go"


sysdir='go/src/github.com/wantonsolutions/TorML/DistSys/'

runclient='export GOPATH=$HOME/go
           export PATH=$PATH:$GOPATH/bin;
           export PATH=$PATH:/usr/local/go/bin;
           killall tor;
           tor & sleep 10;
           cd go/src/github.com/wantonsolutions/TorML/DistSys/;
           go run torclient.go $HOSTNAME models credit1;'
           #./torclient $HOSTNAME models credit1'

pull='cd go/src/github.com/wantonsolutions/TorML/DistSys/; git pull'

killeverything='killall torserver; killall torclient'

## $1 is the filename to read vms from
function readVMs {
IFS=$'\n'
set -f
    for line in $(cat $1);do
        vmname=`echo $line | cut -d, -f1`
        vmpubip=`echo $line | cut -d, -f2`
        vms["$vmname"]="$vmpubip"
    done
echo ${vms[@]}
}

function yeshello {
    for vm in ${vms[@]}
    do
        ssh $username@$vm -oStrictHostKeyChecking=no -x 'echo $HOSTNAME'
    done
}

function onall {
    echo running $1
    for vm in ${vms[@]}
    do
        ssh $username@$vm -x $1 
        break
    done
}

function onallasync {
    echo running $1
    for vm in ${vms[@]}
    do
        ssh $username@$vm -x $1 &
    done
}

function getall {
    echo grabbing $1
    for vm in ${vms[@]}
    do
        scp $username@$vm:$1 $2
    done
}
function getallasync {
    echo grabbing $1
    for vm in ${vms[@]}
    do
        scp $username@$vm:$1 $2 &
    done
}

function getPings {
    onallasync "$pingcommand"
    sleep 10
    #onall "$runclient"
    getallasync "$pinglocation" ./
    sleep 10
    cat *.ping > agg.ping
    mkdir ping
    mv *.ping ping/
}

function installAll {
    onallasync "$bootstrap"
}

function killAll {
    onallasync "$killeverything"
}


readVMs nameip.txt
killAll
#onallasync "$runclient"
#yeshello
#installAll
#getPings
#onall "$bootstrap"

#onallasync "$pull"
#onallasync "$permission"
#onallasync "$pull"
#onall "$firstcommand"
#onall "$pingcommand"
#sleep 10
#onall "$runclient"
#getall "$pinglocation" ./
