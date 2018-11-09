package main

import (
	"bytes"
	"crypto/aes"
	"encoding/hex"
	"fmt"
	"github.com/server/eax"
	"github.com/server/srp6a"
	"github.com/server/vbs"
	"golang.org/x/net/websocket"
	"log"
	"time"
)

const (
	send_add_state int64 = 2
)
// Judge the connection whether is Client
// If it is client connect, return this code is not a server side
// Or check the encrypt flag. If the flag is true, then pack Authenticate message to client, else send H type to client
func JudgeIsServer(ws *websocket.Conn) bool{
	isClient := ws.IsClientConn()
	if isClient {
		return false
	}
	// server side
	var greetByte []byte
	var err error
	if IsEnc == true {
		greetByte = _OutCheck()
	} else {
		greetByte = HelloMessage.sendHello()
	}
	err = websocket.Message.Send(ws, greetByte)
	if err != nil {
		panic("send failed:")
	}
	return true
}

// Resovle the message header
func GetHeader(header []byte) MessageHeader{
	var head MessageHeader
	head.Magic = header[0]
	head.Version = header[1]
	head.Type = header[2]
	head.Flags = header[3]
	head.BodySize = int32(header[4]) << 24 + int32(header[5]) << 16 + int32(header[6]) << 8 +int32(header[7])
	return head
}
// check header whether is qualified
func CheckHeader(header MessageHeader) error {
	if header.Magic != 'X' || header.Version != '!' {
		return fmt.Errorf("Unknown message Magic(%d) and Version(%d)", header.Magic, header.Version)
	}

	switch header.Type {
	case 'Q', 'A', 'C':
		if header.Flags != 0 && header.Flags != 0x01 {
			return fmt.Errorf("Unknown message Flags")
		} else if int(header.BodySize) > MaxMessageSize {
			return fmt.Errorf("Message size too large")
		}
	case 'H', 'B':
		if header.Flags != 0 || header.BodySize != 0 {
			return fmt.Errorf("Invalid Hello or Bye message")
		}
	default:
		return fmt.Errorf("Unknown message Type(%d)", header.Type)
	}
	return  nil
}
// Judge whether receive repeate data.
//func IsRepeatData(srvConn *PeerConn, reply []byte) bool{
//	for _, value := range srvConn.UnDealReplyList {
//		if bytes.Equal(value, reply) {
//			return true
//		}
//	}
//	srvConn.UnDealReplyList[len(srvConn.UnDealReplyList)] = reply
//	return false
//}
// If the data is to deal, delete it from the UndealData List
func DeleteUndealData(srvConn *PeerConn, reply []byte) {
	for j, value := range srvConn.UnDealReplyList {
		if bytes.Equal(value, reply) {
			delete(srvConn.UnDealReplyList, j)
			return
		}
	}
}
// pack the header of XIC protocol 
func fillHeader(size int,t MsgType) []byte{
	var packet []byte
	packet= append(packet,0x58) //X
	packet = append(packet,0x21)  //!
	packet = append(packet,byte(t))  //A
	packet =append(packet, 0x00)  //default 0x00 (encrypt is 0x01)
	packet  = append(packet,byte(size >> 24))  //bodySize, 4 bytes
	packet = append(packet,byte(size >> 16))
	packet  = append(packet,byte(size >> 8))
	packet  = append(packet,byte(size))
	return packet
}
// encode Nonce with VBS
func packNonce(nonce int64) []byte{
	b := &bytes.Buffer{}
	enc := vbs.NewEncoder(b)
	err := enc.Encode(nonce)
	if err != nil {
		panic("vbs.Encoder error when encode send_nonce")
	}
	//send_nonce += send_add_state  // nonce increase
	return b.Bytes()
}
// pack command and args with VBS
// result contain header and C type message that encode with VBS
func packCheckCmd(command string, args map[string]interface{}) []byte{
	var err error
	b := &bytes.Buffer{}
	enc := vbs.NewEncoder(b)
	enc.Encode(command)
	err = enc.Encode(args)
	if err != nil {
		panic("vbs.Encoder error")
	}
	result := make([]byte, enc.Size()+8, (enc.Size())*2) // data to send to client
	packet := fillHeader(enc.Size(),'C')
	if command == "SRP6a4" {  // encrypt flag. If the client verify success with M2, then it can send H type message
		packet[4] = 0x01
	}
	copy(result[:8],packet)
	copy(result[8:],b.Bytes())
	return result
}
// declare H type message
var HelloMessage = _HelloMessage{}

