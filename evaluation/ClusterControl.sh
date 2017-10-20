#!/bin/bash -ex

vms[0]=40.86.185.245

pingcommand='ping -c 5 198.162.52.147 | tail -1 | cut -d / -f 5 > $HOSTNAME.ping'
pinglocation="/home/stew/*ping"

function onall {
    echo running $1
    for vm in ${vms[@]}
    do
        ssh stew@$vm -x $1 
    done
}

function getall {
    echo grabbing $1
    for vm in ${vms[@]}
    do
        scp stew@$vm:$1 $2
    done
}



onall "$pingcommand"
getall "$pinglocation" ./
