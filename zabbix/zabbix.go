package zabbix

import "os/exec"

type Log struct {
	host string // имя сервера
}

func New(host string) *Log {
	return &Log{host: host}
}

func (z Log) Send(key, value string) error {
	return exec.Command("zabbix_sender", "-s", z.host, "-k", key, "-v", value).Run()
}
