package eax_test

import (
	"testing"
	"fmt"
	"crypto/aes"
	"encoding/hex"
	//"bytes"
	"strings"

	"github.com/mangos/eax"
)

func TestEax(t *testing.T) {
	keyHex	  := "8395FCF1E95BEBD697BD010BC766AAC3"
        nonceHex  := "22E7ADD93CFC6393C57EC0B3C17D6B44"
	headerHex := "126735FCC320D25A"
        msgHex    := "CA40D7446E545FFAED3BD12A740A659FFBBB3CEAB7"
		msgHex2      := "91945D3F4DCBEE0BF45EF52255F095A4"
	//cipherHex := "CB8920F87A6C75CFF39627B56E3ED197C552D295A7CFC46AFC253B4652B1AF3795B124AB6E"
	key, _ := hex.DecodeString(keyHex)
	nonce, _ := hex.DecodeString(nonceHex)
	header, _ := hex.DecodeString(headerHex)
	msg, _ := hex.DecodeString(msgHex)
	msg2, _ := hex.DecodeString(msgHex2)
	//cipher, _ := hex.DecodeString(cipherHex)

	out := make([]byte, 128)
	blockCipher, _ := aes.NewCipher(key)
	ax, _ := eax.NewEax(blockCipher)

	ax.Start(true, nonce, header)
	ax.Update(out, msg)
	ax.Update(out[len(msg):], msg2)
	ax.Finish(out[len(msg)+len(msg2):])
	n := len(msg)+len(msg2) + eax.BLOCK_SIZE
	fmt.Println("C2", strings.ToUpper(hex.EncodeToString(out[:n])))
	//if !bytes.Equal(out[:n], cipher) {
	//	fmt.Println("C1", cipherHex)
	//	fmt.Println("C2", strings.ToUpper(hex.EncodeToString(out[:n])))
	//	t.Errorf("Test failed")
	//}
	fmt.Println("out[:n]", n, out[:n])
	testDecrypt(out[:n])
}
func testDecrypt(encryptMsg []byte) {
	fmt.Println("encryptMsg", encryptMsg)
	keyHex	  := "8395FCF1E95BEBD697BD010BC766AAC3"
	nonceHex  := "22E7ADD93CFC6393C57EC0B3C17D6B44"
	headerHex := "126735FCC320D25A"
	key, _ := hex.DecodeString(keyHex)
	nonce, _ := hex.DecodeString(nonceHex)
	header, _ := hex.DecodeString(headerHex)

	out := make([]byte, 128)
	blockCipher, _ := aes.NewCipher(key)
	ax, _ := eax.NewEax(blockCipher)

	ax.Start(false, nonce, header)
	ax.Update(out, encryptMsg)
	ax.Finish(out[len(encryptMsg):])
	n := len(encryptMsg) - eax.BLOCK_SIZE
	fmt.Println("C2", n, strings.ToUpper(hex.EncodeToString(out[:n])))
}


