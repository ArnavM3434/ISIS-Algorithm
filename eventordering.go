package main

import (
	"bufio"
	"container/heap"
	"encoding/gob"
	"fmt"
	"math"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Item struct {
	value    string
	priority float64
	index    int
}

type PriorityQueue []*Item

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].priority < pq[j].priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x any) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[0 : n-1]
	return item
}

func (pq *PriorityQueue) update(item *Item, value string, priority float64) {
	item.value = value
	item.priority = priority
	heap.Fix(pq, item.index)
}

var messageBuffer chan Mess

var outgoingMessageBuffer chan Mess

var outgoingConnections map[net.Conn]string

var incomingConnectionsMutex sync.Mutex
var outgoingConnectionsMutex sync.Mutex
var numNodes int = 3
var numNodesMutex sync.Mutex
var numIncomingConnections int = 0
var numOutgoingConnections int = 0

var timeout int
var timeoutMutex sync.Mutex

var currentOriginalMessageNum int = 0
var currentOriginalMessageMutex sync.Mutex

var currentPriority int = 0
var currentPriorityMutex sync.Mutex

var wg sync.WaitGroup

var originalNodes int

var configuration map[string]string

//  = map[string]string{
// 	"node1": "sp24-cs425-0601.cs.illinois.edu:1234",
// 	"node2": "sp24-cs425-0602.cs.illinois.edu:1234",
// 	"node3": "sp24-cs425-0603.cs.illinois.edu:1234",
// 	// "node4": "sp24-cs425-0604.cs.illinois.edu:1234",
// 	// "node5": "sp24-cs425-0605.cs.illinois.edu:1234",
// 	// "node6": "sp24-cs425-0606.cs.illinois.edu:1234",
// 	// "node7": "sp24-cs425-0607.cs.illinois.edu:1234",
// 	// "node8": "sp24-cs425-0608.cs.illinois.edu:1234",
// }

var inverse_configuration map[string]string

// = map[string]string{
// 	"sp24-cs425-0601.cs.illinois.edu:1234": "node1",
// 	"sp24-cs425-0602.cs.illinois.edu:1234": "node2",
// 	"sp24-cs425-0603.cs.illinois.edu:1234": "node3",
// 	// "sp24-cs425-0604.cs.illinois.edu:1234": "node4",
// 	// "sp24-cs425-0605.cs.illinois.edu:1234": "node5",
// 	// "sp24-cs425-0606.cs.illinois.edu:1234": "node6",
// 	// "sp24-cs425-0607.cs.illinois.edu:1234": "node7",
// 	// "sp24-cs425-0608.cs.illinois.edu:1234": "node8",
// }

var process_numbers map[string]int

// = map[string]int{
// 	"node1": 1,
// 	"node2": 2,
// 	"node3": 3,
// 	// "node4": 4,
// 	// "node5": 5,
// 	// "node6": 6,
// 	// "node7": 7,
// 	// "node8": 8,
// }

var port string

var filepath string

var all_seen_messages map[string]int
var all_seen_messagesMutex sync.Mutex

var sent_original_messages map[string]int //all messages sent from this node originally
var sent_original_messagesMutex sync.Mutex

var sent_original_priorities map[string]float64 //keep track of priorities for messages that have originated from this node
var sent_original_prioritiesMutex sync.Mutex

var all_original_messages map[string]string
var all_original_messagesMutex sync.Mutex

var pq_message_id_map map[string]*Item
var pq_message_id_mapMutex sync.Mutex

var accounts map[string]int
var accounts_mutex sync.Mutex

var hasFinalPriority map[string]int
var hasFinalPriorityMutex sync.Mutex

var redelivery map[string]Mess
var redeliveryMutex sync.Mutex

var initialEntryTimes map[string]float64
var initialEntryTimesMutex sync.Mutex

var priority_queueMutex sync.Mutex

var deadNodes map[string]int
var deadNodesMutex sync.Mutex

var invalidMessages map[string]int
var invalidMessagesMutex sync.Mutex

var pq PriorityQueue

var thisNode string

var f *os.File

type Mess struct {
	Contents                    string
	OriginalSender              string
	OriginalSenderProcessNumber int
	MesssageType                string //original, proposed, final
	ProposedPriority            float64
	FinalPriority               float64
	MessageId                   string
	OriginalMessageId           string
	MostRecentSender            string
}

