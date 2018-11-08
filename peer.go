package main

import (
	"github.com/server/srp6a"
	"golang.org/x/net/websocket"
	"time"
)

var maxAttempTimes int8 = 3
type ClientConfig struct {
	err 			error
	readyState		int
	attempTimes		int
	ReconnectFlag   bool
	StopFlag 		bool
	AlreadyDeal     bool
}

type Client struct {
	ClientConfig
	cfg 				*Config
	HandshakeTimeout 	time.Duration
	DialTimeout      	time.Duration
	conn 				*websocket.Conn
	client				srp6a.Srp6aClient
}

func (cli *Client) DefaultInitPeer(ws *websocket.Conn) *Client{
	conf := new(Config)
	conf.AuthEnc				= false
	conf.NonceHex				= "22E7ADD93CFC6393C57EC0B3C17D6B44"
	conf.HeaderHex				= "126735FCC320D25A"
	conf.NonceList				= make(map[int][]byte)
	conf.UnDealReplyList 		= make(map[int][]byte)
	conf.ReceiveList 		    = make(map[int]int64)
	conf.SendList    		    = make(map[int]int64)
	conf.ReceiveDataList    	= make(map[int64][]byte)
	conf.SendDataList			= make(map[int64][]byte)
	return &Client{
		cfg: conf,
		conn: ws,
		client: srp6a.Srp6aClient{},
		ClientConfig: ClientConfig{
			AlreadyDeal: false,
			attempTimes: 0,
			ReconnectFlag: false,
			StopFlag: false,
		},
	}
}

//func (cli *Client) InitPeerConfig(ws *websocket.Conn) {
//	cli.conn              		  = ws
//	cli.Txid    	  		  	  = 1
//	cli.Send_nonce    	  		  = 2
//	cli.NonceHex    		 	  = "22E7ADD93CFC6393C57EC0B3C17D6B44"
//	cli.HeaderHex   		  	  = "126735FCC320D25A"
//	cli.AlreadyDeal				  = false
//	cli.NonceList				  = make(map[int][]byte)
//	cli.UnDealReplyList 		  = make(map[int][]byte)
//	cli.ReceiveList 		      = make(map[int]int64)
//	cli.SendList    		      = make(map[int]int64)
//	cli.ReceiveDataList    		  = make(map[int64][]byte)
//	cli.SendDataList   	  		  = make(map[int64][]byte)
//	cli.client					  =  srp6a.Srp6aClient{}
//}

//func ClientSocket() {
//	 origin := "http://localhost/"
//	 ws_url := "ws://192.168.200.40/"
//	 ws, err := websocket.Dial(ws_url,"8989",origin)
//	 if err != nil {
//	 	log.Fatal(err)
//	 }
//	 if _, err := ws.Write([]byte("hell world！")); err != nil {  // send data
//	 	 log.Fatal(err)
//	 }
//	 var msg = make([]byte, 512)
//	 var n int
//	 if n, err = ws.Read(msg); err != nil {  // receive data
//	 	log.Fatal(err)
//	 }
//	 fmt.Printf("Received: %s.\n", msg[:n])
//
//}
