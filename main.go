package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

const (
	mllpStart = byte(0x0B)
	mllpEnd1  = byte(0x1C)
	mllpEnd2  = byte(0x0D)
)

// hl7Fields splits an HL7 segment string by "|" and returns 0-indexed slice.
// Index 0 is the segment name, index 1 is MSH.2, index N is MSH.(N+1).
func hl7Fields(segment string) []string {
	return strings.Split(segment, "|")
}

func field(fields []string, i int) string {
	if i < len(fields) {
		return fields[i]
	}
	return ""
}

// triggerEvent extracts MSH.9.2 (e.g. "A01" from "ADT^A01^ADT_A01").
func triggerEvent(msh9 string) string {
	parts := strings.SplitN(msh9, "^", 3)
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func parseMSH(raw string) ([]string, error) {
	lines := strings.FieldsFunc(raw, func(r rune) bool { return r == '\r' || r == '\n' })
	for _, line := range lines {
		if strings.HasPrefix(line, "MSH") {
			return hl7Fields(line), nil
		}
	}
	return nil, fmt.Errorf("no MSH segment found")
}

func timestamp() string {
	return time.Now().Format("20060102150405")
}

func wrapMLLP(msg []byte) []byte {
	var buf bytes.Buffer
	buf.WriteByte(mllpStart)
	buf.Write(msg)
	buf.WriteByte(mllpEnd1)
	buf.WriteByte(mllpEnd2)
	return buf.Bytes()
}

func readMLLP(r *bufio.Reader) ([]byte, error) {
	for {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == mllpStart {
			break
		}
	}

	var buf bytes.Buffer
	for {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == mllpEnd1 {
			next, err := r.ReadByte()
			if err != nil {
				return nil, err
			}
			if next == mllpEnd2 {
				break
			}
			buf.WriteByte(b)
			buf.WriteByte(next)
		} else {
			buf.WriteByte(b)
		}
	}
	return buf.Bytes(), nil
}

func buildACK(msh []string) []byte {
	ctrlID := field(msh, 9)
	procID := field(msh, 10)
	version := field(msh, 11)
	msgType := fmt.Sprintf("ACK^%s^ACK", triggerEvent(field(msh, 8)))

	var sb strings.Builder
	fmt.Fprintf(&sb, "MSH|^~\\&|hl7-pong|hl7-pong|%s|%s|%s||%s|%s|%s|%s\r",
		field(msh, 2), field(msh, 3), timestamp(), msgType, ctrlID, procID, version)
	fmt.Fprintf(&sb, "MSA|AA|%s|Wow, such message, very valid, Wow!\r", ctrlID)
	return wrapMLLP([]byte(sb.String()))
}

func buildNACK(msh []string, errCode int, severity, errMsg string) []byte {
	ctrlID := field(msh, 9)
	procID := field(msh, 10)
	version := field(msh, 11)
	msgType := fmt.Sprintf("ACK^%s^ACK", triggerEvent(field(msh, 8)))

	var sb strings.Builder
	fmt.Fprintf(&sb, "MSH|^~\\&|hl7-pong|hl7-pong|%s|%s|%s||%s|%s|%s|%s\r",
		field(msh, 2), field(msh, 3), timestamp(), msgType, ctrlID, procID, version)
	fmt.Fprintf(&sb, "MSA|AR|%s\r", ctrlID)
	fmt.Fprintf(&sb, "ERR|||%d||%s||%s\r", errCode, severity, errMsg)
	return wrapMLLP([]byte(sb.String()))
}

func buildLastDitchError(exc error) []byte {
	var sb strings.Builder
	fmt.Fprintf(&sb, "MSH|^~\\&|hl7-pong|hl7-pong|||%s||ACK^^ACK|||P|2.5\r", timestamp())
	sb.WriteString("MSA|AR|\r")
	fmt.Fprintf(&sb, "ERR|||207||F||Cannot create valid error resp without a valid request header! %s\r", exc)
	return wrapMLLP([]byte(sb.String()))
}

type handlerFunc func(raw string) []byte

func ackAllHandler(raw string) []byte {
	log.Println("Handling message with AckAllHandler")
	msh, err := parseMSH(raw)
	if err != nil {
		log.Printf("Parse error: %v", err)
		return buildLastDitchError(err)
	}
	resp := buildACK(msh)
	log.Printf("Replying with ACK to ctrl-id=%s", field(msh, 9))
	return resp
}

func chaosHandler(raw string) []byte {
	log.Println("Handling message with ChaosHandler")
	msh, err := parseMSH(raw)
	if err != nil {
		log.Printf("Parse error: %v", err)
		return buildLastDitchError(err)
	}
	return buildNACK(msh, 207, "E", "Unknown error occurred: Eeeevil!")
}

func handleConn(conn net.Conn, handler handlerFunc) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		raw, err := readMLLP(reader)
		if err != nil {
			if err != io.EOF {
				log.Printf("Read error from %s: %v", conn.RemoteAddr(), err)
			}
			return
		}
		resp := handler(string(raw))
		if _, err := conn.Write(resp); err != nil {
			log.Printf("Write error to %s: %v", conn.RemoteAddr(), err)
			return
		}
	}
}

func serve(addr string, handler handlerFunc, wg *sync.WaitGroup) {
	defer wg.Done()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}
	defer ln.Close()
	log.Printf("Listening on %s", addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Accept error on %s: %v", addr, err)
			continue
		}
		go handleConn(conn, handler)
	}
}

func main() {
	_ = godotenv.Load()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("STARTING MLLP PONG SERVER 🏓")
	log.Println("---")

	host := os.Getenv("HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	var wg sync.WaitGroup

	chaosPort := os.Getenv("CHAOS_PORT")
	log.Printf("👹 STARTING CHAOS HANDLER ON PORT %s", chaosPort)
	wg.Add(1)
	go serve(host+":"+chaosPort, chaosHandler, &wg)

	ackPort := os.Getenv("ACK_PORT")
	log.Printf("👍 STARTING ALWAYS ACK SERVER ON PORT %s", ackPort)
	wg.Add(1)
	go serve(host+":"+ackPort, ackAllHandler, &wg)

	wg.Wait()
}