//H type content
var helloMessageBytes = [8]byte{'X','!','H'}

// return H type message
func (m _HelloMessage) sendHello() []byte {
	return helloMessageBytes[:]
}
// server side
// SRP6a consult the common key, return C type message data
//  pack the command and args, then decode outcheck with VBS
// pack header and encode outcheck message, then send the C type message
func  _OutCheck() []byte{
	c := &check{}
	c.command = "AUTHENTICATE"
	args := make(map[string]interface{})
	args["method"] = "SRP6a"
	c.args = args
	return packCheckCmd(c.command, c.args)
}
// Resolve C type message, get command and args
// According to command, send different command and param.
func UnpackCheck(srvConn *PeerConn, reply string) []byte{
	data := getData(srvConn, 0x00, reply)
	c := decodeInCheck(data)
	result := handleCmd(srvConn, c)
	return result
}
// decode C type message
func decodeInCheck(buf []byte) *_InCheck {
	c := &_InCheck{}
	dec := vbs.NewDecoderBytes(buf)
	dec.Decode(&c.cmd)
	dec.Decode(&c.args)
	c.argsOff = dec.Size()
	c.buf = buf
	return c
}

// Judge what type of command, then send the reference message to client
func handleCmd(srvConn *PeerConn, c *_InCheck) []byte{
	var msg []byte
	switch c.cmd {
	case "FORBIDDEN":
		reason := c.args["reason"]
		panic("Authentication Exception, SRP6a Verify fail! The reason is: " + reason.(string))
	case "AUTHENTICATE":
		msg = sendSrp6a1(c.args)
	case "SRP6a2":
		msg = sendSrp6a3(c.args)
	case "SRP6a4":
		msg = verifySrp6aM2(srvConn, c.args)
	default:
		panic("Unknown command type, SRP6a Verify fail !")
	}
	return msg
}
// resolve the request data from client that type is Q
// If txid is 0, do nothing, else if the message has alread recceived, pack the error message to client.
// else pack the answer to client
// Return ANSWER type message
func  DealRequest(srvConn *PeerConn, reply string) []byte{
	q := & _InQuest{}
	isEnc := reply[3]  // encrypt flag
	request, errAnswer := q.resolveRequest(srvConn, isEnc, reply) //resovle request

	txId := request.txid
	if txId == 0 {  // txid is 0, server is no response
		return nil
	}
	if request.repeatFlag {   // repeat data, direct pack
		return packAnswer(srvConn, isEnc, txId, errAnswer)
	}
	var a answer
	a = packAnswerBody(txId)
	fmt.Println("deal request ", txId)
	return packAnswer(srvConn, isEnc, txId, a)
}
// resolve Q type message from client
// Get the message body, decode data with VBS, get the param.
// Judge whether it receive the same message with different txid, If it is, then pack the error answer to client
// Or if txid is not equal 0, record the receive List. Record receive data list
func (q _InQuest) resolveRequest(srvConn *PeerConn, isEnc uint8, reply string) (_InQuest, answer){
	var errAnswer answer
	data := getData(srvConn, isEnc, reply)
	content := decodeInQuest(data)
	//request param
	txId := content.txid
	// judge whether is already receive the data expect txid
	for _, value := range srvConn.ReceiveDataList {
		if bytes.Equal(data[1:], value){
			errAnswer.status = 1
			msg := "message is duplication"
			arg := packExpArg("Receive duplication of data",int(txId) + 1000,"218",msg,"resolveRequest*service","Receive the same request")
			errAnswer.args =  arg
			content.repeatFlag = true
			return *content, errAnswer
		}
	}
	srvConn.ReceiveDataList[txId] = data[1:] // record receive data
	if txId != 0 {
		srvConn.ReceiveList[len(srvConn.ReceiveList)] = txId // record receive List
	}
	return *content, errAnswer
}
// decode Q type message
func decodeInQuest(buf []byte) *_InQuest {
	q := &_InQuest{}
	dec := vbs.NewDecoderBytes(buf)
	dec.Decode(&q.txid)
	dec.Decode(&q.service)
	dec.Decode(&q.method)
	dec.Decode(&q.ctx)
	q.argsOff = dec.Size()
	q.buf = buf
	return q
}
// Get the message body, If it encrypt, then decrypt it
func getData(srvConn *PeerConn, isEnc uint8, reply string) []byte{
	var data []byte
	len1 := int(reply[4]) << 24 + int(reply[5]) << 16 + int(reply[6]) << 8 +int(reply[7])
	if isEnc == 0x01 {
		data = []byte(reply[8:len1+24]) // meassage + MAC
		data = decrypt(srvConn, data)
		if len(data) != len1 {
			log.Fatalln("Data length not equal to bodysize")
		}
	} else {
		data = []byte(reply[8:len1+8])
		if len(data) != len(reply)-8 {
			log.Fatalln("Data length not equal to bodysize")
		}
	}
	return data
}
// Deal A type message
// Get answer type message
func DealAnswer(srvConn *PeerConn, reply string) []byte {
	isEnc := reply[3]  // encrypt flag
	data := getData(srvConn, isEnc, reply)

	a := decodeInAnswer(data)// decode data as array, and the length is 3.
	if a.status != 0 {
		panic(a.args)
	}
	deleteTxId(a.txid, srvConn.SendList) // delete txId that send from server
	return nil
}
// decode answer
func decodeInAnswer(buf []byte) *_InAnswer {
	a := &_InAnswer{}
	dec := vbs.NewDecoderBytes(buf)
	dec.Decode(&a.txid)
	dec.Decode(&a.status)
	dec.Decode(&a.args)
	a.argsOff = dec.Size()
	a.buf = buf
	return a
}
// pack Q type data
func PackQuest(srvConn *PeerConn, isEnc bool, service string, method string, ctx map[string]interface{}, arg map[string]interface{}) []byte{
	q := &_OutQuest{txid:srvConn.Txid}
	msg, size := q.encodeOutQuest(q.txid, service, method, ctx, arg)
	if q.txid != 0 {
		srvConn.SendList[len(srvConn.SendList)] = q.txid  // record send to server list
	}
	fmt.Println("msg", msg)
	//srvConn.cli.cfg.SendDataList[q.txid] = msg[1:]
	//srvConn.SendDataList[q.txid] = msg[1:]
	srvConn.Txid++
	return packMsg(srvConn, isEnc, size,'Q', msg)
}
// pack header and message, if the encrypt flags is true pack nonce, header, message
func packMsg(srvConn *PeerConn, isEnc bool, size int, msgType MsgType, msg []byte) []byte{
	var result []byte
	packet := fillHeader(size, msgType)// fill header message
	if isEnc == true {
		result = make([]byte, size+32)
		nonceNum := packNonce(srvConn.Send_nonce)
		srvConn.Send_nonce += send_add_state  // nonce increase
		copy(result[:8], nonceNum)
		copy(result[8:16],packet)
		result[11] = 0x01
		msg = encrypt(srvConn, msg)
		copy(result[16:],msg)
	} else {
		result = make([]byte, len(msg)+8, len(msg)*2)
		copy(result[:8],packet)
		copy(result[8:],msg)
	}
	return result
}
// encode Q type message
func (q _OutQuest) encodeOutQuest(txId int64, service, method string, ctx Context, args interface{}) ([]byte, int) {
	b := &bytes.Buffer{}
	enc := vbs.NewEncoder(b)
	enc.Encode(txId)
	enc.Encode(service)
	enc.Encode(method)
	enc.Encode(ctx)
	err := enc.Encode(args)
	if err != nil {
		panic("vbs.Encoder error")
	}
	q.buf = b.Bytes()
	return q.buf, enc.Size()
}
// encode A type message
func encodeOutAnswer(id int64,status int64, args interface{}) (*_OutAnswer,int){
	a := &_OutAnswer{txid:id, start:-1}
	b := &bytes.Buffer{}
	enc := vbs.NewEncoder(b)
	a.reserved = b.Len()
	enc.Encode(id)
	enc.Encode(status)
	err := enc.Encode(args)
	if err != nil {
		panic("vbs.Encoder error")
	}
	a.buf = b.Bytes()
	return a,enc.Size()
}
// pack answer message
func packAnswerBody(txid int64) answer {
	var a answer
	var err error
	arg  := make(map[string]interface{})
	// construct the data return to client
	a.txid = txid
	if err != nil {  // exception
		a.status = 1
		arg = packExpArg("",1001,"","","","")
	} else {  // normal
		a.status = 0
		arg["first"] = "this is server reply"
		arg["second"] = "this is Tim server"
	}
	a.args =  arg
	return a
}
// Assemble ANSWER type message
// pack answer with VBS, then assemble header and message, final delete the txid that already resolve
func packAnswer(srvConn *PeerConn, encFlag uint8,txId int64, a answer) []byte{
	content,size := encodeOutAnswer(txId, a.status,a.args) //construct a ANSWER type message
	isEnc := encFlag == 0x01
	var result []byte
	result = packMsg(srvConn, isEnc, size,'A', content.buf)
	deleteTxId(txId, srvConn.ReceiveList) // delete txId
	fmt.Println("delete txid ", txId)
	return result
}
// pack expection param
func packExpArg(name string, code int, tag string, msg string, raiser string, detail string) map[string]interface{}{
	arg  := make(map[string]interface{})
	arg["exname"] = name
	arg["code"] = code
	arg["tag"] = tag
	arg["message"] = msg
	arg["raiser"] = raiser
	arg["detail"] = detail
	return arg
}
// encrypt data with EAX
func encrypt(srvConn *PeerConn, msg []byte) []byte{
	nonce, _ := hex.DecodeString(srvConn.NonceHex)
	header, _ := hex.DecodeString(srvConn.HeaderHex)

	out := make([]byte, 256)
	blockCipher, _ := aes.NewCipher(srvConn.CommonKey)
	ax, _ := eax.NewEax(blockCipher)

	ax.Start(true, nonce, header)
	ax.Update(out, msg)
	ax.Finish(out[len(msg):])
	n := len(msg) + eax.BLOCK_SIZE
	return out[:n]
}
// decrypt data with EAX
func decrypt(srvConn *PeerConn, cipherMsg []byte) ([]byte){
	nonce, _ := hex.DecodeString(srvConn.NonceHex)
	header, _ := hex.DecodeString(srvConn.HeaderHex)

	out := make([]byte, 256)
	blockCipher, _ := aes.NewCipher(srvConn.CommonKey)
	ax, _ := eax.NewEax(blockCipher)

	ax.Start(false, nonce, header)
	ax.Update(out, cipherMsg)
	ax.Finish(out[len(cipherMsg):])
	n := len(cipherMsg) - eax.BLOCK_SIZE
	return out[:n]
}

