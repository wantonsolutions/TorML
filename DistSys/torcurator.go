package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/DistributedClocks/GoVector/govec"
	"golang.org/x/net/proxy"
)

var LOCAL_HOST string = "127.0.0.1:5005"
var ONION_HOST string = "33bwoexeu3sjrxoe.onion:5005"

var TOR_PROXY_CLI string = "127.0.0.1:9050"
var TOR_PROXY_BROWSER string = "127.0.0.1:9150"

var (
	name       string
	modelName  string
	isLocal    bool
	minClients int
)

type MessageData struct {
	Type        string
	SourceNode  string
	ModelId     string
	Key         string
	NumFeatures int
	MinClients  int
}

func main() {
	parseArgs()
	logger := govec.InitGoVector(name, name)
	torDialer := getTorDialer()

	sendCurateMessage(logger, torDialer)

}

func parseArgs() {
	flag.Parse()
	inputargs := flag.Args()
	if len(inputargs) < 3 {
		fmt.Println("USAGE: go run torcurator.go curatorName modelName minclients")
		os.Exit(1)
	}

	var err error
	name = inputargs[0]
	modelName = inputargs[1]
	minClients, err = strconv.Atoi(inputargs[2])

	checkError(err)

	if len(inputargs) > 3 {
		fmt.Println("Running locally.")
		isLocal = true
	}

	fmt.Println("Done parsing args.")

}

//Todo this is duplicated on curator and client
func getTorDialer() proxy.Dialer {

	if isLocal {
		return nil
	}

	// Create proxy dialer using Tor SOCKS proxy via browser socket
	torDialerBrowser, errBrowser := proxy.SOCKS5("tcp", TOR_PROXY_BROWSER, nil, proxy.Direct)
	if errBrowser != nil {
		fmt.Printf("Unable to connect to TOR via Browser gateway %s\n", errBrowser.Error())
	} else {
		return torDialerBrowser
	}

	// Create proxy dialer using Tor SOCKS proxy via browser socket
	torDialerCLI, errCLI := proxy.SOCKS5("tcp", TOR_PROXY_CLI, nil, proxy.Direct)
	if errCLI != nil {
		fmt.Printf("Unable to connect to TOR via CLI gateway %s\n", errCLI.Error())
	} else {
		return torDialerCLI
	}
	checkError(fmt.Errorf("Unable to connect through BROWSER %s \n or CLI %s\n", errBrowser.Error(), errCLI.Error()))
	return nil

}

func sendCurateMessage(logger *govec.GoLog, torDialer proxy.Dialer) int {

	conn, err := getServerConnection(torDialer)
	checkError(err)

	fmt.Println("TOR Dial Success!")

	var msg MessageData
	msg.Type = "curator"
	msg.SourceNode = name
	msg.ModelId = modelName
	msg.Key = ""
	msg.NumFeatures = 25
	msg.MinClients = minClients

	fmt.Println(msg)
	outBuf := logger.PrepareSend("Sending packet to torserver", msg)

	_, errWrite := conn.Write(outBuf)
	checkError(errWrite)

	inBuf := make([]byte, 2048)
	n, errRead := conn.Read(inBuf)
	checkError(errRead)

	var incomingMsg int
	logger.UnpackReceive("Received Message from server", inBuf[0:n], &incomingMsg)

	conn.Close()

	return incomingMsg

}

func getServerConnection(torDialer proxy.Dialer) (net.Conn, error) {

	var conn net.Conn
	var err error

	if torDialer != nil {
		conn, err = torDialer.Dial("tcp", ONION_HOST)
	} else {
		conn, err = net.Dial("tcp", LOCAL_HOST)
	}

	return conn, err

}

// Error checking function
func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}
}
