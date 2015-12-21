package main

import (
	"fmt"
	"testing"
)

func TestHistory(t *testing.T) {
	froms := []string{"100", "200"}
	tos := []string{"01", "02", "03", "02", "01", "03", "03", "03", "01"}
	var history History
	for _, to := range tos {
		from := history.GetFrom(froms, to)
		history.Add("mxName", "jid", from, to)
		fmt.Println(from, to)
	}
}
