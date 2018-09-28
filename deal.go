package main

import (
	"github.com/mangos/vbs"
	"github.com/mangos/srp6a"
	"log"
	"fmt"
	"reflect"
	"bytes"
	"encoding/hex"
	"crypto/aes"
	"github.com/mangos/eax"
	"golang.org/x/net/websocket"
)

const (
	send_add_state int64 = 2
	maxReceveNum = 1024
)

var (
	send_nonce int64 = 1
	key []byte
	nonceHex string = "22E7ADD93CFC6393C57EC0B3C17D6B44"
	headerHex string = "126735FCC320D25A"
	receiveList =  make(map[int]int64)
	receiveDataList = make(map[int64][]byte)  // temp receive data
)

func buildHeader(header []byte) messageHeader{
	var head messageHeader
	head.Magic = header[0]
	head.Version = header[1]
	head.Type = header[2]
	head.Flags = header[3]
	head.BodySize = int32(header[4])*256*256*256 + int32(header[5])*256*256 + int32(header[6])*256 +int32(header[7])
	return head
}
// check header whether is qualified
func checkHeader(header messageHeader) error {
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
// pack the header
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
// VBS Nonce
func packNonce() []byte{
	b := &bytes.Buffer{}
	enc := vbs.NewEncoder(b)
	err := enc.Encode(send_nonce)
	if err != nil {
		panic("vbs.Encoder error when encode send_nonce")
	}
	send_nonce += send_add_state  // nonce increase
	return b.Bytes()
}

// return Q type message
func (q *_InQuest) Type() MsgType {
	return 'Q'
}
// resolve the request data from client that type is Q
// Return ANSWER type message
func dealRequest(reply string) []byte{
	_isEnc := reply[3]  // encrypt flag
	request, errAnswer := resolveRequest(_isEnc, reply) //resovle request

	txid := request.txid
	if txid == 0 {  // txid is 0, server is no response
		return nil
	}
	if request.repeatFlag {   // repeat data, direct pack
		return packAnswer(_isEnc, txid, errAnswer)
	}

	var answer answer
	answer = packAnswerBody(txid)
	return packAnswer(_isEnc, txid, answer)
}

func packAnswer(_isEnc uint8,txid int64, answer answer) []byte{
	var result []byte
	content,size := newOutAnswer(txid, answer.status,answer.args) //construct a ANSWER type message
	packet := fillHeader(size,'A')// fill header message
	noncePack := packNonce()
	if _isEnc == 0x01 {
		result = make([]byte, size+32)  //encrypt data send to client
		// nonce+header+msg(msg+MAC)
		copy(result[:8],noncePack)
		copy(result[8:16],packet)
		result[11] = 0x01
		msg := encrypt(content.buf)
		copy(result[16:], msg)
	} else {
		result = make([]byte, size+8, 2*size)  // data send to client
		copy(result[:8],packet)
		copy(result[8:],content.buf)
	}
	//deleteTxid(txid)
	return result
}
// delete Txid from receive List
func deleteTxid(txid int64) {
	key := 0
	for key <= len(receiveList) {
		if txid == receiveList[key]{
			delete(receiveList, key)
		}
		key++
	}
}

// resolve Q type message from client
func resolveRequest(isEnc uint8, reply string) (_InQuest, answer){
	len1 := int(reply[4])*256*256*256 + int(reply[5])*256*256 + int(reply[6])*256 +int(reply[7])
	var data []byte
	if isEnc == 0x01 {
		data = []byte(reply[8:len1+24]) // meassage + MAC
		data = decrypt(data)
		if len(data) != len1 {
			log.Fatalln("Data length not equal to bodysize")
		}
	} else {
		data = []byte(reply[8:len1+8])
		if len(data) != len(reply)-8 {
			log.Fatalln("Data length not equal to bodysize")
		}
	}

	var request _InQuest
	var errAnswer answer
	content := decodeData(5,data)//5代表5数组长度，解析VBS字符串的数据成数组，
	//request param
	request.txid = content[0].(int64)
	request.service = content[1].(string)
	request.method = content[2].(string)
	request.ctx = content[3].(map[string]interface {})
	request.args = content[4].(map[string]interface {})
	txid := request.txid

	// judge whether is already receive the data
	for _, value := range receiveDataList {
		if bytes.Equal(data[1:], value){  // Remove txid
			errAnswer.status = 1
			msg := "message is duplication"
			arg := packExpArg("Receive duplication of data",1000,"218",msg,"resolveRequest*service","Receive")
			errAnswer.args =  arg
			request.repeatFlag = true
			return request, errAnswer
		}
	}
	if txid != 0 {
		receiveList[len(receiveList)] =  txid   // Receive List
	}
	// record receive data
	receiveDataList[txid] = data[1:]  // Remove txid
	return request, errAnswer
}

func packAnswerBody(txid int64) answer {
	var answer answer
	var err error
	arg  := make(map[string]interface{})
	// construct the data return to client
	answer.txid = txid
	if err != nil {  // exception
		answer.status = 1
		arg = packExpArg("",1001,"","","","")
	} else {  // normal
		answer.status = 0
		arg["first"] = "this is server reply"
		arg["second"] = "this is Tim server"
	}
	answer.args =  arg
	return answer
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
// decode VBS data, return a array
func decodeData(n int,data []byte) []interface{}{
	var content []interface{}
	var err error
	for i := 0; i < n ;i++{
		var tmp interface{}
		data, err = vbs.UnmarshalOneItem(data, &tmp)
		if err != nil  {
			log.Fatalln("error decoding %T: %v:", data, err)
		}
		content = append(content,tmp.(interface{}))
	}
	return content
}
// encrypt data with EAX
func encrypt(msg []byte) []byte{
	nonce, _ := hex.DecodeString(nonceHex)
	header, _ := hex.DecodeString(headerHex)

	out := make([]byte, 256)
	blockCipher, _ := aes.NewCipher(key)
	ax, _ := eax.NewEax(blockCipher)

	ax.Start(true, nonce, header)
	ax.Update(out, msg)
	ax.Finish(out[len(msg):])
	n := len(msg) + eax.BLOCK_SIZE
	return out[:n]
}
// decrypt data with EAX
func decrypt(cipherMsg []byte) ([]byte){
	nonce, _ := hex.DecodeString(nonceHex)
	header, _ := hex.DecodeString(headerHex)

	out := make([]byte, 256)
	blockCipher, _ := aes.NewCipher(key)
	ax, _ := eax.NewEax(blockCipher)

	ax.Start(false, nonce, header)
	ax.Update(out, cipherMsg)
	ax.Finish(out[len(cipherMsg):])
	n := len(cipherMsg) - eax.BLOCK_SIZE
	return out[:n]
}

// response the request
func sendAnswer(res interface{})(rest []byte){
	//VBS encode
	buf, err := vbs.Marshal(res)
	if err != nil {
		fmt.Println("error encoding %v:", err)
	}
	fmt.Println("Marshal success: ", reflect.TypeOf(buf), buf)
	return buf
}

// resolve C type data
func _InCheck(reply string) []byte{
	//var res []byte
	var incheck check
	lenth := int(reply[4])*256*256*256 + int(reply[5])*256*256 + int(reply[6])*256 +int(reply[7])
	data := []byte(reply[8:lenth + 8])
	if len(data) != len(reply) -8{
		log.Fatalln("Data length not equal to bodysize")
	}
	//incheck
	content := decodeData(2,data)
	incheck.command = content[0].(string)
	incheck.args =  content[1].(map[string]interface{})
	return _DealCommand(incheck)
}


// return a Q type data
func newInQuest(buf []byte) *_InQuest {
	q := &_InQuest{}
	dec := vbs.NewDecoderBytes(buf)
	dec.Decode(&q.txid)
	dec.Decode(&q.service)
	dec.Decode(&q.method)
	dec.Decode(&q.ctx)
	q.buf = buf
	return q
}

var srv srp6a.Srp6aServer
//Negotiate Secret key
func _DealCommand(incheck check) []byte{
	cmd := incheck.command
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
	// BITS is different, vHex is different fist is 1024, second is 2048
	vHex := "7E273DE8696FFC4F4E337D05B4B375BEB0DDE1569E8FA00A9886D8129BADA1F1822223CA1A605B530E379BA4729FDC59F105B4787E5186F5C671085A1447B52A48CF1970B4FB6F8400BBF4CEBFBB168152E08AB5EA53D15C1AFF87B2B9DA6E04E058AD51CC72BFC9033B564E26480D78E955A5E29E7AB245DB2BE315E2099AFB"
	//vHex := "400272a61e185e23784e28a16a149dc60a3790fd45856f79a7070c44f7da1ca22f711cd5bc3592171a875c7812472916de2dcfafc22f7dead8f578f1970547936f9eec686bb3df66ff57f724f6b907e83530812b4ffdbf614153e9fbfed4fc6d972da70bb23f6ccd36ad08b72567fe6bcd2bacb713f2cdb9dc8f81f897f489bb393067d66237a3e061902e72096d5ac1cd1d06c1cd648f7e56da5ec6e0094c1b448c5d63ad2addec1e3d9a3aa7118a0410e53434ddbffc60eef5b82548bda5a2f513209484d3221982ca74668a4d37330cc9cfe3b10f0db368293e43026e3a01440ac732bc1cfb983b512d10296f6951ec5e567329af8e58d7c21ea6c778b0bd"
	g := 2
	idPass := map[string]string {"alice": vHex}
	const BITS = 1024
	var outcheck check
	var err error
	if cmd == "SRP6a1"{
		id := incheck.args["I"].(string)
		var verifierHex string
		if _, ok := idPass[id]; ok {
			verifierHex = idPass[id]
		} else {
			panic("Cann't find this user!")
		}
		srv.SetHash(hashName)
		srv.SetParameter(g, N, BITS)
		verifier, _ := hex.DecodeString(verifierHex)
		srv.SetV(verifier)
		B := srv.GenerateB()
		BHex := hex.EncodeToString(B)
		outcheck.command = "SRP6a2"
		args := make(map[string]interface{})
		args["hash"] = hashName
		args["s"] = saltHex
		args["B"] = BHex
		args["g"] = g
		args["N"] = hexN
		outcheck.args = args
	} else if cmd == "SRP6a3"{
		A1 := incheck.args["A"].(string)
		M11 := incheck.args["M1"].(string)
		A, _ := hex.DecodeString(A1)
		M1, _ := hex.DecodeString(M11)

		srv.SetA(A)
		srv.ComputeS()
		M1_mine := srv.ComputeM1()
		if bytes.Equal(M1, M1_mine) {
			M2 := srv.ComputeM2()
			outcheck.command = "SRP6a4"
			args := make(map[string]interface{})
			args["M2"] = M2
			outcheck.args = args
			key = srv.ComputeK()
		} else {
			err = fmt.Errorf("Srp6a Error, M1 is different!")
		}
	} else {
		err = fmt.Errorf("XIC.WARNING", "#=client authentication failed")
	}
	if err != nil {
		outcheck.command = "FORBIDDEN"
		args := make(map[string]interface{})
		args["reason"] = err
		outcheck.args = args
	}

	b := &bytes.Buffer{}
	enc := vbs.NewEncoder(b)
	enc.Encode(outcheck.command)
	err = enc.Encode(outcheck.args)
	if err != nil {
		panic("vbs.Encoder error")
	}

	result := make([]byte, enc.Size()+8, (enc.Size())*2) // data to send to client
	packet := fillHeader(enc.Size(),'C')
	if outcheck.command == "SRP6a4" {  // encrypt
		packet[4] = 0x01
	}
	copy(result[:8],packet)
	copy(result[8:],b.Bytes())
	return result
}

// SRP6a consult the common key, return C type message data
func _OutCheck() []byte{
	var outcheck check
	outcheck.command = "AUTHENTICATE"
	args := make(map[string]interface{})
	args["method"] = "SRP6a"
	outcheck.args = args

	//vbs encode check data
	a := &check{}
	b := &bytes.Buffer{}
	enc := vbs.NewEncoder(b)
	a.reserved = b.Len()
	enc.Encode(outcheck.command)
	err := enc.Encode(outcheck.args)
	if err != nil {
		panic("vbs.Encoder error")
	}
	a.buf = b.Bytes()

	result := make([]byte, enc.Size()+8, (enc.Size())*2) // data to send to client
	packet := fillHeader(enc.Size(),'C')
	copy(result[:8],packet)
	copy(result[8:],a.buf)
	return result

}

// declare H type message
var theHelloMessages = _HelloMessage{}

// return H type
func (m _HelloMessage) Type() MsgType {
	return 'H'
}
//H type content
var helloMessageBytes = [8]byte{'X','!','H'}

// return H type message
func (m _HelloMessage) sendHello() []byte {
	return helloMessageBytes[:]
}



// declare B type message
var theByeMessages = _ByeMessage{}

// return B type
func (m _ByeMessage) Type() MsgType {
	return 'B'
}

// B type message content
var byeMessageBytes = [8]byte{'X','!','B'}

// return B type message
func (m _ByeMessage) sendBye() []byte {
	return byeMessageBytes[:]
}


// encode A type message
func newOutAnswer(id int64,status int64, args interface{}) (*_OutAnswer,int){
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
// common header
var commonHeaderBytes = [8]byte{'X','!'}

//Gracefully close the connection with one client.
// If receiveList is empty, directly send Bye to client
// else deal with request firstly, send Bye to client when receiveList is empty
func gracefulClose(ws *websocket.Conn) bool{
	var res []byte
	var err error

	for len(receiveList) > 0 {
		var txid int64 = receiveList[0]
		data := receiveDataList[txid]

		fmt.Println("--Undeal request to send ", data)
		var answer answer
		answer = packAnswerBody(txid)
		res = packAnswer(0x01, txid, answer)
		err = websocket.Message.Send(ws, res);
		if err != nil {
			fmt.Println("send failed:", err, )
			return false
		}
		deleteTxid(txid)
	}
	res = theByeMessages.sendBye()
	err = websocket.Message.Send(ws, res);
	if err != nil {
		fmt.Println("send failed:", err, )
		return false
	}
	return true
}