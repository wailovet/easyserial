package easyserial

import (
	"encoding/json"
	"github.com/jiguorui/crc16"
	"github.com/tarm/serial"
	"time"
)

var SerialConfig serial.Config

func init() {
	SerialConfig.Name = "/dev/ttyS0"
	SerialConfig.Baud = 9600
	SerialConfig.ReadTimeout = time.Millisecond * 500

}
func SendCrc16CheckSum(sendRaw []byte, planLen int) ([]byte, error) {
	return send(sendRaw, crc16CheckSum, planLen)
}

func SendBccCheckSum(sendRaw []byte, planLen int) ([]byte, error) {
	return send(sendRaw, bccCheckSum, planLen)
}

func send(sendRaw []byte, checkSum func(instruction []byte, isAppend bool) []byte, planLen int) ([]byte, error) {
	c := &SerialConfig
	s, err := serial.OpenPort(c)
	if err != nil {
		return nil, err
	}
	defer func() {
		s.Close()
	}()
	data := checkSum(sendRaw, true)
	_, err = s.Write(data)

	//log.Printf("%x",data)

	if err != nil {
		return nil, err
	}

	if planLen == 0 {
		return nil, nil
	}

	var bufAll []byte
	err = nil
	for {
		buf := make([]byte, 128)
		n, err := s.Read(buf)
		if err != nil {
			break
		}
		bufAll = append(bufAll, buf[:n]...)

		if len(bufAll) >= planLen {
			break
		}
	}
	return bufAll, err
}

func crc16CheckSum(instruction []byte, isAppend bool) []byte {

	if isAppend {
		instruction = append(instruction, []byte{0, 0}...)
	}

	instruction[len(instruction)-2], instruction[len(instruction)-1] = uint16ToBytes(crc16.CheckSum(instruction[:len(instruction)-2]))
	return instruction
}
func bccCheckSum(instruction []byte, isAppend bool) []byte {

	if isAppend {
		instruction = append(instruction, []byte{0}...)
	}
	tmp := instruction[:len(instruction)-1]
	var result byte = 0x00
	for e := range tmp {
		result ^= tmp[e]
	}
	instruction[len(instruction)-1] = result
	return instruction
}

func uint16ToBytes(n uint16) (byte, byte) {
	return byte(n), byte(n >> 8)
}

func DisplayToString(r interface{}) string {
	result, _ := json.Marshal(r)
	return string(result)
}
