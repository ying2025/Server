package main

type MsgType byte

//消息最大的长度
var MaxMessageSize = 64*1024*1024
type Context map[string]interface{}
type _ByeMessage struct {
}
type _HelloMessage struct {
}

//check类型的数据结构
type check struct {
	command string
	args map[string]interface{}
	reserved int
	_InMsg
}
//解析answer类型的数据结构
type answer struct{
	txid  int64
	status int64
	args Context
}

//发送出去的answer类型的数据结构
type _OutAnswer struct {
	txid int64
	reserved int
	start int
	buf []byte
}


type _InMsg struct {
	argsOff int
	buf []byte
}

type _InAnswer struct {
	txid int64
	status int
	args	   map[string]interface{}
	_InMsg
}
//解析的quest数据的结构类型
type _InQuest struct {
	txid int64
	repeatFlag bool
	service string
	method string
	ctx map[string]interface {}
	_InMsg
	args map[string]interface {}
}
// return Q type message
func (q *_InQuest) Type() MsgType {
	return 'Q'
}

type _OutQuest struct {
	txid int64
	reserved int
	start int
	buf []byte
}
func (q *_OutQuest) Type() MsgType {
	return 'Q'
}

//消息头部结构
type MessageHeader struct {
	Magic byte	// 'X'
	Version byte	// '!'
	Type byte	// 'Q', 'A', 'H', 'B', 'C'
	Flags byte	// 0x00 or 0x01
	BodySize int32	// in big endian byte order
}
