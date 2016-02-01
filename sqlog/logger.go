package sqlog

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

type DB struct {
	db *sql.DB
}

func Connect(url string) (*DB, error) {
	db, err := sql.Open("mysql", url) //"/mxsms?charset=utf8")
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		return nil, err
	}
	return &DB{db: db}, nil
}

func (db *DB) Close() error {
	return db.db.Close()
}

func (db *DB) Insert(mx, in, out, text string, inbound bool, phoneType, pid int64, delivered int) error {
	stmt, err := db.db.Prepare(`INSERT log SET mx=?,calling=?,called=?,inbound=?,text=?,pid=?,delivered=?,type=?`)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(mx, in, out, inbound, text, pid, delivered, phoneType)
	return err
}
