package main

import (
	"bytes"
	"fmt"
	"github.com/server/srp6a"
	"golang.org/x/net/websocket"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"unsafe"
)

var (
	IsEnc bool 	= 	true
)
type Config struct {
	AuthEnc 		bool // authenticated encryption
	Txid			int64
	Send_nonce 		int64
	CommonKey 		[]byte
	NonceHex 		string
	HeaderHex 		string
	NonceList       map[int][]byte
	UnDealReplyList	map[int][]byte
	ReceiveList 	map[int]int64   //  As a server receive txid list from client
	SendList		map[int]int64    // As a client active request to server txid list
	ReceiveDataList map[int64][]byte  //  As a server receive data list which the key is txid from client
	SendDataList 	map[int64][]byte  // As a client active request to server data list which the key is txid
}

type ServerConn struct {
	RejectReqFlag   bool   // Reject new request
	CloseFlag		bool   // client is to close
	cli				*Client// as a client
	Config
	PeersNum    	 int
	MaxPeers     	int // MaxPeers is the maximum number of peers that can be connected
	WsConn 			*websocket.Conn
	srv 			srp6a.Srp6aServer
	//lock			sync.Mutex    // protect running
	//running    	 bool
}

func (srvConn *ServerConn) initServerParam(ws *websocket.Conn){
	srvConn.WsConn 					  = ws
	srvConn.CloseFlag				  = false
	srvConn.RejectReqFlag			  = false
	srvConn.Txid    	  		  	  = 1
	srvConn.Send_nonce    	  		  = 1
	srvConn.PeersNum				  = 0
	srvConn.MaxPeers				  = 10000000
	srvConn.NonceHex    		 	  = "22E7ADD93CFC6393C57EC0B3C17D6B44"
	srvConn.HeaderHex   		  	  = "126735FCC320D25A"
	srvConn.NonceList				  = make(map[int][]byte)
	srvConn.UnDealReplyList 		  = make(map[int][]byte)
	srvConn.ReceiveList 		      = make(map[int]int64)
	srvConn.SendList    		      = make(map[int]int64)
	srvConn.ReceiveDataList    		  = make(map[int64][]byte)
	srvConn.SendDataList   	  		  = make(map[int64][]byte)
	srvConn.srv						  = srp6a.Srp6aServer{}
}

//func echo(ws *websocket.Conn) {
//	var err error
//	server := &ServerConn{}
//	server.initServerParam(ws)
//	fmt.Println("begin to listen")
//	// Judge whether is server and do reference deal
//	isServer := JudgeIsServer(ws)
//	//var i int = 0
//
//	for {
//		var reply string
//		var res []byte
//		// Active close. Send Bye to others
//		if server.CloseFlag {
//			if suc := Close(server); suc {  // graceful colse
//				server.CloseFlag = false
//				server.RejectReqFlag = false // Alread send
//				break
//			}
//		}
//		//If RejectReqFlag is false, then websocket receive message
//		// else no longer accept new requests.
//		if !server.RejectReqFlag {
//			err = websocket.Message.Receive(ws, &reply);
//		}
//		if err == io.EOF {
//			panic("=========== Read ERROR: Connection has already broken of")
//		} else if err != nil {
//			panic(err.Error())
//		}
//		if  (len(reply) > 16) && (reply[8] == 0x58) && (reply[11] == 0x01) { // encrypt
//			nonce := []byte(reply[:8])
//			for _, val := range server.NonceList {  // nonce is same, do nothing
//				if bytes.Equal(val, nonce) {
//					panic("Data have been receive")
//				}
//			}
//			server.NonceList[len(server.NonceList)] = nonce
//			reply = reply[8:]
//		}
//
//		server.UnDealReplyList[len(server.UnDealReplyList)] = []byte(reply[:])
//		header   := []byte(reply[:8])
//		head	 := GetHeader(header)
//		//fmt.Println("head: ",head)
//		if err = CheckHeader(head); err != nil {
//			break
//		}
//		DeleteUndealData(server, []byte(reply[:]))  // Reply is to deal
//		switch head.Type {
//			case 'H':
//				// TODO  service, method, ctx, args
//				if bytes.Equal(server.CommonKey, nil) {
//					IsEnc = false
//				}
//				ctx := make(map[string]interface{})
//				arg := make(map[string]interface{})
//				res = PackQuest(server, IsEnc, "service", "method", ctx, arg)
//			case 'Q': 				//Q
//				//i++
//				//if i > 3 {
//				//	server.CloseFlag = true
//				//	server.RejectReqFlag = true
//				//	i = 0
//				//}
//				res = DealRequest(server, reply)
//				if res == nil {
//					continue
//				}
//			case 'C':
//				if !isServer {  // client
//					res = UnpackCheck(server, reply)
//				} else { // server
//					res = DealCheck(server, reply)  // TODO UnTest
//					if res[4] == 0x01 { // encrypt
//						websocket.Message.Send(ws, res);
//						res = HelloMessage.sendHello()
//					}
//				}
//			case 'A': 				//A
//				res = DealAnswer(server, reply)
//				if res == nil {
//					continue
//				}
//			case 'B':               //B
//				server.RejectReqFlag = true
//				if flag := GracefulClose(server); flag {
//					ws.Close()
//					return
//				} else {
//					continue
//				}
//			default:
//				log.Fatalln("ERROR")
//			}
//
//		// The message will send
//		err = websocket.Message.Send(ws, res);
//		if err != nil {
//			fmt.Println("send failed:", err, )
//			break
//		}
//	}
//}

