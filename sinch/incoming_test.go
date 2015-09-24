package sinch

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/kr/pretty"
)

func TestIncoming2(t *testing.T) {
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
        "endpoint": "79031744444"
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