func main() {

	timeout = 20

	//initialize priority queue
	pq = make(PriorityQueue, 0)
	heap.Init(&pq)

	messageBuffer = make(chan Mess, 700)
	outgoingMessageBuffer = make(chan Mess, 700)

	argv := os.Args[1:]
	thisNode = argv[0]
	filepath = argv[1]
	parse()

	fileName := thisNode + ".txt"

	f, _ = os.Create(fileName)

	originalNodes = numNodes

	go processMessages()
	go processOutgoingMessages()

	outgoingConnections = make(map[net.Conn]string)
	all_seen_messages = make(map[string]int)
	sent_original_messages = make(map[string]int)
	sent_original_priorities = make(map[string]float64)
	all_original_messages = make(map[string]string)
	pq_message_id_map = make(map[string]*Item)
	accounts = make(map[string]int)
	redelivery = make(map[string]Mess)
	initialEntryTimes = make(map[string]float64)
	deadNodes = make(map[string]int)
	invalidMessages = make(map[string]int)

	go startServer()

	for k, v := range configuration {
		if k != thisNode {
			wg.Add(1)
			go connectToNode(v)
		}

	}

	wg.Wait()

	time.Sleep(time.Second * 2)

	reader := bufio.NewReader(os.Stdin)
	for {

		message, _ := reader.ReadString('\n')
		message = strings.TrimSpace(message)

		//fmt.Println(message)

		currentOriginalMessageMutex.Lock()
		currentOriginalMessageNum++
		struct_message := Mess{Contents: message, OriginalSender: thisNode, OriginalSenderProcessNumber: process_numbers[thisNode], MesssageType: "original", ProposedPriority: 0, FinalPriority: 0, MessageId: strconv.Itoa(process_numbers[thisNode]) + ":" + strconv.Itoa(currentOriginalMessageNum), OriginalMessageId: strconv.Itoa(process_numbers[thisNode]) + ":" + strconv.Itoa(currentOriginalMessageNum), MostRecentSender: thisNode}
		currentOriginalMessageMutex.Unlock()

		sent_original_messagesMutex.Lock()
		sent_original_messages[struct_message.OriginalMessageId] = 0
		sent_original_messagesMutex.Unlock()

		all_original_messagesMutex.Lock()
		all_original_messages[struct_message.OriginalMessageId] = struct_message.Contents
		all_original_messagesMutex.Unlock()

		//ASUTHOSH - over here write to textfile: struct_message.OriginalMessageId, "original", current time

		// sent_original_prioritiesMutex.Lock()
		// priorityNum, _ := strconv.ParseFloat(strconv.Itoa(currentPriority)+"."+strconv.Itoa(process_numbers[thisNode]), 64)
		// sent_original_priorities[struct_message.OriginalMessageId] = priorityNum
		// sent_original_prioritiesMutex.Unlock()

		curr_time := float64(time.Now().UnixNano()) / 1e9

		d2 := struct_message.OriginalMessageId + " " + "original" + " " + fmt.Sprintf("%.7f", curr_time) + " " + "\n"

		f.WriteString(d2)

		go ISISLayer(struct_message)
		//multicast message

		go RMulticast(struct_message)

	}
}

func parse() {
	file, ferr := os.Open(filepath)

	if ferr != nil {
		panic(ferr)
	}

	scanner := bufio.NewScanner(file)
	configuration = make(map[string]string)
	inverse_configuration = make(map[string]string)
	process_numbers = make(map[string]int)
	var lineNum int = 0
	for scanner.Scan() {
		line := scanner.Text() // retrives the current line as a string
		if lineNum == 0 {
			numNodes, _ = strconv.Atoi(line)
		} else {
			split := strings.Split(line, " ")
			configuration[split[0]] = split[1] + ":" + split[2]
			inverse_configuration[split[1]+":"+split[2]] = split[0]
			process_numbers[split[0]] = lineNum

			if split[0] == thisNode {
				port = ":" + split[2]
			}

		}

		lineNum++

	}

}

