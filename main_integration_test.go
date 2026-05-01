//go:build integration

package main

import (
	"bufio"
	"net"
	"os"
	"testing"
	"time"
)

// Run with: go test -tags integration ./...
//
// Requires the server to be running locally. Set TARGET_HOST, ACK_PORT,
// CHAOS_PORT, and SMART_PORT to override the defaults (localhost:2575/2576/2577).

func targetHost() string {
	if h := os.Getenv("TARGET_HOST"); h != "" {
		return h
	}
	return "localhost"
}


func ackAddr() string   { return net.JoinHostPort(targetHost(), envOr("ACK_PORT", "2575")) }
func chaosAddr() string { return net.JoinHostPort(targetHost(), envOr("CHAOS_PORT", "2576")) }
func smartAddr() string { return net.JoinHostPort(targetHost(), envOr("SMART_PORT", "2577")) }

// --- Integration tests: ACK handler (port 2575) ---

func TestIntegration_AckHandler_ReturnsAA(t *testing.T) {
	resp := sendAndReceive(t, ackAddr(), buildHL7("ADT", "A01"))
	if ack, _, _ := getMSA(resp); ack != "AA" {
		t.Errorf("expected AA, got %q", ack)
	}
}

func TestIntegration_AckHandler_ReturnsAA_ForAnyMsgType(t *testing.T) {
	for _, tc := range []struct{ msgType, trigEvt string }{
		{"ORM", "O01"}, {"ORU", "R01"}, {"SIU", "S12"}, {"MDM", "T01"}, {"MFN", ""},
	} {
		resp := sendAndReceive(t, ackAddr(), buildHL7(tc.msgType, tc.trigEvt))
		if ack, _, _ := getMSA(resp); ack != "AA" {
			t.Errorf("expected AA for %s^%s, got %q", tc.msgType, tc.trigEvt, ack)
		}
	}
}

func TestIntegration_AckHandler_NoERRSegment(t *testing.T) {
	resp := sendAndReceive(t, ackAddr(), buildHL7("ADT", "A01"))
	if _, found := getERR(resp); found {
		t.Error("ackAllHandler should not include ERR segment")
	}
}

// --- Integration tests: chaos handler (port 2576) ---

func TestIntegration_ChaosHandler_ReturnsAR(t *testing.T) {
	resp := sendAndReceive(t, chaosAddr(), buildHL7("ADT", "A01"))
	if ack, _, _ := getMSA(resp); ack != "AR" {
		t.Errorf("expected AR, got %q", ack)
	}
}

func TestIntegration_ChaosHandler_HasERRSegment(t *testing.T) {
	resp := sendAndReceive(t, chaosAddr(), buildHL7("ADT", "A01"))
	if _, found := getERR(resp); !found {
		t.Error("expected ERR segment in chaos response")
	}
}

func TestIntegration_ChaosHandler_ReturnsAR_ForAnyMsgType(t *testing.T) {
	for _, tc := range []struct{ msgType, trigEvt string }{
		{"ORM", "O01"}, {"ORU", "R01"}, {"SIU", "S12"},
	} {
		resp := sendAndReceive(t, chaosAddr(), buildHL7(tc.msgType, tc.trigEvt))
		if ack, _, _ := getMSA(resp); ack != "AR" {
			t.Errorf("expected AR for %s^%s, got %q", tc.msgType, tc.trigEvt, ack)
		}
	}
}

// --- Integration tests: smart handler (port 2577) ---
// Expectations match the default rules.json shipped with the server.

func TestIntegration_SmartHandler_ADT_A01(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("ADT", "A01"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Patient admitted" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_ADT_A02(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("ADT", "A02"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Patient transferred" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_ADT_A03(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("ADT", "A03"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Patient discharged" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_ADT_A04(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("ADT", "A04"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Patient registered" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_ADT_A08(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("ADT", "A08"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Patient information updated" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_ADT_A11(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("ADT", "A11"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Admission cancelled" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_ADT_A13(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("ADT", "A13"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Discharge cancelled" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_ADT_A28(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("ADT", "A28"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Person information added" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_ADT_A31(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("ADT", "A31"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Person information updated" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_ADT_A40(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("ADT", "A40"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Patient merged" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_ADT_GenericFallsToTypeRule(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("ADT", "A99"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "ADT message accepted" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_ORM_O01(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("ORM", "O01"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Order accepted" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_ORM_GenericFallsToTypeRule(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("ORM", "O99"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Order message accepted" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_ORU_R01(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("ORU", "R01"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Observation result received" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_ORU_R30(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("ORU", "R30"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Observation result received (point of care)" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_SIU_S12(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("SIU", "S12"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Appointment booked" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_SIU_S15(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("SIU", "S15"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Appointment cancelled" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_MDM_T01(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("MDM", "T01"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Original document notification accepted" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_MDM_T11(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("MDM", "T11"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Document cancel accepted" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_MFN(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("MFN", "M01"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Master file notification accepted" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_DFT_P03(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("DFT", "P03"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Financial transaction posted" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_BAR_P01(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("BAR", "P01"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Patient account added" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_VXU(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("VXU", "V04"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Vaccination record update accepted" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_UnknownTypeFallsToWildcard(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("ZZZ", "Z01"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Message accepted" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestIntegration_SmartHandler_AA_HasNoERRSegment(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), buildHL7("ADT", "A01"))
	if _, found := getERR(resp); found {
		t.Error("AA response should not contain ERR segment")
	}
}

func TestIntegration_SmartHandler_EchoesControlID(t *testing.T) {
	msg := "MSH|^~\\&|SENDER|FAC|RECV|FAC2|20240101120000||ADT^A01^ADT_A01|MYCTRLID|P|2.5\r"
	resp := sendAndReceive(t, smartAddr(), msg)
	if _, ctrlID, _ := getMSA(resp); ctrlID != "MYCTRLID" {
		t.Errorf("expected control ID %q, got %q", "MYCTRLID", ctrlID)
	}
}

func TestIntegration_SmartHandler_BadMessage_LastDitchError(t *testing.T) {
	resp := sendAndReceive(t, smartAddr(), "EVN|A01|20240101\rPID|||123\r")
	if ack, _, _ := getMSA(resp); ack != "AR" {
		t.Errorf("expected AR from last-ditch error, got %q", ack)
	}
}

func TestIntegration_SmartHandler_MultipleMessagesOnOneConnection(t *testing.T) {
	conn, err := net.DialTimeout("tcp", smartAddr(), 5*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReader(conn)

	send := func(msg string) string {
		if _, err := conn.Write(wrapMLLP([]byte(msg))); err != nil {
			t.Fatalf("write: %v", err)
		}
		raw, err := readMLLP(reader)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		return string(raw)
	}

	cases := []struct {
		msgType, trigEvt, wantText string
	}{
		{"ADT", "A01", "Patient admitted"},
		{"ORM", "O01", "Order accepted"},
		{"ORU", "R01", "Observation result received"},
		{"SIU", "S12", "Appointment booked"},
		{"MDM", "T01", "Original document notification accepted"},
	}
	for _, c := range cases {
		resp := send(buildHL7(c.msgType, c.trigEvt))
		if ack, _, text := getMSA(resp); ack != "AA" || text != c.wantText {
			t.Errorf("%s^%s: expected AA/%q, got %q/%q", c.msgType, c.trigEvt, c.wantText, ack, text)
		}
	}
}