var cli srp6a.Srp6aClient
 // create client object, set id and password of the client, send id to server.
func sendSrp6a1(args map[string]interface{}) []byte{
	method := args["method"]
	if method != "SRP6a" {
		panic("Unknown authenticate method: " + method.(string))
	}
	identity := "alice"
	pass := "password123"
	cli.SetIdentity(identity, pass)
	command := "SRP6a1"
	arg := make(map[string]interface{})
	arg["I"] = identity
	return packCheckCmd(command, arg)
}
 // set param of the client, generate A, compute M1.
 // Send A and M1 to Server.
func sendSrp6a3(args map[string]interface{}) []byte{
	command := "SRP6a3"
	hash := args["hash"].(string)
	NHex := args["N"].(string)
	g := 	args["g"].(int64)
	sHex := args["s"].(string)
	BHex := args["B"].(string)
	s, _ := hex.DecodeString(sHex)
	B, _ := hex.DecodeString(BHex)
	N, _ := hex.DecodeString(NHex)
	cli.NewClient(g, N, len(NHex) *4, hash)
	cli.SetSalt(s)
	cli.SetB(B)
	A := cli.GenerateA()
	cli.ComputeS()
	M1 := cli.ComputeM1()
	A1 := hex.EncodeToString(A)
	M11 := hex.EncodeToString(M1)

	arg := make(map[string]interface{})
	arg["A"] = A1
	arg["M1"] = M11
	return packCheckCmd(command, arg)
}
 // According to srp6a, Compute M2, verify server send M2. Confirm the public key.
