package main

import (
	"bytes"
	"fmt"
	"golang.org/x/net/websocket"
	"html/template"
	"io"
	"log"
	"github.com/server/srp6a"
	"net/http"
)

var (
	IsEnc bool 	= 	true
)
type Client struct {
	SendByteFlag    bool    // Whether already rent byte to client
	RejectReqFlag   bool   // Reject new request
	CloseFlag		bool   // client is to close
	Txid			int64
	Send_nonce 		int64
	Key 			[]byte
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
	Client
	WsConn 		*websocket.Conn
	srv 		srp6a.Srp6aServer
}

func (srvConn *ServerConn) initServerParam(ws *websocket.Conn){
	srvConn.WsConn 					  = ws
	srvConn.SendByteFlag			  = false
	srvConn.CloseFlag				  = false
	srvConn.RejectReqFlag			  = false
	srvConn.Txid    	  		  	  = 1
	srvConn.Send_nonce    	  		  = 1
	srvConn.NonceHex    		 	  = "22E7ADD93CFC6393C57EC0B3C17D6B44"
	srvConn.HeaderHex   		  	  = "126735FCC320D25A"
	srvConn.NonceList				  = make(map[int][]byte)
	srvConn.UnDealReplyList 		  = make(map[int][]byte)
	srvConn.ReceiveList 		      = make(map[int]int64)
	srvConn.SendList    		      = make(map[int]int64)
	srvConn.ReceiveDataList    		  = make(map[int64][]byte)
	srvConn.SendDataList   	  		  = make(map[int64][]byte)

}

func echo(ws *websocket.Conn) {
	var err error
	server := &ServerConn{}
	server.initServerParam(ws)
	fmt.Println("begin to listen")
	// Judge whether is server and do reference deal
	isServer := JudgeIsServer(ws)
	var i int = 0
loop:
	for {
		var reply string
		var res []byte
		// Active close. Send Bye to others
		if server.CloseFlag == true {
			suc := Close(server)
			if suc == true {  // graceful colse
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
			log.Fatalln("=========== EOF ERROR")
		} else if err != nil {
			log.Fatalln("Can't receive",err.Error())
			break
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
		// message is same,include txid do nothing
		if IsRepeatData(server, []byte(reply[:])) {
			continue
		}
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
				res = PackQuest(server, IsEnc)
			case 'Q': 				//Q
				i++
				if i > 3 {
					server.CloseFlag = true
					server.RejectReqFlag = true
					i = 0
				}
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
				flag := GracefulClose(server)
				if flag {
					ws.Close()
					break loop
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

func main() {
	// receive websocket router addrsess
	http.Handle("/", websocket.Handler(echo))
	//html layout
	http.HandleFunc("/web", web)

	if err := http.ListenAndServe(":8888", nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
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
