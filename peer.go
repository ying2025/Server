package main

import (
	"bytes"
	"flag"
	"fmt"
	"golang.org/x/net/websocket"
	"html/template"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
	"unsafe"
	"strings"
)

type PeerConn struct {
	client		    *ClientConfig// as a client
	*Config
	*ServerConfig    // as a server
	WsConn 			*websocket.Conn
}

func web(w http.ResponseWriter, r *http.Request) {
	// print request method
	fmt.Println("method", r.Method, r.URL)
	if r.Method == "GET" {  // Show login.html anf transfer it to  front-end
		t, _ := template.ParseFiles("./index.html")  // support html template
		t.Execute(w, nil)
		fmt.Println("==========:", w)
	} else { // print the param (username and password) of receiving
		fmt.Println(r.PostFormValue("username"))
		fmt.Println(r.PostFormValue("password"))
	}
}

var wg sync.WaitGroup
func serverReceive(server *PeerConn) {
	defer wg.Done()

	var err error
	server.PeerCount++
	fmt.Println("begin to listen")
	// Judge whether is server and do reference deal
	isServer := JudgeIsServer(server)
	var i int = 1
	for {
		var receiveData string
		var reply []byte
		// Active close. Send Bye to others
		//if server.CloseFlag {
		//	if suc := Close(server); suc {  // graceful colse
		//		server.CloseFlag = false
		//		server.RejectReqFlag = false // Alread send
		//		server.PeerCount--
		//		break
		//	}
		//}
		//If RejectReqFlag is false, then websocket receive message
		// else no longer accept new requests.
		if !server.RejectReqFlag {
			err = websocket.Message.Receive(server.WsConn, &receiveData);
		}

		if err == io.EOF {
			panic("=========== Read ERROR: Connection has already broken of")
		} else if err != nil {
			panic(err.Error())
		}
		if  (len(receiveData) > 16) && (receiveData[8] == 0x58) && (receiveData[11] == 0x01) { // encrypt
			nonce := []byte(receiveData[:8])
			for _, val := range server.NonceList {  // nonce is same, do nothing
				if bytes.Equal(val, nonce) {
					panic("Data have been receive")
				}
			}
			server.NonceList[len(server.NonceList)] = nonce
			receiveData = receiveData[8:]
		}
		server.UnDealReplyList[len(server.UnDealReplyList)] = []byte(receiveData[:])
		header   := []byte(receiveData[:8])
		head	 := GetHeader(header)
		//fmt.Println("head: ",head)
		if err = CheckHeader(head); err != nil {
			break
		}
		DeleteUndealData(server, []byte(receiveData[:]))  // Reply is to deal
		switch head.Type {
		case 'H':
			// TODO  service, method, ctx, args
			if bytes.Equal(server.CommonKey, nil) {
				server.EncFlag = false
			}
			ctx := make(map[string]interface{})
			arg := make(map[string]interface{})
			reply = PackQuest(server, server.EncFlag, "service", "method", ctx, arg)
		case 'Q': 				//Q
			//if i > 2 {
			//	server.CloseFlag = true
			//	server.RejectReqFlag = true
			//	i = 0
			//}
			i++
			reply = DealRequest(server, receiveData)
			if reply == nil {
				continue
			}
		case 'C':
			if !isServer {  // client
				reply = UnpackCheck(server, receiveData)
			} else { // server
				reply = DealCheck(server, receiveData)  // TODO UnTest
				if reply[4] == 0x01 { // encrypt
					server.SendChan <- reply
					reply = HelloMessage.sendHello()
				}
			}
		case 'A': 				//A
			reply = DealAnswer(server, receiveData)
			if reply == nil {
				continue
			}
		case 'B':               //B
			server.RejectReqFlag = true
			if flag := GracefulClose(server); flag {
				server.WsConn.Close()
				server.PeerCount--
				return
			} else {
				continue
			}
		default:
			log.Fatalln("ERROR")
		}
		server.SendChan <- reply  // send the reply to send channel
	}
}

func serverReply(peer *PeerConn) {
	for {
		select {
		case message, ok := <-peer.SendChan:
			if !ok {
				panic("The data is already to send !")
			}
			if err := websocket.Message.Send(peer.WsConn, message); err != nil {  // send data
				log.Fatal(err)
			}
			fmt.Println("Server send data ", message)
		}
	}
}

func Listen(ws *websocket.Conn) {
	//go EchoHandle(ws)
	server := NewServerConfig(ws)
	wg.Add(1)
	go serverReceive(server)
	serverReply(server)
	res := WaitTimeout(&wg, 1 * time.Second)
	if res {
		fmt.Println("执行完成退出")
	} else {
		fmt.Println("执行超时")
	}
}
func WaitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	ch := make(chan struct{})
	go func() {
		wg.Wait()
		close(ch)
	}()
	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}

func startServerMode() {
	//http.Handle("/", websocket.Handler(EchoHandle))

	http.Handle("/", websocket.Handler(Listen))
	http.HandleFunc("/web", web)

	log.Fatal(http.ListenAndServe(":8989", nil))
}