func verifySrp6aM2(srvConn *PeerConn, args map[string]interface{}) []byte{
	M2hex := args["M2"].(string)
	M2_mine := cli.ComputeM2()
	M2, _ := hex.DecodeString(M2hex)
	if !bytes.Equal(M2_mine, M2) {
		panic("srp6a M2 not equal")
	}
	srvConn.CommonKey = cli.ComputeK()
	cli = srp6a.Srp6aClient{} // clear client
	return nil
}

// resolve C type data
// Get data body, then decode the data with vbs
// According to command, send the reference message to client.
func  DealCheck(srvConn *PeerConn,reply string) []byte{
	data := getData(srvConn, 0x00, reply)
	c := decodeInCheck(data)
	return c.dealCommand(srvConn, c)
}

//Negotiate Secret key
// Judge the command. If it "SRP6a1"  pack the "SRP6a2" and args, encode them with VBS, then pack header and encode message
// If it is "SRP6a3" compute M1, compare M1 with the M1 that client transfer .
// If them is equal, then compute M2, pack M2 with command
// else negotita fail.
func (outcheck *_InCheck) dealCommand(srvConn *PeerConn, c *_InCheck) []byte{
	cmd := c.cmd
	hexN := "EEAF0AB9ADB38DD69C33F80AFA8FC5E86072618775FF3C0B9EA2314C" +
		"9C256576D674DF7496EA81D3383B4813D692C6E0E0D5D8E250B98BE4" +
		"8E495C1D6089DAD15DC7D7B46154D6B6CE8EF4AD69B15D4982559B29" +
		"7BCF1885C529F566660E57EC68EDBC3C05726CC02FD4CBF4976EAA9A" +
		"FD5138FE8376435B9FC61D2FC0EB06E3"
	//hexN := "ac6bdb41324a9a9bf166de5e1389582faf72b6651987ee07fc3192943db56050" +
		//"a37329cbb4a099ed8193e0757767a13dd52312ab4b03310dcd7f48a9da04fd50" +
		//"e8083969edb767b0cf6095179a163ab3661a05fbd5faaae82918a9962f0b93b8" +
		//"55f97993ec975eeaa80d740adbf4ff747359d041d5c33ea71d281e446b14773b" +
		//"ca97b43a23fb801676bd207a436c6481f1d2b9078717461a5b9d32e688f87748" +
		//"544523b524b0d57d5ea77a2775d2ecfa032cfbdbf52fb3786160279004e57ae6" +
		//"af874e7303ce53299ccc041c7bc308d82a5698f3a8d0c38271ae35f8e9dbfbb6" +
		//"94b5c803d89f7ae435de236d525f54759b65e372fcd68ef20fa7111f9e4aff73"
	N, _ := hex.DecodeString(hexN)
	saltHex := "BEB25379D1A8581EB5A727673A2441EE"
	hashName := "SHA1"
	//bb, _ := hex.DecodeString("E487CB59D31AC550471E81F00F6928E01DDA08E974A004F49E61F5D105284D20")
	// BITS is different, vHex is different fist is 1024, second is 2048
	vHex := "7E273DE8696FFC4F4E337D05B4B375BEB0DDE1569E8FA00A9886D8129BADA1F1822223CA1A605B530E379BA4729FDC59F105B4787E5186F5C671085A1447B52A48CF1970B4FB6F8400BBF4CEBFBB168152E08AB5EA53D15C1AFF87B2B9DA6E04E058AD51CC72BFC9033B564E26480D78E955A5E29E7AB245DB2BE315E2099AFB"
	//different hash name have different v
	//vHex := "400272a61e185e23784e28a16a149dc60a3790fd45856f79a7070c44f7da1ca22f711cd5bc3592171a875c7812472916de2dcfafc22f7dead8f578f1970547936f9eec686bb3df66ff57f724f6b907e83530812b4ffdbf614153e9fbfed4fc6d972da70bb23f6ccd36ad08b72567fe6bcd2bacb713f2cdb9dc8f81f897f489bb393067d66237a3e061902e72096d5ac1cd1d06c1cd648f7e56da5ec6e0094c1b448c5d63ad2addec1e3d9a3aa7118a0410e53434ddbffc60eef5b82548bda5a2f513209484d3221982ca74668a4d37330cc9cfe3b10f0db368293e43026e3a01440ac732bc1cfb983b512d10296f6951ec5e567329af8e58d7c21ea6c778b0bd"
	var g int64 = 2
	idPass := map[string]string {"alice": vHex}
	const BITS = 1024
	var err error
	if cmd == "SRP6a1"{
		id := c.args["I"].(string)
		var verifierHex string
		if _, ok := idPass[id]; ok {
			verifierHex = idPass[id]
		} else {
			panic("Cann't find this user!")
		}
		srvConn.srv.NewServer(g, N, BITS, hashName)
		//salt := srp6a.GenerateSalt()
		//saltHex := hex.EncodeToString(salt)
		//fmt.Println("Salt", saltHex)
		verifier, _ := hex.DecodeString(verifierHex)
		srvConn.srv.SetV(verifier)
		B := srvConn.srv.GenerateB()
		BHex := hex.EncodeToString(B)
		outcheck.cmd = "SRP6a2"
		args := make(map[string]interface{})
		args["hash"] = hashName
		args["s"] = saltHex
		args["B"] = BHex
		args["g"] = g
		args["N"] = hexN
		outcheck.args = args
	} else if cmd == "SRP6a3"{
		A1 := c.args["A"].(string)
		M11 := c.args["M1"].(string)
		A, _ := hex.DecodeString(A1)
		M1, _ := hex.DecodeString(M11)
		srvConn.srv.SetA(A)
		srvConn.srv.ComputeS()
		M1_mine := srvConn.srv.ComputeM1()
		if bytes.Equal(M1, M1_mine) {
			M2 := srvConn.srv.ComputeM2()
			outcheck.cmd = "SRP6a4"
			args := make(map[string]interface{})
			args["M2"] = M2
			outcheck.args = args
			srvConn.CommonKey = srvConn.srv.ComputeK()
			srvConn.srv =  srp6a.Srp6aServer{}
		} else {
			err = fmt.Errorf("Srp6a Error, M1 is different!")
		}
	} else {
		err = fmt.Errorf("XIC.WARNING", "#=client authentication failed")
	}
	if err != nil && bytes.Equal(srvConn.CommonKey, nil) { //SRP6a Error
		outcheck.cmd = "FORBIDDEN"
		args := make(map[string]interface{})
		args["reason"] = err
		outcheck.args = args
		srvConn.srv =  srp6a.Srp6aServer{}
	} else if err != nil { // Expection
		panic(err)
	}
	return packCheckCmd(outcheck.cmd, outcheck.args)
}

