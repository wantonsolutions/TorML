package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/DistributedClocks/GoVector/govec"
	"github.com/sbinet/go-python"
	"golang.org/x/net/proxy"
)

var CONTROL_PORT int = 5005

var TOR_PROXY_CLI string = "127.0.0.1:9050"
var TOR_PROXY_BROWSER string = "127.0.0.1:9150"

type MessageData struct {
	Type        string
	SourceNode  string
	ModelId     string
	Key         string
	NumFeatures int
	MinClients  int
}

// Schema for data used in gradient updates
type GradientData struct {
	ModelId string
	Key     string
	Deltas  []float64
}

var (
	name           string
	modelName      string
	datasetName    string
	numFeatures    int
	minClients     int
	isLocal        bool
	puzzleKey      string
	pulledGradient []float64
	hostServer     string
	torAddress     string
	epsilon        float64
	adversary      bool
	latency        int

	pyLogModule   *python.PyObject
	pyLogInitFunc *python.PyObject
	pyLogPrivFunc *python.PyObject
	pyNumFeatures *python.PyObject
)

func init() {
	err := python.Initialize()
	if err != nil {
		panic(err.Error())
	}
}

func pyInit(datasetName string) {

	sysPath := python.PySys_GetObject("path")
	python.PyList_Insert(sysPath, 0, python.PyString_FromString("./"))
	python.PyList_Insert(sysPath, 0, python.PyString_FromString("../ML/code"))

	pyLogModule = python.PyImport_ImportModule("logistic_model")
	pyLogInitFunc = pyLogModule.GetAttrString("init")
	pyLogPrivFunc = pyLogModule.GetAttrString("privateFun")
	pyNumFeatures = pyLogInitFunc.CallFunction(python.PyString_FromString(datasetName), python.PyFloat_FromDouble(epsilon))

	numFeatures = python.PyInt_AsLong(pyNumFeatures)
	minClients = 5
	pulledGradient = make([]float64, numFeatures)

	fmt.Printf("Sucessfully pulled dataset. Features: %d\n", numFeatures)
}

func main() {

	parseArgs()
	logger := govec.InitGoVector(name, name)
	torDialer := getTorDialer()

	// Initialize the python side
	pyInit(datasetName)

	fmt.Printf("Joining model %s \n", modelName)
	joined := 0
	for joined == 0 {
		joined = sendJoinMessage(logger, torDialer)
		if joined == 0 {
			fmt.Println("Could not join.")
			time.Sleep(1 * time.Second)
		}
	}

	TorPing(logger, torDialer)

	sendGradMessage(logger, torDialer, pulledGradient, true)

	for i := 0; i < 200000; i++ {
		sendGradMessage(logger, torDialer, pulledGradient, false)
	}

	heartbeat(logger, torDialer)
	fmt.Println("The end")

}

func heartbeat(logger *govec.GoLog, torDialer proxy.Dialer) {

	for {

		time.Sleep(30 * time.Second)

		conn, err := getServerConnection(torDialer, false)
		if err != nil {
			fmt.Println("Got a Dial failure, retrying...")
			continue
		}

		var msg MessageData
		msg.Type = "beat"
		msg.SourceNode = name
		msg.ModelId = modelName
		msg.Key = puzzleKey
		msg.NumFeatures = numFeatures
		msg.MinClients = minClients

		outBuf := logger.PrepareSend("Sending packet to torserver", msg)

		_, err = conn.Write(outBuf)
		if err != nil {
			fmt.Println("Got a Conn Write failure, retrying...")
			conn.Close()
			continue
		}

		inBuf := make([]byte, 131072)
		n, errRead := conn.Read(inBuf)
		if errRead != nil {
			fmt.Println("Got a Conn Read failure, retrying...")
			conn.Close()
			continue
		}

		var reply int
		logger.UnpackReceive("Received Message from server", inBuf[0:n], &reply)

		fmt.Println("Send heartbeat success")
		conn.Close()

	}

}

func parseArgs() {
	flag.Parse()
	inputargs := flag.Args()
	if len(inputargs) != 9 {
		fmt.Println("USAGE: go run torclient.go nodeName studyName datasetName epsilon usetor serverip onionservice adversary latency")
		fmt.Printf("Args = %s", inputargs)
		fmt.Printf("Len of args = %d", len(inputargs))
		os.Exit(1)
	}
	name = inputargs[0]
	modelName = inputargs[1]
	datasetName = inputargs[2]

	var err error
	epsilon, err = strconv.ParseFloat(inputargs[3], 64)

	if err != nil {
		fmt.Println("Must pass a float for epsilon.")
		os.Exit(1)
	}

	fmt.Printf("Name: %s\n", name)
	fmt.Printf("Study: %s\n", modelName)
	fmt.Printf("Dataset: %s\n", datasetName)

	if inputargs[4] == "false" {
		fmt.Println("Running locally.")
		isLocal = true
		hostServer = inputargs[5] + ":"
	} else if inputargs[4] == "true" {
		fmt.Println("Running over tor")
		isLocal = false
		hostServer = inputargs[6] + ":"
	}

	if inputargs[7] == "false" {
		adversary = false
	} else if inputargs[7] == "true" {
		adversary = true
	} else {
		fmt.Printf("Invalid adversary option %s range should be [true/false]", inputargs[7])
		os.Exit(1)
	}

	l, err := strconv.ParseInt(inputargs[8], 10, 64)
	if err != nil {
		fmt.Println("Unable to parse latency of %s, the argument must be an integer of ms", inputargs[8])
		os.Exit(1)
	}
	latency = int(l)

	fmt.Println("Done parsing args.")
}

