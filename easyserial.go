package easyserial

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/jiguorui/crc16"
	"github.com/sigurn/crc8"
	"github.com/tarm/serial"
)

var SerialConfig serial.Config

var TcpToSerialIPAndPort = ""

var debug bool

func init() {
	SerialConfig.Name = "/dev/ttyS0"
	SerialConfig.Baud = 9600
	SerialConfig.ReadTimeout = time.Millisecond * 500

	flag.StringVar(&TcpToSerialIPAndPort, "tcp", "", "")

	flag.BoolVar(&debug, "debug", false, "")
}

func SendCrc16CheckSum(sendRaw []byte, planLen int) ([]byte, error) {
	return send(sendRaw, crc16CheckSum, planLen)
}

func SendCrcCcittCheckSum(sendRaw []byte, planLen int) ([]byte, error) {
	return send(sendRaw, crcCCITTCheckSum, planLen)
}

func SendCrc8CheckSum(sendRaw []byte, planLen int) ([]byte, error) {
	return send(sendRaw, crc8CheckSum, planLen)
}

func SendAccumulateCheckSum(sendRaw []byte, planLen int) ([]byte, error) {
	return send(sendRaw, accumulateCheckSum, planLen)
}
func SendNoneCheckSum(sendRaw []byte, planLen int) ([]byte, error) {
	return send(sendRaw, noneCheckSum, planLen)
}

func SendBccCheckSum(sendRaw []byte, planLen int) ([]byte, error) {
	return send(sendRaw, bccCheckSum, planLen)
}

var EofRemaining = 3

var SendHook func(sendRaw []byte, planLen int) ([]byte, error)

func send(sendRaw []byte, checkSum func(instruction []byte, isAppend bool) []byte, planLen int) ([]byte, error) {
	if SendHook != nil {
		data := checkSum(sendRaw, true)
		return SendHook(data, planLen)
	}

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

	DebugLog("Write data:%# x\n", data)

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

		DebugLog("==> Read bytes:%# x\n", buf, n, err)

		if err != nil && err != io.EOF {
			break
		}
		bufAll = append(bufAll, buf[:n]...)

		if len(bufAll) >= planLen {
			if err == io.EOF {
				err = nil
			}
			break
		}
	}

	//if err == io.EOF && EofRemaining > 0 {
	//	EofRemaining--
	//	time.Sleep(time.Second)
	//	//log.Println("EOF:",EofRemaining)
	//	bufAll, err = send(sendRaw, checkSum, planLen)
	//}

	DebugLog("Read data:%# x\n", bufAll)
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

func crcCCITTCheckSum(instruction []byte, isAppend bool) []byte {

	if isAppend {
		instruction = append(instruction, []byte{0, 0}...)
	}

	instruction[len(instruction)-1], instruction[len(instruction)-2] = uint16ToBytes(CrcCcitt(instruction[:len(instruction)-2]))
	return instruction
}

func CheckCrc16CheckSum(data []byte) bool {
	l := len(data)
	a, b := uint16ToBytes(crc16.CheckSum(data[:l-2]))
	return a == data[l-2] && b == data[l-1]
}

func crc8CheckSum(instruction []byte, isAppend bool) []byte {

	if isAppend {
		instruction = append(instruction, []byte{0}...)
	}

	table := crc8.MakeTable(crc8.CRC8)
	instruction[len(instruction)-1] = crc8.Checksum(instruction[:len(instruction)-1], table)
	return instruction
}

func accumulateCheckSum(instruction []byte, isAppend bool) []byte {

	if isAppend {
		instruction = append(instruction, []byte{0}...)
	}

	var c uint8
	for _, v := range instruction {
		c += v
	}

	instruction[len(instruction)-1] = c
	return instruction
}

