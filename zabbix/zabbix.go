package zabbix

import "os/exec"

type Log struct {
	Server string
	Host   string // имя сервера
}

func (z Log) Send(key, value string) error {
	return exec.Command("zabbix_sender",
		"-z", z.Server,
		"-s", z.Host,
		"-k", key,
		"-o", value).Run()
}
