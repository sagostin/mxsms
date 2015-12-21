package sms

import (
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/kr/pretty"
)

func TestStatus(t *testing.T) {
	stat := `id:881444543 sub:001 dlvrd:000 submit date:1512201521 done date:1512201521 stat:ACCEPTD err:000 text:ACK/OK      `
	re := regexp.MustCompile(`^\s*id:(\d+) sub:(\d+) dlvrd:(\d+) submit date:(\d+) done date:(\d+) stat:(\w+) err:(\d+) text:(.+?)\s*$`)
	parts := re.FindStringSubmatch(stat)
	status := Status{
		ID:     parts[1],
		Sub:    0,
		Dlvrd:  0,
		Submit: time.Now(),
		Done:   time.Now(),
		Stat:   parts[6],
		Err:    0,
		Text:   parts[8],
	}
	var err error
	status.Sub, err = strconv.Atoi(parts[2])
	if err != nil {
		t.Error(err)
	}
	status.Dlvrd, err = strconv.Atoi(parts[3])
	if err != nil {
		t.Error(err)
	}
	status.Submit, err = time.Parse(statusTimeFormat, parts[4])
	if err != nil {
		t.Error(err)
	}
	status.Done, err = time.Parse(statusTimeFormat, parts[5])
	if err != nil {
		t.Error(err)
	}
	status.Err, err = strconv.Atoi(parts[7])
	if err != nil {
		t.Error(err)
	}
	pretty.Println(status)
}
