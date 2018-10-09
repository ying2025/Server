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


func echo(ws *websocket.Conn) {
	var err error
	fmt.Println("begin to listen")
	isServer := JudgeIsServer(ws)
	var i int = 0
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
			suc := gracefulClose(ws)
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
			res = PackQuest(IsEnc)
		case 'Q': 				//Q
			i++
			if i > 5 {
				closeFlag = true
			}
			res = DealRequest(reply)
			if res == nil {
				continue
			}
		case 'C':
			if !isServer {  // client
				res = UnpackCheck(reply)
			} else { // server
				res = DealCheck(reply)
				if res[4] == 0x01 { // encrypt
					websocket.Message.Send(ws, res);
					res = HelloMessage.sendHello()
				}
			}
		case 'A': 				//A
			res = DealAnswer(reply)
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