//func main() {
//	// receive websocket router addrsess
//	http.Handle("/", websocket.Handler(echo))
//	//html layout
//	http.HandleFunc("/web", web)
//
//	if err := http.ListenAndServe(":8989", nil); err != nil {
//		log.Fatal("ListenAndServe:", err)
//	}
//	//ClientSocket()
//}

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

func EchoHandle(ws *websocket.Conn) {
	var err error
		server := &ServerConn{}
		server.initServerParam(ws)
		fmt.Println("begin to listen")
		// Judge whether is server and do reference deal
		isServer := JudgeIsServer(ws)
		//var i int = 0

		for {
			var reply string
			var res []byte
			// Active close. Send Bye to others
			if server.CloseFlag {
				if suc := Close(server); suc {  // graceful colse
					server.CloseFlag = false
					server.RejectReqFlag = false // Alread send
					break
				}
			}
			//If RejectReqFlag is false, then websocket receive message
			// else no longer accept new requests.
			if !server.RejectReqFlag {
				err = websocket.Message.Receive(ws, &reply);
			}
			if err == io.EOF {
				panic("=========== Read ERROR: Connection has already broken of")
			} else if err != nil {
				panic(err.Error())
			}
			if  (len(reply) > 16) && (reply[8] == 0x58) && (reply[11] == 0x01) { // encrypt
				nonce := []byte(reply[:8])
				for _, val := range server.NonceList {  // nonce is same, do nothing
					if bytes.Equal(val, nonce) {
						panic("Data have been receive")
					}
				}
				server.NonceList[len(server.NonceList)] = nonce
				reply = reply[8:]
			}

			server.UnDealReplyList[len(server.UnDealReplyList)] = []byte(reply[:])
			header   := []byte(reply[:8])
			head	 := GetHeader(header)
			//fmt.Println("head: ",head)
			if err = CheckHeader(head); err != nil {
				break
			}
			DeleteUndealData(server, []byte(reply[:]))  // Reply is to deal
			switch head.Type {
				case 'H':
					// TODO  service, method, ctx, args
					if bytes.Equal(server.CommonKey, nil) {
						IsEnc = false
					}
					ctx := make(map[string]interface{})
					arg := make(map[string]interface{})
					res = PackQuest(server, IsEnc, "service", "method", ctx, arg)
				case 'Q': 				//Q
					//i++
					//if i > 3 {
					//	server.CloseFlag = true
					//	server.RejectReqFlag = true
					//	i = 0
					//}
					res = DealRequest(server, reply)
					if res == nil {
						continue
					}
				case 'C':
					if !isServer {  // client
						res = UnpackCheck(server, reply)
					} else { // server
						res = DealCheck(server, reply)  // TODO UnTest
						if res[4] == 0x01 { // encrypt
							websocket.Message.Send(ws, res);
							res = HelloMessage.sendHello()
						}
					}
				case 'A': 				//A
					res = DealAnswer(server, reply)
					if res == nil {
						continue
					}
				case 'B':               //B
					server.RejectReqFlag = true
					if flag := GracefulClose(server); flag {
						ws.Close()
						return
					} else {
						continue
					}
				default:
					log.Fatalln("ERROR")
				}

			// The message will send
			err = websocket.Message.Send(ws, res);
			if err != nil {
				fmt.Println("send failed:", err, )
				break
			}
		}
}

