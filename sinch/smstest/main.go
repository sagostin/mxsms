package main

import (
	"expvar"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mdigger/sinch2"
)

var (
	addr, key, secret, from string
	sms                     *sinch.SMS
)

func init() {
	expvar.Publish("started", time.Now())
	flag.StringVar(&addr, "http", ":8080", "HTTP server address & port")
	flag.StringVar(&key, "key", "8efa3870-ec30-4a55-b612-0a9065d4e5f7", "Sinch Key")
	flag.StringVar(&secret, "secret", "Ai9PHJVc/UKHpPgiqaZgOA==", "Sinch Secret")
	flag.StringVar(&from, "from", "+14152364961", "From phone number")
	flag.Parse()
	sms = &sinch.SMS{
		Key:    key,
		Secret: secret,
	}
	http.HandleFunc("/", index)
	http.HandleFunc("/send", send)
	http.HandleFunc("/send/", status)
	http.HandleFunc("/incoming", incoming)
}

func main() {
	log.Println("Starting", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func index(w http.ResponseWriter, r *http.Request) {
	log.Println("index:", r.Method, r.URL)
	if r.URL.Path == "/" {
		log.Println("redirect to /send")
		http.Redirect(w, r, "/send", http.StatusMovedPermanently)
		return
	}
	log.Println("not found")
	http.NotFound(w, r)
}

func send(w http.ResponseWriter, r *http.Request) {
	log.Println("send:", r.Method, r.URL)
	if r.Method != "POST" {
		io.WriteString(w, `<!DOCTYPE html>
<meta charset="utf-8">
<form method="POST">
<table>
<tr><td><label>From:</label></td><td><input name="from" value="`+from+`"></td></tr>
<tr><td><label>To:</label></td><td><input name="to" value="+79670238554"></td></tr>
<tr><td valign="top"><label>Message:</label></td><td><textarea name="msg" rows="4" cols="32">Проверка связи!</textarea></td></tr>
<tr><td></td><td><input type="submit" value="Send"></td></tr>
</table>
</form>`)
		return
	}
	msgID, err := sms.Send(r.FormValue("from"), r.FormValue("to"), r.FormValue("msg"))
	if err != nil {
		log.Println("Send SMS error:", err)
		http.Error(w, "Send SMS error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println("Sended message ID:", msgID)
	http.Redirect(w, r, "/send/"+strconv.Itoa(msgID), http.StatusFound)
}

func status(w http.ResponseWriter, r *http.Request) {
	log.Println("status:", r.Method, r.URL)
	msgID, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/send/"))
	if err != nil {
		log.Println("Bad message ID:", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	status, err := sms.Status(msgID)
	if err != nil {
		log.Println("Message Status error:", err)
		http.Error(w, "SMS status error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println("Message Status:", msgID, status)
	io.WriteString(w, `<!DOCTYPE html>
<meta charset="utf-8">
`)
	if status == "Pending" {
		io.WriteString(w, `<meta http-equiv="refresh" content="10">
`)
	}
	fmt.Fprintf(w, "<p>Status: %s</p>", status)
}

func incoming(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.Header().Set("Allowed", "POST")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	msg, err := sms.IncomingHTTP(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Println("Incoming Error:", err)
		return
	}
	fmt.Printf("incoming: %#v\n", msg)
	w.WriteHeader(http.StatusNoContent)
}