// Function to start the server that listens for incoming connections.
func startServer() {
	//listeningPort := configuration[thisNode]
	ln, err := net.Listen("tcp", port) // Listen on the specified port.
	if err != nil {
		//fmt.Println("Error starting server:", err)
		return
	}
	defer ln.Close() // Ensure the listener is closed when the function exits.
	//fmt.Println("Server started on port", ":1234")

	for {
		conn, err := ln.Accept() // Accept new connections.
		if err != nil {
			//fmt.Println("Error accepting connection:", err)
			//fmt.Println("lost the node")
			continue
		}
		go handleConnection(conn) // Handle the connection in a separate goroutine.
	}
}

// Function to handle incoming connections.
func handleConnection(conn net.Conn) {
	incomingConnectionsMutex.Lock() // Lock the mutex to safely add the connection to the map.
	//incomingConnections[remoteAddr] = conn
	numIncomingConnections += 1
	incomingConnectionsMutex.Unlock() // Unlock the mutex.

	for {
		decoder := gob.NewDecoder(conn)
		var incoming_message Mess
		err := decoder.Decode(&incoming_message)
		if err != nil {
			//fmt.Printf("%s %s\n", incoming_message.OriginalMessageId, incoming_message.MesssageType)
			//fmt.Print(err)
		}
		if err == nil {
			messageBuffer <- incoming_message
		}

		// all_seen_messagesMutex.Lock()
		// _, ok := all_seen_messages[incoming_message.MessageId]
		// if !ok {
		// 	all_seen_messages[incoming_message.MessageId] = 1
		// }
		// all_seen_messagesMutex.Unlock()

		// //fmt.Printf("%s %s %s", incoming_message.MesssageType, incoming_message.OriginalMessageId, incoming_message.OriginalSender)

		// if !ok {
		// 	// if incoming_message.MesssageType == "proposed" {
		// 	// 	fmt.Printf("proposed for message %s\n", incoming_message.OriginalMessageId)
		// 	// }
		// 	go ISISLayer(incoming_message)
		// 	go RMulticast(incoming_message)

		// }

	}

}

func processMessages() {
	for {
		select {
		case message := <-messageBuffer:
			// Process the message
			all_seen_messagesMutex.Lock()
			_, ok := all_seen_messages[message.MessageId]
			if !ok {
				all_seen_messages[message.MessageId] = 1
			}
			all_seen_messagesMutex.Unlock()

			//fmt.Printf("%s %s %s", incoming_message.MesssageType, incoming_message.OriginalMessageId, incoming_message.OriginalSender)
			redeliveryMutex.Lock()
			redelivery[message.OriginalMessageId] = message
			redeliveryMutex.Unlock()
			if !ok {
				// if incoming_message.MesssageType == "proposed" {
				// 	fmt.Printf("proposed for message %s\n", incoming_message.OriginalMessageId)
				// }
				go ISISLayer(message)
				go RMulticast(message)

			}

		}
	}
}

// Function to establish a connection to another node.
func connectToNode(address string) {
	defer wg.Done()
	conn, err := net.Dial("tcp", address) // Dial the specified address.
	if err != nil {
		//fmt.Println("Error connecting to node:", err)
		wg.Add(1)
		go connectToNode(address)
		return
	}

	// Add the connection to the map for future communication.
	outgoingConnectionsMutex.Lock()
	numOutgoingConnections++
	outgoingConnections[conn] = inverse_configuration[address]
	outgoingConnectionsMutex.Unlock()

	//fmt.Println("Connected to node", address)
}

// Function to send a message to a connected node.
func RMulticast(mes Mess) {

	outgoingMessageBuffer <- mes

}

func processOutgoingMessages() {
	for {
		select {
		case message := <-outgoingMessageBuffer:
			// Process the outgoing message
			sendMessageToConnectedNodes(message)
		}
	}
}

func sendMessageToConnectedNodes(message Mess) {
	for conn := range outgoingConnections {
		encoder := gob.NewEncoder(conn)
		err := encoder.Encode(&message)
		if err != nil {
			//fmt.Println("Error encoding message:", err)
			numNodesMutex.Lock()
			numNodes--
			//fmt.Printf("numNodes: %d\n", numNodes)
			numNodesMutex.Unlock()

			possiblyIncreaseTimeout(outgoingConnections[conn])

			deadNodesMutex.Lock()
			deadNodes[outgoingConnections[conn]] = 1
			deadNodesMutex.Unlock()

			// outgoingConnectionsMutex.Lock()
			// delete(outgoingConnections, conn)
			// outgoingConnectionsMutex.Unlock()

			// sendMessageToConnectedNodes(message)

		}
	}

}