func CheckCrc8CheckSum(data []byte) bool {
	l := len(data)
	table := crc8.MakeTable(crc8.CRC8)
	c := crc8.Checksum(data[:l-1], table)
	return c == data[l-1]
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

func ByteToNum(b ...byte) int {
	bl := len(b)
	if bl < 1 {
		return 0
	}
	if bl == 1 {
		return int(b[0])
	}

	return ByteToNum(b[0:bl-1]...)*0xFF + int(b[bl-1])

}

func CheckTimeOut() {
	go func() {
		time.Sleep(time.Second * 10)
		fmt.Println(DisplayToString("timeout"))
		os.Exit(0)
	}()
}

func CheckDeviceBusy(isWait bool) func() {
	tcp, merr := net.Listen("tcp", "127.0.0.1:53485")
	if isWait {
		for merr != nil {
			time.Sleep(time.Second)
			tcp, merr = net.Listen("tcp", "127.0.0.1:53485")
		}
	} else {
		if merr != nil {
			DeviceBusy()
		}
	}
	return func() {
		tcp.Close()
	}
}

func DeviceBusy() {
	fmt.Println(DisplayToString("busy"))
	os.Exit(0)
}

func CrcCcitt(data []byte) (crc uint16) {

	crc = 0x0
	crcTable := [256]uint16{
		0x0000, 0x1021, 0x2042, 0x3063, 0x4084, 0x50A5, 0x60C6, 0x70E7,
		0x8108, 0x9129, 0xA14A, 0xB16B, 0xC18C, 0xD1AD, 0xE1CE, 0xF1EF,
		0x1231, 0x0210, 0x3273, 0x2252, 0x52B5, 0x4294, 0x72F7, 0x62D6,
		0x9339, 0x8318, 0xB37B, 0xA35A, 0xD3BD, 0xC39C, 0xF3FF, 0xE3DE,
		0x2462, 0x3443, 0x0420, 0x1401, 0x64E6, 0x74C7, 0x44A4, 0x5485,
		0xA56A, 0xB54B, 0x8528, 0x9509, 0xE5EE, 0xF5CF, 0xC5AC, 0xD58D,
		0x3653, 0x2672, 0x1611, 0x0630, 0x76D7, 0x66F6, 0x5695, 0x46B4,
		0xB75B, 0xA77A, 0x9719, 0x8738, 0xF7DF, 0xE7FE, 0xD79D, 0xC7BC,
		0x48C4, 0x58E5, 0x6886, 0x78A7, 0x0840, 0x1861, 0x2802, 0x3823,
		0xC9CC, 0xD9ED, 0xE98E, 0xF9AF, 0x8948, 0x9969, 0xA90A, 0xB92B,
		0x5AF5, 0x4AD4, 0x7AB7, 0x6A96, 0x1A71, 0x0A50, 0x3A33, 0x2A12,
		0xDBFD, 0xCBDC, 0xFBBF, 0xEB9E, 0x9B79, 0x8B58, 0xBB3B, 0xAB1A,
		0x6CA6, 0x7C87, 0x4CE4, 0x5CC5, 0x2C22, 0x3C03, 0x0C60, 0x1C41,
		0xEDAE, 0xFD8F, 0xCDEC, 0xDDCD, 0xAD2A, 0xBD0B, 0x8D68, 0x9D49,
		0x7E97, 0x6EB6, 0x5ED5, 0x4EF4, 0x3E13, 0x2E32, 0x1E51, 0x0E70,
		0xFF9F, 0xEFBE, 0xDFDD, 0xCFFC, 0xBF1B, 0xAF3A, 0x9F59, 0x8F78,
		0x9188, 0x81A9, 0xB1CA, 0xA1EB, 0xD10C, 0xC12D, 0xF14E, 0xE16F,
		0x1080, 0x00A1, 0x30C2, 0x20E3, 0x5004, 0x4025, 0x7046, 0x6067,
		0x83B9, 0x9398, 0xA3FB, 0xB3DA, 0xC33D, 0xD31C, 0xE37F, 0xF35E,
		0x02B1, 0x1290, 0x22F3, 0x32D2, 0x4235, 0x5214, 0x6277, 0x7256,
		0xB5EA, 0xA5CB, 0x95A8, 0x8589, 0xF56E, 0xE54F, 0xD52C, 0xC50D,
		0x34E2, 0x24C3, 0x14A0, 0x0481, 0x7466, 0x6447, 0x5424, 0x4405,
		0xA7DB, 0xB7FA, 0x8799, 0x97B8, 0xE75F, 0xF77E, 0xC71D, 0xD73C,
		0x26D3, 0x36F2, 0x0691, 0x16B0, 0x6657, 0x7676, 0x4615, 0x5634,
		0xD94C, 0xC96D, 0xF90E, 0xE92F, 0x99C8, 0x89E9, 0xB98A, 0xA9AB,
		0x5844, 0x4865, 0x7806, 0x6827, 0x18C0, 0x08E1, 0x3882, 0x28A3,
		0xCB7D, 0xDB5C, 0xEB3F, 0xFB1E, 0x8BF9, 0x9BD8, 0xABBB, 0xBB9A,
		0x4A75, 0x5A54, 0x6A37, 0x7A16, 0x0AF1, 0x1AD0, 0x2AB3, 0x3A92,
		0xFD2E, 0xED0F, 0xDD6C, 0xCD4D, 0xBDAA, 0xAD8B, 0x9DE8, 0x8DC9,
		0x7C26, 0x6C07, 0x5C64, 0x4C45, 0x3CA2, 0x2C83, 0x1CE0, 0x0CC1,
		0xEF1F, 0xFF3E, 0xCF5D, 0xDF7C, 0xAF9B, 0xBFBA, 0x8FD9, 0x9FF8,
		0x6E17, 0x7E36, 0x4E55, 0x5E74, 0x2E93, 0x3EB2, 0x0ED1, 0x1EF0,
	}

	var bs = data
	for i := 0; i < len(data); i++ {
		crc = crcTable[(uint16(bs[i])^(crc>>8))&0xff] ^ (crc << 8)
		//log.Printf("%# x",crc)
	}

	//log.Printf("%# x", bs)
	crc = ^crc
	//log.Printf("%# x", crc)
	return
}

func DebugLog(s string, v ...interface{}) {
	if debug {
		fmt.Printf(s, v)
	}
}