func clientReceive(peer *PeerConn) {
	var err error
	var receiveData  string
	var reply []byte
	for {
		err = websocket.Message.Receive(peer.WsConn, &receiveData)
		if err == io.EOF {
			panic("=========== Read ERROR: Connection has already broken of")
		} else if err != nil {
			panic(err.Error())
		}

		peer.client.AlreadyDeal = false
		if  (len(receiveData) > 16) && (receiveData[8] == 0x58) && (receiveData[11] == 0x01) { // encrypt
			nonce := []byte(receiveData[:8])
			for _, val := range peer.NonceList {  // nonce is same, do nothing
				if bytes.Equal(val, nonce) {
					panic("Data have been receive")
				}
			}
			peer.NonceList[len(peer.NonceList)] = nonce
			receiveData = receiveData[8:]
		}
		header   := []byte(receiveData[:8])
		head	 := GetHeader(header)
		//fmt.Println("head: ",head)
		if err = CheckHeader(head); err != nil {
			break
		}
		switch head.Type {
		case 'H':
			if bytes.Equal(peer.CommonKey, nil) {
				peer.EncFlag = false
			}
			ctx := make(map[string]interface{})
			arg := make(map[string]interface{})
			arg["first"] = "this is the first data client send"
			arg["second"] = "this is second data "
			reply = PackQuest(peer, peer.EncFlag, "service", "method", ctx, arg)
		case 'Q': 				//Q
			reply = DealRequest(peer, receiveData)
			if reply == nil {
				continue
			}
		case 'C':
			reply = UnpackCheck(peer, receiveData)
		case 'A': 				//A
			reply = DealAnswer(peer, receiveData)
			if reply == nil {
				continue
			}
		case 'B':               //B
			peer.RejectReqFlag = true
			if flag := GracefulClose(peer); flag {
				peer.WsConn.Close()
				return
			} else {
				continue
			}
		default:
			log.Fatalln("ERROR")
		}
		peer.SendChan <- reply
		peer.client.AlreadyDeal = true
	}
}
func clientReply(peer *PeerConn) {
	for {
		select {
		case message, ok := <-peer.SendChan:
			  if !ok {
			  		panic("The data is already to send !")
			  }
			  if err := websocket.Message.Send(peer.WsConn, message); err != nil {  // send data
				  log.Fatal(err)
			  }
			  fmt.Println("Send data ", message)
		}
	}
}

func startClientMode() {
	//var receiveData string
	//var reply []byte

	origin := "http://127.0.0.1:8989/"
	// "ws://192.168.200.40:8989/"
	ws_url := "ws://10.8.161.112:8989/"
	ws, err := websocket.Dial(ws_url,"",origin)
	if err != nil {
		log.Fatal(err)
	}
	peer := NewClientConfig(ws)
	//////////////////////////////////////////////////
	go clientReceive(peer)
	clientReply(peer)
	/////////////////////////////////////////////////
	// If donnot set msg size, it cannot read the data.
	//for {
	//	err = websocket.Message.Receive(ws, &receiveData)
	//	if err == io.EOF {
	//		panic("=========== Read ERROR: Connection has already broken of")
	//	} else if err != nil {
	//		panic(err.Error())
	//	}
	//
	//	peer.client.AlreadyDeal = false
	//	if  (len(receiveData) > 16) && (receiveData[8] == 0x58) && (receiveData[11] == 0x01) { // encrypt
	//		nonce := []byte(receiveData[:8])
	//		for _, val := range peer.NonceList {  // nonce is same, do nothing
	//			if bytes.Equal(val, nonce) {
	//				panic("Data have been receive")
	//			}
	//		}
	//		peer.NonceList[len(peer.NonceList)] = nonce
	//		receiveData = receiveData[8:]
	//	}
	//	header   := []byte(receiveData[:8])
	//	head	 := GetHeader(header)
	//	//fmt.Println("head: ",head)
	//	if err = CheckHeader(head); err != nil {
	//		break
	//	}
	//	switch head.Type {
	//	case 'H':
	//		if bytes.Equal(peer.CommonKey, nil) {
	//			EncFlag = false
	//		}
	//		ctx := make(map[string]interface{})
	//		arg := make(map[string]interface{})
	//		arg["first"] = "this is the first data client send"
	//		arg["second"] = "this is second data "
	//		reply = PackQuest(peer, EncFlag, "service", "method", ctx, arg)
	//	case 'Q': 				//Q
	//		reply = DealRequest(peer, receiveData)
	//		if reply == nil {
	//			continue
	//		}
	//	case 'C':
	//		reply = UnpackCheck(peer, receiveData)
	//	case 'A': 				//A
	//		reply = DealAnswer(peer, receiveData)
	//		if reply == nil {
	//			continue
	//		}
	//	case 'B':               //B
	//		peer.RejectReqFlag = true
	//		if flag := GracefulClose(peer); flag {
	//			ws.Close()
	//			return
	//		} else {
	//			continue
	//		}
	//	default:
	//		log.Fatalln("ERROR")
	//	}
	//	if err := websocket.Message.Send(ws, reply); err != nil {  // send data
	//		log.Fatal(err)
	//	}
	//	fmt.Println("Send data ", reply)
	//	peer.client.AlreadyDeal = true
	//}
	
}

func main() {
	flagMode := flag.String("mode", "server", "start in client or server")
	flag.Parse()
	if strings.ToLower(*flagMode) == "server" {
		startServerMode()
	} else {
		startClientMode()
	}
}

//func main() {
//	fmt.Println("os.Args[1]", os.Args)
//	//startClientMode()
//	//if len(os.Args) == 2 && strings.ToLower(os.Args[1]) == "server" {
//		startServerMode()
//	//} else if len(os.Args) == 2 && strings.ToLower(os.Args[1]) == "client" {
//	//	startClientMode()
//	//} else {
//	//	panic("No know as a server or a client")
//	//}
//}


func  String(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}