func ISISLayer(mes Mess) {

	typeOfMessage := mes.MesssageType
	//fmt.Println("In ISIS layer")
	if typeOfMessage == "proposed" {
		if mes.OriginalSender == thisNode {

			numNodesMutex.Lock()
			newNumNodes := numNodes
			// if numNodes < 8 {
			// 	fmt.Printf("still goes to proposed layer\n")
			// }
			numNodesMutex.Unlock()

			var finalPriority float64

			//fmt.Printf("I process %s am sending final priority for message %s\n", thisNode, mes.OriginalMessageId)

			sent_original_prioritiesMutex.Lock()
			sent_original_priorities[mes.OriginalMessageId] = math.Max(mes.ProposedPriority, sent_original_priorities[mes.OriginalMessageId])
			finalPriority = sent_original_priorities[mes.OriginalMessageId]
			sent_original_prioritiesMutex.Unlock()

			var haveAllProposals bool = false
			sent_original_messagesMutex.Lock()
			sent_original_messages[mes.OriginalMessageId] = sent_original_messages[mes.OriginalMessageId] + 1
			if newNumNodes < 8 {
				//fmt.Printf("Message ID: %s Tracked Messages: %d", mes.OriginalMessageId, sent_original_messages[mes.OriginalMessageId])
			}
			if sent_original_messages[mes.OriginalMessageId] >= newNumNodes-1 {
				haveAllProposals = true
			}
			sent_original_messagesMutex.Unlock()
			//haveAllProposals = true
			if haveAllProposals == true {
				//fmt.Println("Have all proposals")
				if newNumNodes < 8 {
					//	fmt.Printf("%s Number of proposals: %d", mes.OriginalMessageId, sent_original_messages[mes.OriginalMessageId])
				}

				currentPriorityMutex.Lock()
				// currentPriority = int(finalPriority) + 1
				currentPriority = (int)(math.Max(float64(currentPriority), finalPriority))
				currentPriorityMutex.Unlock()

				currentOriginalMessageMutex.Lock()
				currentOriginalMessageNum++
				currentOriginalMessageMutex.Unlock()

				finalMessage := Mess{Contents: mes.Contents, OriginalSender: thisNode, OriginalSenderProcessNumber: mes.OriginalSenderProcessNumber, MesssageType: "final", ProposedPriority: 0, FinalPriority: finalPriority, MessageId: strconv.Itoa(process_numbers[thisNode]) + ":" + strconv.Itoa(currentOriginalMessageNum), OriginalMessageId: mes.OriginalMessageId, MostRecentSender: thisNode}

				go ISISLayer(finalMessage)

				for i := 0; i < 10; i++ {
					go RMulticast(finalMessage)
				}

			}

		}

	}

	if typeOfMessage == "final" {
		//if numNodes < 3 {
		//fmt.Printf("changing status of message %s with %d proposals\n", mes.OriginalMessageId, sent_original_messages[mes.OriginalMessageId])
		//}
		//update priority queue
		pq_message_id_mapMutex.Lock()
		tempElement, ok := pq_message_id_map[mes.OriginalMessageId]
		pq_message_id_mapMutex.Unlock()

		if ok {
			// numNodesMutex.Lock()
			// currentNodes := numNodes
			// numNodesMutex.Unlock()
			// if currentNodes < 8 {
			// 	fmt.Printf("Deliverable: %s Priority: %f\n", mes.OriginalMessageId, mes.FinalPriority)
			// }
			//fmt.Println("why is the priority queue not being modified?")
			priority_queueMutex.Lock()
			pq.update(tempElement, mes.OriginalMessageId+"_Deliverable", mes.FinalPriority)
			priority_queueMutex.Unlock()

			currentPriorityMutex.Lock()
			currentPriority = (int)(math.Max(mes.FinalPriority, (float64)(currentPriority)))
			currentPriorityMutex.Unlock()

			go ApplicationLayer(pq)
		}
		// priority_queueMutex.Lock()
		// for _, item := range pq {
		// 	// Check if the item's value matches the desired value
		// 	if strings.Split(item.value, "_")[0] == mes.MessageId {
		// 		item.priority = mes.FinalPriority
		// 		item.value = mes.OriginalMessageId + "_Deliverable"
		// 		heap.Fix(&pq, item.index)

		// 	}
		// }
		// priority_queueMutex.Unlock()
		// currentPriorityMutex.Lock()
		// currentPriority = (int)(math.Max(mes.FinalPriority, (float64)(currentPriority)))
		// currentPriorityMutex.Unlock()
		// go ApplicationLayer(pq)

	}

	if typeOfMessage == "original" {
		// numNodesMutex.Lock()
		// if numNodes < 8 {
		// 	fmt.Printf("still goes to original layer\n")
		// }
		// numNodesMutex.Unlock()
		var priorityNum float64
		currentPriorityMutex.Lock()
		currentPriority++
		priorityNum, _ = strconv.ParseFloat(strconv.Itoa(currentPriority)+"."+strconv.Itoa(process_numbers[thisNode]), 64)
		currentPriorityMutex.Unlock()

		currentOriginalMessageMutex.Lock()
		currentOriginalMessageNum++
		currentOriginalMessageMutex.Unlock()

		sent_original_prioritiesMutex.Lock()
		if mes.OriginalSender == thisNode {
			//priorityNum = sent_original_priorities[mes.OriginalMessageId]
			sent_original_priorities[mes.OriginalMessageId] = priorityNum

		} else {
			priorityNum, _ = strconv.ParseFloat(strconv.Itoa(currentPriority)+"."+strconv.Itoa(process_numbers[thisNode]), 64)
		}
		sent_original_prioritiesMutex.Unlock()

		all_original_messagesMutex.Lock()
		all_original_messages[mes.OriginalMessageId] = mes.Contents
		all_original_messagesMutex.Unlock()

		var alreadyExists bool = false
		priority_queueMutex.Lock()
		for _, item := range pq {
			if strings.Split(item.value, "_")[0] == mes.OriginalMessageId {
				alreadyExists = true
			}

		}

		//fmt.Printf("Priority Number:  %f\n", priorityNum)
		item2 := &Item{value: mes.OriginalMessageId + "_Undeliverable", priority: priorityNum}
		if alreadyExists == false {
			heap.Push(&pq, item2)
			pq_message_id_map[mes.OriginalMessageId] = item2
		}
		priority_queueMutex.Unlock()

		if alreadyExists == false {
			initialEntryTimesMutex.Lock()
			initialEntryTimes[mes.OriginalMessageId] = float64(time.Now().UnixNano()) / 1e9
			initialEntryTimesMutex.Unlock()
		}

		var proposalMessage Mess
		//if mes.OriginalSender != thisNode {
		proposalMessage = Mess{Contents: mes.Contents, OriginalSender: mes.OriginalSender, OriginalSenderProcessNumber: mes.OriginalSenderProcessNumber, MesssageType: "proposed", ProposedPriority: priorityNum, FinalPriority: 0, MessageId: strconv.Itoa(process_numbers[thisNode]) + ":" + strconv.Itoa(currentOriginalMessageNum), OriginalMessageId: mes.OriginalMessageId, MostRecentSender: thisNode}

		//fmt.Printf("Sending proposal for %s to %s\n", mes.OriginalMessageId, mes.OriginalSender)
		if mes.OriginalSender != thisNode {
			go RMulticast(proposalMessage)
		}
		//go ISISLayer(proposalMessage)

	}

}