//Todo this is duplicated on curator and client
func getTorDialer() proxy.Dialer {

	if isLocal {
		return nil
	}
	// Create proxy dialer using Tor SOCKS proxy via browser socket
	torDialerCLI, errCLI := proxy.SOCKS5("tcp", TOR_PROXY_CLI, nil, proxy.Direct)
	if errCLI != nil {
		fmt.Printf("Unable to connect to TOR via CLI gateway %s\n", errCLI.Error())
	} else {
		return torDialerCLI
	}

	// Create proxy dialer using Tor SOCKS proxy via browser socket
	torDialerBrowser, errBrowser := proxy.SOCKS5("tcp", TOR_PROXY_BROWSER, nil, proxy.Direct)
	if errBrowser != nil {
		fmt.Printf("Unable to connect to TOR via Browser gateway %s\n", errBrowser.Error())
	} else {
		return torDialerBrowser
	}

	checkError(fmt.Errorf("Unable to connect through BROWSER %s \n or CLI %s\n", errBrowser.Error(), errCLI.Error()))
	return nil

}

func sendGradMessage(logger *govec.GoLog,
	torDialer proxy.Dialer,
	globalW []float64,
	bootstrapping bool) int {

	completed := false

	// prevents the screen from overflowing and freezing
	time.Sleep(100 * time.Millisecond)

	for !completed {

		conn, err := getServerConnection(torDialer, true)
		if err != nil {
			fmt.Println("Got a Dial failure, retrying...")
			time.Sleep(100 * time.Millisecond)
			continue
		}

		var msg GradientData
		if !bootstrapping {
			msg.Key = puzzleKey
			msg.ModelId = modelName
			msg.Deltas, err = oneGradientStep(globalW)

			if err != nil {
				fmt.Println("Got a GoPython failure, retrying...")
				conn.Close()
				continue
			}

		} else {
			msg.Key = puzzleKey
			msg.ModelId = modelName
			msg.Deltas = make([]float64, numFeatures)
			bootstrapping = false
		}

		outBuf := logger.PrepareSend("Sending packet to torserver", msg)

		_, err = conn.Write(outBuf)
		if err != nil {
			fmt.Println("Got a conn write failure, retrying...")
			conn.Close()
			continue
		}

		inBuf := make([]byte, 131072)
		n, errRead := conn.Read(inBuf)
		if errRead != nil {
			fmt.Println("Got a reply read failure, retrying...")
			conn.Close()
			continue
		}

		var incomingMsg []float64
		logger.UnpackReceive("Received Message from server", inBuf[0:n], &incomingMsg)

		conn.Close()

		pulledGradient = incomingMsg
		if len(incomingMsg) > 0 {
			completed = true
		} else {
			time.Sleep(1 * time.Second)
		}

	}

	return 1
}

func getServerConnection(torDialer proxy.Dialer, isGradient bool) (net.Conn, error) {

	var conn net.Conn
	var err error

	if isGradient && torDialer != nil {
		conn, err = torDialer.Dial("tcp", torAddress)
	} else if isGradient {
		conn, err = net.Dial("tcp", torAddress)
	} else if torDialer != nil {
		conn, err = torDialer.Dial("tcp", constructAddress(hostServer, CONTROL_PORT))
	} else {
		conn, err = net.Dial("tcp", constructAddress(hostServer, CONTROL_PORT))
	}

	return conn, err

}

