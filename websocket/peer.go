package websocket

import (
	"bytes"
	"flag"
	"fmt"
	"golang.org/x/net/websocket"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"
	"unsafe"
)

var (
	IsEnc bool 	= 	true
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

func EchoHandle(ws *websocket.Conn) {
		var err error
		server := NewServerConfig(ws)
		server.PeerCount++
		fmt.Println("begin to listen")
		// Judge whether is server and do reference deal
		isServer := JudgeIsServer(ws)
		//var i int = 0

		for {
			var reply string
			var res []byte
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
					//if i > 2 {
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
						server.PeerCount--
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
	ws_url := "ws://192.168.200.40:8989/"
	ws, err := websocket.Dial(ws_url,"",origin)
	if err != nil {
		log.Fatal(err)
	}
	peer := NewClientConfig(ws)
	// If donnot set msg size, it cannot read the data.
	for {
		err = websocket.Message.Receive(ws, &reply)
		if err == io.EOF {
			panic("=========== Read ERROR: Connection has already broken of")
		} else if err != nil {
			panic(err.Error())
		}

		peer.client.AlreadyDeal = false
		if  (len(reply) > 16) && (reply[8] == 0x58) && (reply[11] == 0x01) { // encrypt
			nonce := []byte(reply[:8])
			for _, val := range peer.NonceList {  // nonce is same, do nothing
				if bytes.Equal(val, nonce) {
					panic("Data have been receive")
				}
			}
			peer.NonceList[len(peer.NonceList)] = nonce
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
			if bytes.Equal(peer.CommonKey, nil) {
				IsEnc = false
			}
			ctx := make(map[string]interface{})
			arg := make(map[string]interface{})
			arg["first"] = "this is the first data client send"
			arg["second"] = "this is second data "
			res = PackQuest(peer, IsEnc, "service", "method", ctx, arg)
		case 'Q': 				//Q
			res = DealRequest(peer, reply)
			if res == nil {
				continue
			}
		case 'C':
			res = UnpackCheck(peer, reply)
		case 'A': 				//A
			res = DealAnswer(peer, reply)
			if res == nil {
				continue
			}
		case 'B':               //B
			peer.RejectReqFlag = true
			if flag := GracefulClose(peer); flag {
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
		peer.client.AlreadyDeal = true
	}

}

func StartNode() {
	flagMode := flag.String("mode", "client", "start in client or server")
	flag.Parse()
	if strings.ToLower(*flagMode) == "server" {
		receiverAsServer()
	} else {
		initiatorAsClient()
	}
	// command mode
	//if len(os.Args) == 2 && strings.ToLower(os.Args[1]) == "server" {
	//	receiverAsServer()
	//} else if len(os.Args) == 2 && strings.ToLower(os.Args[1]) == "client" {
	//
	//} else {
	//	panic("No know as a server or a client")
	//}
}



func  String(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}