func receiverAsServer() {
	http.Handle("/", websocket.Handler(EchoHandle))

	http.HandleFunc("/web", web)

	if err := http.ListenAndServe(":8989", nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

func initiatorAsClient() {
	var reply string
	var res []byte

	origin := "http://127.0.0.1:8989/"
	// "ws://192.168.200.40:8989/"
	ws_url := "ws://192.168.32.1:8989/"
	ws, err := websocket.Dial(ws_url,"",origin)
	if err != nil {
		log.Fatal(err)
	}
	server := &ServerConn{}
	server.cli = server.cli.DefaultInitPeer(ws)
	fmt.Println("-----establish connect-------")
	// If donnot set msg size, it cannot read the data.
	//var msg = make([]byte, 512, 1024 * 8)
	for {
		//var n int
		//n, err = ws.Read(msg)
		err = websocket.Message.Receive(ws, &reply)
		if err == io.EOF {
			panic("=========== Read ERROR: Connection has already broken of")
		} else if err != nil {
			panic(err.Error())
		}
		//if  err != nil {  // receive data
		//	log.Fatal(err)
		//} else if (string.Equal(reply, nil)) {
		//	continue
		//}
		server.cli.AlreadyDeal = false
		//reply = String(msg[:n])

		fmt.Printf("Received All: %s.\n", reply)
		if  (len(reply) > 16) && (reply[8] == 0x58) && (reply[11] == 0x01) { // encrypt
			nonce := []byte(reply[:8])
			for _, val := range server.cli.cfg.NonceList {  // nonce is same, do nothing
				if bytes.Equal(val, nonce) {
					panic("Data have been receive")
				}
			}
			server.cli.cfg.NonceList[len(server.cli.cfg.NonceList)] = nonce
			reply = reply[8:]
		}
		header   := []byte(reply[:8])
		head	 := GetHeader(header)
		//fmt.Println("head: ",head)
		if err = CheckHeader(head); err != nil {
			break
		}
		switch head.Type {
		case 'H':
			if bytes.Equal(server.cli.cfg.CommonKey, nil) {
				IsEnc = false
			}
			ctx := make(map[string]interface{})
			arg := make(map[string]interface{})
			arg["first"] = "this is the first data client send"
			arg["second"] = "this is second data "
			res = PackQuest(server, IsEnc, "service", "method", ctx, arg)
		case 'Q': 				//Q
			res = DealRequest(server, reply)
			if res == nil {
				continue
			}
		case 'C':
			res = UnpackCheck(server, reply)
		case 'A': 				//A
			res = DealAnswer(server, reply)
			if res == nil {
				continue
			}
		case 'B':               //B
			server.RejectReqFlag = true
			if flag := GracefulClose(server); flag {
				ws.Close()
				return
			} else {
				continue
			}
		default:
			log.Fatalln("ERROR")
		}
		if err := websocket.Message.Send(ws, res); err != nil {  // send data
			log.Fatal(err)
		}
		fmt.Println("Send data ", res)
		server.cli.AlreadyDeal = true
	}
	
}


func main() {
	fmt.Println("os.Args[1]", os.Args)
	initiatorAsClient()
	//if len(os.Args) == 2 && strings.ToLower(os.Args[1]) == "server" {
	//	receiverAsServer()
	//} else if len(os.Args) == 2 && strings.ToLower(os.Args[1]) == "client" {
	//	initiatorAsClient()
	//} else {
	//	panic("No know as a server or a client")
	//}


	// receive websocket router addrsess
	//http.Handle("/", websocket.Handler(echo))
	//http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
	//
	//})
	//html layout
	//http.HandleFunc("/web", web)
	//
	//if err := http.ListenAndServe(":8989", nil); err != nil {
	//	log.Fatal("ListenAndServe:", err)
	//}
}

func  String(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

