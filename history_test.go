package main

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestHistory(t *testing.T) {
	jids := []string{"jid1", "jid2"}
	froms := []string{"100", "200"}
	tos := []string{"01", "02", "03", "02", "01", "03", "03", "03", "01"}
	var history History
	for _, to := range tos {
		jid := jids[rand.Intn(len(jids))]
		from := history.GetFrom(froms, to, jid)
		history.Add("mxName", jid, from, to)
		fmt.Println(jid, from, to)
	}
}