// declare B type message
var theByeMessages = _ByeMessage{}
// B type message content
var byeMessageBytes = [8]byte{'X','!','B'}

// return B type message
func (m _ByeMessage) sendBye() []byte {
	return byeMessageBytes[:]
}
// common header
var commonHeaderBytes = [8]byte{'X','!'}
// Active request close
func Close(srvConn *PeerConn) bool {
	flag := false
	var res []byte
	flag = GracefulClose(srvConn)
	if flag == true {
		if len(srvConn.UnDealReplyList) != 0{ // UnDealReplyList is not empty
			return false
		}
		res = theByeMessages.sendBye()
		err := websocket.Message.Send(srvConn.WsConn, res);
		if err != nil {
			fmt.Println("send failed:", err, )
			return false
		}
	}
	return flag
}

//Gracefully close the connection with one client.
// If receiveList is empty, directly send Bye to client
// else deal with request firstly, send Bye to client when receiveList is empty
func GracefulClose(srvConn *PeerConn) bool{
	var res []byte
	var err error
	for _, value := range srvConn.ReceiveList { // receive list
		var txId int64 = value
		data := srvConn.ReceiveDataList[txId]
		fmt.Println("--Undeal request to send ", data)

		var a answer
		a = packAnswerBody(txId)
		res = packAnswer(srvConn, 0x01, txId, a)
		err = websocket.Message.Send(srvConn.WsConn, res)
		time.Sleep(5 * time.Second)
		if err != nil {
			fmt.Println("send failed:", err, )
			return false
		}
	}
	attempTimes := 0
	for len(srvConn.SendList) != 0 {  // send List
		done := startTimer(func(now time.Time) {
			fmt.Println("Waiting for reply that already send" ,now)
		})
		time.Sleep(5 * time.Second)
		close(done)
		attempTimes++
		if attempTimes > len(srvConn.SendList) || len(srvConn.SendList) == 0{
			return true   // Force close
		}
	}
	if len(srvConn.ReceiveList) == 0 && len(srvConn.SendList) == 0 {
		return true
	}
	return false
}
// timer
func startTimer(f func(now time.Time)) chan bool {
	done := make(chan bool, 1)
	go func() {
		t := time.NewTimer(time.Second * 3)
		defer t.Stop()
		select {
		case now := <-t.C:
			f(now)
		case <-done:
			fmt.Println("Waiting for reply data : ")
			return
		}
	}()
	return done
}

// delete TxId from receive List
func deleteTxId(txId int64, dataList map[int]int64) {
	k := 0
	for k <= len(dataList) {
		if txId == dataList[k]{
			delete(dataList, k)
		}
		k++
	}
}