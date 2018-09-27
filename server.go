package main

import (
	"fmt"
	"html/template" //支持模板html
	"log"
	"net/http"
	"golang.org/x/net/websocket"

	"io"
	"time"
)

var (
	isEnc bool 	= 	true
	closeFlag bool = false
)
type server struct {
	conns       map[websocket.Conn]struct{}
	wsConn			*websocket.Conn
}

func echo(ws *websocket.Conn) {
	var err error
	var greetByte []byte
	fmt.Println("begin to listen")
	if isEnc == true {
		greetByte = _OutCheck()
	} else {
		greetByte = theHelloMessages.sendHello()
	}
	err = websocket.Message.Send(ws, greetByte)
	if err != nil {
		panic("send failed:")
	}
	//var i int = 0
loop:
	for {
		var reply string
		var res []byte
		err = websocket.Message.Receive(ws, &reply);	//websocket接受信息
		if err == io.EOF {
			log.Fatalln("=========== EOF ERROR")
		} else if err != nil {
			log.Fatalln("Can't receive",err.Error())
			break
		}
		if closeFlag == true {
			suc := gracefulClose(ws)
			ws.SetDeadline(time.Now())    // Stop receive new request
			if suc == true {  // graceful colse
				closeFlag = false
			}
			continue
		}
		fmt.Println("received from client: ",reply)
		if  (len(reply) > 16) && (reply[8] == 0x58) && (reply[11] == 0x01) { // encrypt
			nonce := []byte(reply[:8])
			fmt.Println("----", nonce)
			reply = reply[8:]
		}
		header :=[]byte(reply[:8])
		head := buildHeader(header)
		fmt.Println("head: ",[]byte(reply[1:]))
		if wrong := checkHeader(head); wrong != nil {
			break
		}
		switch head.Type {
		case 'Q': 				//Q
			//i++
			//if i > 3 {
			//	closeFlag = true
			//}
			res = dealRequest(reply)
			if res == nil {
				continue
			}
		case 'C':
			res = _InCheck(reply)
			if res[4] == 0x01 { // encrypt
				websocket.Message.Send(ws, res);
				res = theHelloMessages.sendHello()
			}
		case 'B':               //B
			ws.Close()
			break loop
		default:
			log.Fatalln("ERROR")
		}

/*
		//对接收的二进制字符串进行处理
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
		//这里是发送消息
		err = websocket.Message.Send(ws, res);
		if err != nil {
			fmt.Println("send failed:", err, )
			break
		}
	}
}


func main() {
	//接受websocket的路由地址
	http.Handle("/", websocket.Handler(echo))
	//html页面
	http.HandleFunc("/web", web)
	//client ip 192.168.200.113:8888
	if err := http.ListenAndServe(":8888", nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

func web(w http.ResponseWriter, r *http.Request) {
	//打印请求的方法
	fmt.Println("method", r.Method, r.URL)
	if r.Method == "GET" { //如果请求方法为get显示login.html,并相应给前端
		t, _ := template.ParseFiles("./index.html")
		t.Execute(w, nil)
		fmt.Println("==========:", w)
	} else {
		//否则走打印输出post接受的参数username和password
		fmt.Println(r.PostFormValue("username"))
		fmt.Println(r.PostFormValue("password"))
	}
}