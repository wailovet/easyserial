package easyserial

import (
	"encoding/json"
	"flag"
	"github.com/jiguorui/crc16"
	"github.com/sigurn/crc8"
	"github.com/tarm/serial"
	"io"
	"net"
	"time"
)

var SerialConfig serial.Config

var TcpToSerialIPAndPort = ""

func init() {
	SerialConfig.Name = "/dev/ttyS0"
	SerialConfig.Baud = 9600
	SerialConfig.ReadTimeout = time.Millisecond * 500

	flag.StringVar(&TcpToSerialIPAndPort, "tcp", "", "")
}

func SendCrc16CheckSum(sendRaw []byte, planLen int) ([]byte, error) {
	return send(sendRaw, crc16CheckSum, planLen)
}

func SendCrc8CheckSum(sendRaw []byte, planLen int) ([]byte, error) {
	return send(sendRaw, crc8CheckSum, planLen)
}
func SendNoneCheckSum(sendRaw []byte, planLen int) ([]byte, error) {
	return send(sendRaw, noneCheckSum, planLen)
}

func SendBccCheckSum(sendRaw []byte, planLen int) ([]byte, error) {
	return send(sendRaw, bccCheckSum, planLen)
}

var EofRemaining = 3

func send(sendRaw []byte, checkSum func(instruction []byte, isAppend bool) []byte, planLen int) ([]byte, error) {
	var s io.ReadWriteCloser
	var err error
	if TcpToSerialIPAndPort != "" {
		s, err = net.Dial("tcp", TcpToSerialIPAndPort)
		if err != nil {
			return nil, err
		}
	} else {
		s, err = serial.OpenPort(&SerialConfig)
		if err != nil {
			return nil, err
		}
	}

	defer func() {
		s.Close()
	}()
	data := checkSum(sendRaw, true)
	//log.Printf("%x", data)
	_, err = s.Write(data)

	if err != nil {
		return nil, err
	}

	if planLen == 0 {
		return nil, nil
	}

	var bufAll []byte
	err = nil
	for {
		var n int
		buf := make([]byte, 128)
		n, err = s.Read(buf)
		if err != nil {
			break
		}
		bufAll = append(bufAll, buf[:n]...)

		if len(bufAll) >= planLen {
			break
		}
	}

	if err == io.EOF && EofRemaining > 0 {
		EofRemaining--
		time.Sleep(time.Second)
		//log.Println("EOF:",EofRemaining)
		bufAll, err = send(sendRaw, checkSum, planLen)
	}
	return bufAll, err
}

func noneCheckSum(instruction []byte, isAppend bool) []byte {
	return instruction
}

func crc16CheckSum(instruction []byte, isAppend bool) []byte {

	if isAppend {
		instruction = append(instruction, []byte{0, 0}...)
	}

	instruction[len(instruction)-2], instruction[len(instruction)-1] = uint16ToBytes(crc16.CheckSum(instruction[:len(instruction)-2]))
	return instruction
}

func crc8CheckSum(instruction []byte, isAppend bool) []byte {

	if isAppend {
		instruction = append(instruction, []byte{0}...)
	}

	table := crc8.MakeTable(crc8.CRC8)
	instruction[len(instruction)-1] = crc8.Checksum(instruction[:len(instruction)-1], table)
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
