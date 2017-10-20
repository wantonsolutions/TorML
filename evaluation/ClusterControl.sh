#!/bin/bash -ex

vms[0]=40.86.185.245

bootstrap='rm TorMentorAzureInstall.sh; wget https://raw.githubusercontent.com/wantonsolutions/TorML/master/evaluation/TorMentorAzureInstall.sh; chmod 755 TorMentorAzureInstall.sh; sudo sh -c "yes | ./TorMentorAzureInstall.sh"'
pingcommand='ping -c 5 198.162.52.147 | tail -1 | cut -d / -f 5 > $HOSTNAME.ping'
pinglocation="/home/stew/*ping"

sysdir='go/src/github.com/wantonsolutions/TorML/DistSys/'

runclient='tor & sleep 3;cd go/src/github.com/wantonsolutions/TorML/DistSys/; go build torclient.go; ./torclient $HOSTNAME models credit1'

function onall {
    echo running $1
    for vm in ${vms[@]}
    do
        ssh stew@$vm -x $1 
    done
}

function onallasync {
    echo running $1
    for vm in ${vms[@]}
    do
        ssh stew@$vm -x $1 &
    done
}

function getall {
    echo grabbing $1
    for vm in ${vms[@]}
    do
        scp stew@$vm:$1 $2
    done
}


#onall "$bootstrap"
onall "$pingcommand"
onall "$runclient"
getall "$pinglocation" ./