func ApplicationLayer(queue_structure PriorityQueue) {
	//fmt.Println("In application layer")
	priority_queueMutex.Lock()
	//fmt.Println("New queue")
	for _, item := range pq {
		tempValue := item.value
		tempMessageId := strings.Split(tempValue, "_")[0]
		tempDeliveryStatus := strings.Split(tempValue, "_")[1]
		initialEntryTimesMutex.Lock()
		temptime := initialEntryTimes[tempMessageId]
		initialEntryTimesMutex.Unlock()
		redeliveryMutex.Lock()
		//tempOriginalSender := redelivery[tempMessageId].OriginalSender
		redeliveryMutex.Unlock()
		deadNodesMutex.Lock()
		//_, invalidNode := deadNodes[tempOriginalSender]
		deadNodesMutex.Unlock()

		timeoutMutex.Lock()
		currTimeout := timeout
		timeoutMutex.Unlock()

		if tempDeliveryStatus == "Undeliverable" && (float64)((time.Now().UnixNano())/1e9)-temptime >= float64(currTimeout) { //} {
			invalidMessagesMutex.Lock()
			invalidMessages[tempMessageId] = 1
			invalidMessagesMutex.Unlock()

		}
	}

	for pq.Len() > 0 {
		item := heap.Pop(&pq).(*Item)
		tempValue := item.value
		tempDeliveryStatus := strings.Split(tempValue, "_")[1]
		tempMessageId := strings.Split(tempValue, "_")[0]
		if tempDeliveryStatus == "Deliverable" {
			//fmt.Println("Delivered Successfully : " + all_original_messages[tempMessageId])

			//ASUTHOSH - write to file tempMessageId, "receiving", current time
			curr_time := float64(time.Now().UnixNano()) / 1e9

			d2 := tempMessageId + " " + "receiving" + " " + fmt.Sprintf("%.7f", curr_time) + " " + "\n"

			f.WriteString(d2)

			go ProcessTransaction(all_original_messages[tempMessageId])
		} else {
			invalidMessagesMutex.Lock()
			_, inValid := invalidMessages[tempMessageId]
			invalidMessagesMutex.Unlock()

			if !inValid {
				heap.Push(&pq, item)
				break
			}
			// } else {

			// 	deadNodesMutex.Lock()
			// 	_, isDeadNode := deadNodes[redelivery[tempMessageId].OriginalSender]
			// 	deadNodesMutex.Unlock()
			// 	if !isDeadNode {
			// 		go RMulticast(redelivery[tempMessageId])
			// 		go ISISLayer(redelivery[tempMessageId])

			// 	}

			// }

			// 	break
		}

	}
	priority_queueMutex.Unlock()

}

