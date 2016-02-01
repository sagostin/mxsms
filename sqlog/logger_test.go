package sqlog

import "testing"

func TestLog(t *testing.T) {
	db, err := Connect("root@/mxsms?charset=utf8")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	err = db.Insert("mx", "in", "out", "text", true, 3, 2345, 2)
	if err != nil {
		t.Fatal(err)
	}
}
