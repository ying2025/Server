package main

import (
	"fmt"
	"html/template" //支持模板html
	"log"
	"net/http"
	"golang.org/x/net/websocket"

	"io"
)

var (
	IsEnc bool 	= 	true
	closeFlag bool = false
)
type Client struct {
	Txid			int64
	Send_nonce 		int64
	Key []byte
	NonceHex 		string
	HeaderHex 		string
	ReceiveList 	map[int]int64
	SendList		map[int]int64
	ReceiveDataList map[int64][]byte  // temp receive data
	SendDataList 	map[int64][]byte  // temp send data
}

type ServerConn struct {
	Client
	WsConn 		*websocket.Conn
}

func (srvConn *ServerConn) newServer(ws *websocket.Conn){
	srvConn.WsConn 					  = ws
	srvConn.Txid    	  		  	  = 1
	srvConn.Send_nonce    	  		  = 1
	srvConn.NonceHex    		 	  = "22E7ADD93CFC6393C57EC0B3C17D6B44"
	srvConn.HeaderHex   		  	  = "126735FCC320D25A"
	srvConn.ReceiveList 		      = make(map[int]int64)
	srvConn.SendList    		      = make(map[int]int64)
	srvConn.ReceiveDataList    		  = make(map[int64][]byte)
	srvConn.SendDataList   	  		  = make(map[int64][]byte)
}

func echo(ws *websocket.Conn) {
	var err error
	server := &ServerConn{}
	server.newServer(ws)
	fmt.Println("begin to listen")
	isServer := JudgeIsServer(ws)
	//var i int = 0
loop:
	for {
		var reply string
		var res []byte

		err = websocket.Message.Receive(ws, &reply);	//websocket receive message
		if err == io.EOF {
			log.Fatalln("=========== EOF ERROR")
		} else if err != nil {
			log.Fatalln("Can't receive",err.Error())
			break
		}
		if closeFlag == true {
			suc := GracefulClose(server)
			if suc == true {  // graceful colse
				closeFlag = false
				break
			}
			continue
		}
		//fmt.Println("received from client: ",reply)
		if  (len(reply) > 16) && (reply[8] == 0x58) && (reply[11] == 0x01) { // encrypt
			//nonce := []byte(reply[:8])
			//fmt.Println("---nonce-", nonce)
			reply = reply[8:]
		}
		header :=[]byte(reply[:8])
		head := BuildHeader(header)
		fmt.Println("head: ",head)
		if wrong := CheckHeader(head); wrong != nil {
			break
		}
		switch head.Type {
		case 'H':
			// TODO  service, method, ctx, args
			res = PackQuest(server, IsEnc)
		case 'Q': 				//Q
			//i++
			//if i > 5 {
			//	closeFlag = true
			//}
			res = DealRequest(server, reply)
			if res == nil {
				continue
			}
		case 'C':
			if !isServer {  // client
				res = UnpackCheck(server, reply)
			} else { // server
				res = DealCheck(server, reply)
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
			ws.Close()
			break loop
		default:
			log.Fatalln("ERROR")
		}

/*
		// deal binary string from client
		//decode data
		var v interface{}
		var data []byte
		fmt.Println("data is", data)
		_, err := vbs.Unmarshal(data, &v)
		if err == nil {
			fmt.Println("resdata is : ", reflect.TypeOf(v), v)
		} else {
			fmt.Println("Error resdata is  %V", err)
		}
		//srv := &http.Server{
		//	ReadTimeout: 30 * time.Second,
		//	WriteTimeout: 50 * time.Second,
		//}
		//if d := srv.ReadTimeout; d != 0 {
		//	ws.SetReadDeadline(time.Now().Add(d))
		//}
		//if d := srv.WriteTimeout; d != 0 {
		//	ws.SetWriteDeadline(time.Now().Add(d))
		//}
*/

		//encode data
 /*
		vals := v.(map[string]interface{})
		vals["title"] = "A"
		fmt.Println("vals is ===: ", vals)
		buf, err := vbs.Marshal(result)
		if err != nil {
			fmt.Println("error encoding %v:", err)
		}
		fmt.Printf("Marshal success: ", reflect.TypeOf(buf), buf)
		for _, x := range buf {
			results = append(results, x)
		}
	*/
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
	//go func() {log.Fatal("ListenAndServe:", http.ListenAndServe(":8888", nil))} ()
	if err := http.ListenAndServe(":8888", nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

func web(w http.ResponseWriter, r *http.Request) {
	// print request method
	fmt.Println("method", r.Method, r.URL)
	if r.Method == "GET" {  // Show login.html anf transfer it to  front-end
		t, _ := template.ParseFiles("./index.html")
		t.Execute(w, nil)
		fmt.Println("==========:", w)
	} else { // print the param (username and password) of receiving
		fmt.Println(r.PostFormValue("username"))
		fmt.Println(r.PostFormValue("password"))
	}
}