func ProcessTransaction(message string) {
	parameters := strings.Split(message, " ")
	accounts_mutex.Lock()
	if parameters[0] == "DEPOSIT" {
		accountName := parameters[1]
		amount := parameters[2]
		_, accountexists := accounts[accountName]
		temp, _ := strconv.Atoi(amount)
		if !accountexists {
			accounts[accountName] = temp
		} else {
			accounts[accountName] = accounts[accountName] + temp

		}

	} else if parameters[0] == "TRANSFER" {
		sender := parameters[1]
		recipient := parameters[3]
		transferAmount, _ := strconv.Atoi(parameters[4])
		_, exists := accounts[sender]
		if exists && accounts[sender] >= transferAmount {
			accounts[sender] = accounts[sender] - transferAmount

			_, accountExists := accounts[recipient]
			if !accountExists {
				accounts[recipient] = 0

			}
			accounts[recipient] = accounts[recipient] + transferAmount

		}

	}

	keys := make([]string, 0, len(accounts))
	for k, _ := range accounts {
		if accounts[k] > 0 {
			keys = append(keys, k)
		}
		//fmt.Printf("%s %d", k, accounts[k])
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		fmt.Printf("BALANCES ")
	}
	for _, k := range keys {
		fmt.Printf("%s:%d ", k, accounts[k])
	}
	if len(keys) > 0 {
		fmt.Println()
	}
	accounts_mutex.Unlock()

}

func possiblyIncreaseTimeout(currentNode string) {
	deadNodesMutex.Lock()
	_, ok := deadNodes[currentNode]
	deadNodesMutex.Unlock()

	if !ok && numNodes < originalNodes {

		// priority_queueMutex.Lock()
		// //fmt.Println("New queue")
		// for _, item := range pq {
		// 	tempValue := item.value
		// 	tempMessageId := strings.Split(tempValue, "_")[0]
		// 	initialEntryTimesMutex.Lock()
		// 	initialEntryTimes[tempMessageId] = float64(time.Now().UnixNano()) / 1e9
		// 	initialEntryTimesMutex.Unlock()

		// }

		// priority_queueMutex.Unlock()

		// timeoutMutex.Lock()
		// //timeout = 30
		// timeoutMutex.Unlock()

	}

}
