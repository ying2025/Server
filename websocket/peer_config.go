package websocket

import (
	"github.com/server/srp6a"
	"golang.org/x/net/websocket"
	"time"
)

var maxAttempTimes int8 = 3
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

func InitConfig() *Config {
	return &Config {
		Txid:				1,
		AuthEnc:    		false,
		NonceHex:   		"22E7ADD93CFC6393C57EC0B3C17D6B44",
		HeaderHex:  		"126735FCC320D25A",
		NonceList:   		make(map[int][]byte),
		UnDealReplyList: 	make(map[int][]byte),
		ReceiveList: 		make(map[int]int64),
		SendList: 			make(map[int]int64),
		ReceiveDataList: 	make(map[int64][]byte),
		SendDataList: 		make(map[int64][]byte),
	}
}

type ServerConfig struct {
	RejectReqFlag   bool   // Reject new request
	CloseFlag		bool   // client is to close
	PeerCount    	 int
	MaxPeers     	int // MaxPeers is the maximum number of peers that can be connected
	srv 			srp6a.Srp6aServer
	//lock			sync.Mutex    // protect running
	//running    	 bool
}

func NewServerConfig(ws *websocket.Conn) *PeerConn{
	cfg := InitConfig()
	return &PeerConn{
		WsConn: ws,
		Config: cfg,
		ServerConfig: &ServerConfig{
			CloseFlag: false,
			RejectReqFlag: false,
			PeerCount: 0,
			MaxPeers:10000000,
			srv: srp6a.Srp6aServer{},
		},
	}
}

type ClientConfig struct {
	err 			error
	readyState		int
	attempTimes		int
	ReconnectFlag   bool
	StopFlag 		bool
	AlreadyDeal     bool
	HandshakeTimeout 	time.Duration
	DialTimeout      	time.Duration
	cli				srp6a.Srp6aClient
}

func NewClientConfig(ws *websocket.Conn) *PeerConn{
	cfg := InitConfig()
	return &PeerConn{
		WsConn: ws,
		Config: cfg,
		client: &ClientConfig{
			AlreadyDeal: false,
			attempTimes: 0,
			ReconnectFlag: false,
			StopFlag: false,
			cli: 	srp6a.Srp6aClient{},
		},
	}
}
