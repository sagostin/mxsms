package sinch

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/kr/pretty"
)

func TestIncoming(t *testing.T) {
	sms := &SMS{
		Key:    "8efa3870-ec30-4a55-b612-0a9065d4e5f7",
		Secret: "Ai9PHJVc/UKHpPgiqaZgOA==",
	}
	data := `{
    "event": "incomingSms",
    "to": {
        "type": "number",
        "endpoint": "+46700000000"
    },
    "from": {
        "type": "number",
        "endpoint": "+46700000001"
    },
    "message": "Hello world",
    "timestamp": "2014-12-01T12:00:00Z",
    "version": 1
}`
	// формируем запрос
	req, err := http.NewRequest("POST", "http://localhost:8080/incoming", strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	response := make(map[string]interface{})
	if err := sms.request(req, &response); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	pretty.Println(response)
}

func TestIncomingSign(t *testing.T) {
	sms := &SMS{
		Key:    "8efa3870-ec30-4a55-b612-0a9065d4e5f7",
		Secret: "Ai9PHJVc/UKHpPgiqaZgOA==",
	}
	data := `{
    "event": "incomingSms",
    "to": {
        "type": "number",
        "endpoint": "+46700000000"
    },
    "from": {
        "type": "number",
        "endpoint": "+46700000001"
    },
    "message": "Hello world",
    "timestamp": "2014-12-01T12:00:00Z",
    "version": 1
}`
	// формируем запрос
	req, err := http.NewRequest("POST", "http://localhost:8080/incoming", strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Timestamp", time.Now().UTC().Format(time.RFC3339))
	req.Header.Set("Accept", "application/json")
	if req.Method == "POST" || req.Method == "PUT" || req.Method == "PATCH" {
		req.Header.Set("Content-Type", "application/json")
	}
	signature, err := sms.sign(req)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Application %s:%s", sms.Key, signature))
	msg, err := sms.IncomingHTTP(req)
	if err != nil {
		t.Fatal(err)
	}
	pretty.Println(msg)
}
