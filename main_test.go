package main

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- Test helpers ---

func startTestServer(t *testing.T, handler handlerFunc) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleConn(conn, handler)
		}
	}()
	return ln.Addr().String()
}

func sendAndReceive(t *testing.T, addr, msg string) string {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("connect to %s: %v", addr, err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.Write(wrapMLLP([]byte(msg))); err != nil {
		t.Fatalf("write: %v", err)
	}
	raw, err := readMLLP(bufio.NewReader(conn))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(raw)
}

func getMSA(resp string) (ackCode, ctrlID, text string) {
	for _, line := range strings.FieldsFunc(resp, func(r rune) bool { return r == '\r' || r == '\n' }) {
		if strings.HasPrefix(line, "MSA") {
			f := hl7Fields(line)
			return field(f, 1), field(f, 2), field(f, 3)
		}
	}
	return "", "", ""
}

func getERR(resp string) (errCode string, found bool) {
	for _, line := range strings.FieldsFunc(resp, func(r rune) bool { return r == '\r' || r == '\n' }) {
		if strings.HasPrefix(line, "ERR") {
			f := hl7Fields(line)
			return field(f, 3), true
		}
	}
	return "", false
}

func buildHL7(msgType, trigEvt string) string {
	now := time.Now().Format("20060102150405")
	var msh9 string
	if trigEvt != "" {
		msh9 = msgType + "^" + trigEvt + "^" + msgType + "_" + trigEvt
	} else {
		msh9 = msgType
	}
	return "MSH|^~\\&|SENDER|SENDFAC|RECEIVER|RECVFAC|" + now + "||" + msh9 + "|CTRL001|P|2.5\r"
}

func makeSmartHandler(rules []Rule) handlerFunc {
	return smartHandler(&RulesConfig{Rules: rules})
}

// --- Unit tests: messageType ---

func TestMessageType(t *testing.T) {
	cases := []struct{ input, want string }{
		{"ADT^A01^ADT_A01", "ADT"},
		{"ORM^O01^ORM_O01", "ORM"},
		{"ADT", "ADT"},
		{"", ""},
	}
	for _, c := range cases {
		if got := messageType(c.input); got != c.want {
			t.Errorf("messageType(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// --- Unit tests: triggerEvent ---

func TestTriggerEvent(t *testing.T) {
	cases := []struct{ input, want string }{
		{"ADT^A01^ADT_A01", "A01"},
		{"ORM^O01", "O01"},
		{"ADT", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := triggerEvent(c.input); got != c.want {
			t.Errorf("triggerEvent(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// --- Unit tests: matchRule ---

func TestMatchRule_ExactMatch(t *testing.T) {
	rules := []Rule{
		{Match: "ADT^A01", Response: "AA"},
		{Match: "ADT", Response: "AE"},
		{Match: "*", Response: "AR"},
	}
	r := matchRule(rules, "ADT", "A01")
	if r == nil || r.Response != "AA" {
		t.Errorf("expected exact match AA, got %v", r)
	}
}

func TestMatchRule_MessageTypeOnly(t *testing.T) {
	rules := []Rule{
		{Match: "ADT^A08", Response: "AE"},
		{Match: "ADT", Response: "AA"},
		{Match: "*", Response: "AR"},
	}
	r := matchRule(rules, "ADT", "A01")
	if r == nil || r.Response != "AA" {
		t.Errorf("expected message-type-only match AA, got %v", r)
	}
}

func TestMatchRule_Wildcard(t *testing.T) {
	rules := []Rule{
		{Match: "ORM", Response: "AE"},
		{Match: "*", Response: "AA"},
	}
	r := matchRule(rules, "ORU", "R01")
	if r == nil || r.Response != "AA" {
		t.Errorf("expected wildcard match AA, got %v", r)
	}
}

func TestMatchRule_PriorityExactOverType(t *testing.T) {
	rules := []Rule{
		{Match: "ADT", Response: "AE"},
		{Match: "ADT^A01", Response: "AA"},
		{Match: "*", Response: "AR"},
	}
	r := matchRule(rules, "ADT", "A01")
	if r == nil || r.Response != "AA" {
		t.Errorf("exact match should beat type match, got %v", r)
	}
}

func TestMatchRule_PriorityTypeOverWildcard(t *testing.T) {
	rules := []Rule{
		{Match: "*", Response: "AR"},
		{Match: "ADT", Response: "AA"},
	}
	r := matchRule(rules, "ADT", "A99")
	if r == nil || r.Response != "AA" {
		t.Errorf("type match should beat wildcard, got %v", r)
	}
}

func TestMatchRule_NoMatch(t *testing.T) {
	rules := []Rule{{Match: "ORM", Response: "AE"}}
	if r := matchRule(rules, "ADT", "A01"); r != nil {
		t.Errorf("expected nil, got %v", r)
	}
}

func TestMatchRule_CaseInsensitive(t *testing.T) {
	rules := []Rule{{Match: "adt^a01", Response: "AA"}}
	r := matchRule(rules, "ADT", "A01")
	if r == nil || r.Response != "AA" {
		t.Errorf("expected case-insensitive match, got %v", r)
	}
}

func TestMatchRule_EmptyRules(t *testing.T) {
	if r := matchRule(nil, "ADT", "A01"); r != nil {
		t.Errorf("expected nil for empty rules, got %v", r)
	}
}

func TestMatchRule_EmptyTriggerEvent(t *testing.T) {
	rules := []Rule{{Match: "ADT", Response: "AA"}}
	r := matchRule(rules, "ADT", "")
	if r == nil || r.Response != "AA" {
		t.Errorf("expected ADT rule match, got %v", r)
	}
}

// --- Unit tests: loadRules ---

func TestLoadRules_ValidJSON(t *testing.T) {
	cfg := RulesConfig{Rules: []Rule{{Match: "ADT^A01", Response: "AA", AckText: "ok"}}}
	data, _ := json.Marshal(cfg)
	f := filepath.Join(t.TempDir(), "rules.json")
	os.WriteFile(f, data, 0644)

	loaded, err := loadRules(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(loaded.Rules) != 1 || loaded.Rules[0].Match != "ADT^A01" {
		t.Errorf("unexpected rules: %+v", loaded.Rules)
	}
}

func TestLoadRules_AllFields(t *testing.T) {
	raw := `{"rules":[{"match":"ADT^A01","response":"AE","error_code":100,"error_severity":"W","error_msg":"oops","delay_ms":50,"nack_rate":0.5,"ack_text":"hi"}]}`
	f := filepath.Join(t.TempDir(), "rules.json")
	os.WriteFile(f, []byte(raw), 0644)

	loaded, err := loadRules(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := loaded.Rules[0]
	if r.Response != "AE" || r.ErrorCode != 100 || r.ErrorSeverity != "W" ||
		r.ErrorMsg != "oops" || r.DelayMs != 50 || r.NackRate != 0.5 || r.AckText != "hi" {
		t.Errorf("field mismatch: %+v", r)
	}
}

func TestLoadRules_InvalidJSON(t *testing.T) {
	f := filepath.Join(t.TempDir(), "rules.json")
	os.WriteFile(f, []byte("not json"), 0644)
	if _, err := loadRules(f); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadRules_MissingFile(t *testing.T) {
	if _, err := loadRules("/nonexistent/rules.json"); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadRules_EmptyRules(t *testing.T) {
	f := filepath.Join(t.TempDir(), "rules.json")
	os.WriteFile(f, []byte(`{"rules":[]}`), 0644)
	loaded, err := loadRules(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(loaded.Rules) != 0 {
		t.Errorf("expected empty rules, got %d", len(loaded.Rules))
	}
}

// --- In-process tests: ackAllHandler ---

func TestAckAllHandler_ReturnsAA(t *testing.T) {
	addr := startTestServer(t, ackAllHandler)
	resp := sendAndReceive(t, addr, buildHL7("ADT", "A01"))
	if ack, _, _ := getMSA(resp); ack != "AA" {
		t.Errorf("expected AA, got %q", ack)
	}
}

func TestAckAllHandler_ReturnsAA_ForAnyMsgType(t *testing.T) {
	addr := startTestServer(t, ackAllHandler)
	for _, tc := range []struct{ msgType, trigEvt string }{
		{"ORM", "O01"}, {"ORU", "R01"}, {"SIU", "S12"}, {"MDM", "T01"}, {"MFN", ""},
	} {
		resp := sendAndReceive(t, addr, buildHL7(tc.msgType, tc.trigEvt))
		if ack, _, _ := getMSA(resp); ack != "AA" {
			t.Errorf("expected AA for %s^%s, got %q", tc.msgType, tc.trigEvt, ack)
		}
	}
}

func TestAckAllHandler_NoERRSegment(t *testing.T) {
	addr := startTestServer(t, ackAllHandler)
	resp := sendAndReceive(t, addr, buildHL7("ADT", "A01"))
	if _, found := getERR(resp); found {
		t.Error("ackAllHandler should not include ERR segment")
	}
}

// --- In-process tests: chaosHandler ---

func TestChaosHandler_ReturnsAR(t *testing.T) {
	addr := startTestServer(t, chaosHandler)
	resp := sendAndReceive(t, addr, buildHL7("ADT", "A01"))
	if ack, _, _ := getMSA(resp); ack != "AR" {
		t.Errorf("expected AR, got %q", ack)
	}
}

func TestChaosHandler_HasERRSegment(t *testing.T) {
	addr := startTestServer(t, chaosHandler)
	resp := sendAndReceive(t, addr, buildHL7("ADT", "A01"))
	if _, found := getERR(resp); !found {
		t.Error("expected ERR segment in chaos response")
	}
}

func TestChaosHandler_ReturnsAR_ForAnyMsgType(t *testing.T) {
	addr := startTestServer(t, chaosHandler)
	for _, tc := range []struct{ msgType, trigEvt string }{
		{"ORM", "O01"}, {"ORU", "R01"}, {"SIU", "S12"},
	} {
		resp := sendAndReceive(t, addr, buildHL7(tc.msgType, tc.trigEvt))
		if ack, _, _ := getMSA(resp); ack != "AR" {
			t.Errorf("expected AR for %s^%s, got %q", tc.msgType, tc.trigEvt, ack)
		}
	}
}

// --- In-process tests: smartHandler ---

func TestSmartHandler_ADT_A01_AA(t *testing.T) {
	addr := startTestServer(t, makeSmartHandler([]Rule{
		{Match: "ADT^A01", Response: "AA", AckText: "Patient admitted"},
	}))
	resp := sendAndReceive(t, addr, buildHL7("ADT", "A01"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "Patient admitted" {
		t.Errorf("got ack=%q text=%q", ack, text)
	}
}

func TestSmartHandler_ORM_AE(t *testing.T) {
	addr := startTestServer(t, makeSmartHandler([]Rule{
		{Match: "ORM", Response: "AE", ErrorCode: 207, ErrorSeverity: "E", ErrorMsg: "Order failed"},
	}))
	resp := sendAndReceive(t, addr, buildHL7("ORM", "O01"))
	if ack, _, _ := getMSA(resp); ack != "AE" {
		t.Errorf("expected AE, got %q", ack)
	}
	if errCode, found := getERR(resp); !found || errCode != "207" {
		t.Errorf("expected ERR 207, found=%v code=%q", found, errCode)
	}
}

func TestSmartHandler_ORM_AR(t *testing.T) {
	addr := startTestServer(t, makeSmartHandler([]Rule{
		{Match: "ORM", Response: "AR", ErrorCode: 207, ErrorSeverity: "F", ErrorMsg: "Order rejected"},
	}))
	resp := sendAndReceive(t, addr, buildHL7("ORM", "O01"))
	if ack, _, _ := getMSA(resp); ack != "AR" {
		t.Errorf("expected AR, got %q", ack)
	}
}

func TestSmartHandler_GenericTypeFallback(t *testing.T) {
	addr := startTestServer(t, makeSmartHandler([]Rule{
		{Match: "ADT^A01", Response: "AE"},
		{Match: "ADT", Response: "AA", AckText: "generic ADT"},
		{Match: "*", Response: "AR"},
	}))
	// ADT^A99 has no exact rule — falls to ADT type rule
	resp := sendAndReceive(t, addr, buildHL7("ADT", "A99"))
	if ack, _, _ := getMSA(resp); ack != "AA" {
		t.Errorf("expected AA for generic ADT fallback, got %q", ack)
	}
}

func TestSmartHandler_WildcardFallback(t *testing.T) {
	addr := startTestServer(t, makeSmartHandler([]Rule{
		{Match: "ADT", Response: "AE"},
		{Match: "*", Response: "AA"},
	}))
	resp := sendAndReceive(t, addr, buildHL7("ORU", "R01"))
	if ack, _, _ := getMSA(resp); ack != "AA" {
		t.Errorf("expected wildcard AA, got %q", ack)
	}
}

func TestSmartHandler_NackRate_AlwaysNACK(t *testing.T) {
	addr := startTestServer(t, makeSmartHandler([]Rule{
		{Match: "*", Response: "AA", NackRate: 1.0},
	}))
	for range 5 {
		resp := sendAndReceive(t, addr, buildHL7("ADT", "A01"))
		if ack, _, _ := getMSA(resp); ack != "AR" {
			t.Errorf("expected AR from nack_rate=1.0, got %q", ack)
		}
	}
}

func TestSmartHandler_NackRate_NeverNACK(t *testing.T) {
	addr := startTestServer(t, makeSmartHandler([]Rule{
		{Match: "*", Response: "AA", NackRate: 0.0},
	}))
	for range 5 {
		resp := sendAndReceive(t, addr, buildHL7("ADT", "A01"))
		if ack, _, _ := getMSA(resp); ack != "AA" {
			t.Errorf("expected AA from nack_rate=0.0, got %q", ack)
		}
	}
}

func TestSmartHandler_Delay(t *testing.T) {
	const delayMs = 150
	addr := startTestServer(t, makeSmartHandler([]Rule{
		{Match: "*", Response: "AA", DelayMs: delayMs},
	}))
	start := time.Now()
	sendAndReceive(t, addr, buildHL7("ADT", "A01"))
	if elapsed := time.Since(start); elapsed < time.Duration(delayMs)*time.Millisecond {
		t.Errorf("expected at least %dms delay, got %v", delayMs, elapsed)
	}
}

func TestSmartHandler_CustomAckText(t *testing.T) {
	const want = "All systems nominal"
	addr := startTestServer(t, makeSmartHandler([]Rule{
		{Match: "*", Response: "AA", AckText: want},
	}))
	resp := sendAndReceive(t, addr, buildHL7("ADT", "A01"))
	if _, _, text := getMSA(resp); text != want {
		t.Errorf("expected ack_text %q, got %q", want, text)
	}
}

func TestSmartHandler_CustomErrorCode(t *testing.T) {
	addr := startTestServer(t, makeSmartHandler([]Rule{
		{Match: "*", Response: "AE", ErrorCode: 100, ErrorSeverity: "W"},
	}))
	resp := sendAndReceive(t, addr, buildHL7("ADT", "A01"))
	if errCode, found := getERR(resp); !found || errCode != "100" {
		t.Errorf("expected ERR code 100, found=%v code=%q", found, errCode)
	}
}

func TestSmartHandler_DefaultErrorCode_207(t *testing.T) {
	addr := startTestServer(t, makeSmartHandler([]Rule{
		{Match: "*", Response: "AE"},
	}))
	resp := sendAndReceive(t, addr, buildHL7("ADT", "A01"))
	if errCode, found := getERR(resp); !found || errCode != "207" {
		t.Errorf("expected default ERR code 207, found=%v code=%q", found, errCode)
	}
}

func TestSmartHandler_AA_HasNoERRSegment(t *testing.T) {
	addr := startTestServer(t, makeSmartHandler([]Rule{{Match: "*", Response: "AA"}}))
	resp := sendAndReceive(t, addr, buildHL7("ADT", "A01"))
	if _, found := getERR(resp); found {
		t.Error("AA response should not contain ERR segment")
	}
}

func TestSmartHandler_NoMatchingRule_DefaultsToAA(t *testing.T) {
	addr := startTestServer(t, makeSmartHandler([]Rule{
		{Match: "ORM", Response: "AE"},
	}))
	resp := sendAndReceive(t, addr, buildHL7("ADT", "A01"))
	if ack, _, _ := getMSA(resp); ack != "AA" {
		t.Errorf("expected default AA for unmatched rule, got %q", ack)
	}
}

func TestSmartHandler_EmptyRules_DefaultsToAA(t *testing.T) {
	addr := startTestServer(t, makeSmartHandler(nil))
	resp := sendAndReceive(t, addr, buildHL7("ADT", "A01"))
	if ack, _, _ := getMSA(resp); ack != "AA" {
		t.Errorf("expected AA for empty rules, got %q", ack)
	}
}

func TestSmartHandler_BadMessage_LastDitchError(t *testing.T) {
	addr := startTestServer(t, makeSmartHandler([]Rule{{Match: "*", Response: "AA"}}))
	resp := sendAndReceive(t, addr, "EVN|A01|20240101\rPID|||123\r")
	if ack, _, _ := getMSA(resp); ack != "AR" {
		t.Errorf("expected AR from last-ditch error, got %q", ack)
	}
}

func TestSmartHandler_EchoesControlID(t *testing.T) {
	addr := startTestServer(t, makeSmartHandler([]Rule{{Match: "*", Response: "AA"}}))
	msg := "MSH|^~\\&|SENDER|FAC|RECV|FAC2|20240101120000||ADT^A01^ADT_A01|MYCTRLID|P|2.5\r"
	resp := sendAndReceive(t, addr, msg)
	if _, ctrlID, _ := getMSA(resp); ctrlID != "MYCTRLID" {
		t.Errorf("expected control ID echo %q, got %q", "MYCTRLID", ctrlID)
	}
}

func TestSmartHandler_MultipleMessagesOnOneConnection(t *testing.T) {
	addr := startTestServer(t, makeSmartHandler([]Rule{
		{Match: "ADT^A01", Response: "AA"},
		{Match: "ORM", Response: "AE", ErrorCode: 207},
		{Match: "ORU^R01", Response: "AA"},
	}))

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
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

	if ack, _, _ := getMSA(send(buildHL7("ADT", "A01"))); ack != "AA" {
		t.Errorf("msg1 ADT^A01: expected AA")
	}
	if ack, _, _ := getMSA(send(buildHL7("ORM", "O01"))); ack != "AE" {
		t.Errorf("msg2 ORM^O01: expected AE")
	}
	if ack, _, _ := getMSA(send(buildHL7("ORU", "R01"))); ack != "AA" {
		t.Errorf("msg3 ORU^R01: expected AA")
	}
}

func TestSmartHandler_LoadedFromFile(t *testing.T) {
	raw := `{"rules":[
		{"match":"ADT^A01","response":"AA","ack_text":"from file"},
		{"match":"*","response":"AR"}
	]}`
	f := filepath.Join(t.TempDir(), "rules.json")
	os.WriteFile(f, []byte(raw), 0644)

	cfg, err := loadRules(f)
	if err != nil {
		t.Fatalf("loadRules: %v", err)
	}
	addr := startTestServer(t, smartHandler(cfg))

	resp := sendAndReceive(t, addr, buildHL7("ADT", "A01"))
	if ack, _, text := getMSA(resp); ack != "AA" || text != "from file" {
		t.Errorf("expected AA/from file, got ack=%q text=%q", ack, text)
	}

	resp2 := sendAndReceive(t, addr, buildHL7("ORU", "R01"))
	if ack, _, _ := getMSA(resp2); ack != "AR" {
		t.Errorf("expected AR from wildcard, got %q", ack)
	}
}