func sendJoinMessage(logger *govec.GoLog, torDialer proxy.Dialer) int {

	var failedconnect = 0
	conn, err := getServerConnection(torDialer, false)
	if err != nil {
		fmt.Println("Failed to connect to server: %s", err.Error())
		return failedconnect
	}

	fmt.Println("TOR Dial Success!")

	var msg MessageData
	msg.Type = "join"
	msg.SourceNode = name
	msg.ModelId = modelName
	msg.Key = ""
	msg.NumFeatures = numFeatures
	msg.MinClients = minClients

	outBuf := logger.PrepareSend("Sending packet to torserver", msg)

	_, errWrite := conn.Write(outBuf)
	if errWrite != nil {
		fmt.Println("Failed to write to server: %s", errWrite.Error())
		return failedconnect
	}

	inBuf := make([]byte, 131072)
	n, errRead := conn.Read(inBuf)
	if errRead != nil {
		fmt.Println("Failed to read from server: %s", errWrite.Error())
		return failedconnect
	}

	var puzzle string
	var solution string
	var solved bool
	logger.UnpackReceive("Received puzzle from server", inBuf[0:n], &puzzle)
	conn.Close()

	for !solved {

		h := sha256.New()
		h.Write([]byte(puzzle))

		// Attempt a candidate
		timeHash := sha256.New()
		timeHash.Write([]byte(time.Now().String()))

		solution = hex.EncodeToString(timeHash.Sum(nil))
		h.Write([]byte(solution))

		hashed := hex.EncodeToString(h.Sum(nil))
		//fmt.Println(hashed)

		if strings.HasSuffix(hashed, "0000") {
			fmt.Println("BINGO!")
			solved = true
		}

	}

	conn, err = getServerConnection(torDialer, false)
	if err != nil {
		fmt.Println("Failed to connect to server: %s", err.Error())
		return failedconnect
	}

	msg.Type = "solve"
	msg.Key = solution

	fmt.Printf("Sending solution: %s\n", solution)

	outBuf = logger.PrepareSend("Sending puzzle solution", msg)
	_, errWrite = conn.Write(outBuf)
	if errWrite != nil {
		fmt.Println("Failed to write to server: %s", errWrite.Error())
		return failedconnect
	}

	inBuf = make([]byte, 131072)
	n, errRead = conn.Read(inBuf)
	if errRead != nil {
		fmt.Println("Failed to read from server: %s", errWrite.Error())
		return failedconnect
	}

	var reply int
	logger.UnpackReceive("Received Message from server", inBuf[0:n], &reply)

	// The server replies with the port
	if reply != 0 {
		fmt.Println("Got ACK for puzzle")
		puzzleKey = solution

		if isLocal {
			torAddress = constructAddress(hostServer, reply)
		} else {
			torAddress = constructAddress(hostServer, reply)
		}

		fmt.Printf("Set up connection address %s\n", torAddress)

	} else {
		fmt.Println("My puzzle solution failed.")
	}

	conn.Close()

	return reply

}

func oneGradientStep(globalW []float64) ([]float64, error) {

	argArray := python.PyList_New(len(globalW))

	for i := 0; i < len(globalW); i++ {
		python.PyList_SetItem(argArray, i, python.PyFloat_FromDouble(globalW[i]))
	}

	// Either use full GD or SGD here
	result := pyLogPrivFunc.CallFunction(python.PyInt_FromLong(1), argArray,
		python.PyInt_FromLong(10))

	// Convert the resulting array to a go byte array
	pyByteArray := python.PyByteArray_FromObject(result)
	goByteArray := python.PyByteArray_AsBytes(pyByteArray)

	var goFloatArray []float64
	size := len(goByteArray) / 8

	for i := 0; i < size; i++ {
		currIndex := i * 8
		bits := binary.LittleEndian.Uint64(goByteArray[currIndex : currIndex+8])
		aFloat := math.Float64frombits(bits)
		goFloatArray = append(goFloatArray, aFloat)
	}

	return goFloatArray, nil
}

func constructAddress(host string, port int) string {

	var buffer bytes.Buffer
	buffer.WriteString(host)
	buffer.WriteString(strconv.Itoa(port))
	return buffer.String()
}

// Error checking function
func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}
}

//Ping the tor server, write the ping out to a file called torping.out
func TorPing(logger *govec.GoLog, torDialer proxy.Dialer) {
	conn, err := getServerConnection(torDialer, false)
	defer conn.Close()
	if err != nil {
		fmt.Printf("Unable to get ping connection srew it. Err:%s", err.Error())
		return
	}
	fmt.Println("TOR Dial Success!")

	fmt.Println("Collecting Ping")
	var msg MessageData
	msg.Type = "ping"
	msg.SourceNode = ""
	msg.ModelId = ""
	msg.Key = ""
	msg.NumFeatures = 0
	msg.MinClients = 0

	inBuf := make([]byte, 131072)
	outBuf := logger.PrepareSend("Sending ping", msg)
	conn.Write(outBuf)
	start := time.Now()
	n, errRead := conn.Read(inBuf)
	if errRead != nil {
		fmt.Println("unable to read ping, screw it. Err:%s", err.Error())
		return
	}
	end := time.Now()
	logger.UnpackReceive("Receving ping?", inBuf[0:n], &msg)
	hname, _ := os.Hostname()
	f, err := os.OpenFile(fmt.Sprintf("%s.torping", hname), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	defer f.Sync()
	if err != nil {
		panic(err)
	}

	if msg.Type == "pong" {
		ping := end.Sub(start)
		ms := fmt.Sprintf("%2.1f", float32(float32(ping.Nanoseconds())/1000000)) //I think this is right
		fmt.Printf("Ping: %s\n", ms)
		f.WriteString(fmt.Sprintf("%s\n", ms))
	} else {
		errmsg := fmt.Sprintf("Did not receive pong after ping got %s instead", msg.Type)
		f.WriteString(errmsg)
		panic(fmt.Errorf(errmsg))
	}
	return
}
