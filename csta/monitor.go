package csta

import "encoding/xml"

// MonitorStart запускает монитор пользователя и возвращает его идентификатор.
// В качестве параметра указывается внутренний номер пользователя. Если номер
// не указан, то используется внутренний номер авторизованного пользователя.
func (c *Conn) MonitorStart(ext string) (int64, error) {
	// проверяем, что монитор уже не запущен
	if mid := c.Monitor(ext); mid != 0 {
		return mid, nil
	}
	// отправляем команду на запуск монитора
	resp, err := c.SendWithResponse(&struct {
		XMLName xml.Name `xml:"MonitorStart"`
		Ext     string   `xml:"monitorObject>deviceObject"`
	}{
		Ext: ext,
	})
	if err != nil {
		return 0, err
	}
	// разбираем идентификатор монитора
	var monitor = new(struct {
		ID int64 `xml:"monitorCrossRefID"`
	})
	if err = resp.Decode(monitor); err != nil {
		return 0, err
	}
	// сохраняем идентификатор монитора пользователя
	c.monitors.Store(ext, monitor.ID)
	return monitor.ID, nil
}

// MonitorStopID останавливает ранее запущенный монитор пользователя. Если
// идентификатор монитора не задан, то останавливается монитор авторизованного
// пользователя, если он был раньше запущен.
func (c *Conn) MonitorStopID(id int64) error {
	if id == 0 {
		// монитор авторизованного пользователя
		if id = c.Monitor(c.Ext); id == 0 {
			return nil
		}
	}
	// отправляем команду на остановку монитора
	_, err := c.SendWithResponse(&struct {
		XMLName xml.Name `xml:"MonitorStop"`
		ID      int64    `xml:"monitorCrossRefID"`
	}{
		ID: id,
	})
	// находим монитор по его идентификатору и удаляем его из списка
	c.monitors.Range(func(ext, mid interface{}) bool {
		if mid.(int64) == id {
			c.monitors.Delete(ext)
			return false
		}
		return true
	})
	return err
}

// MonitorStop останавливает ранее запущенный монитор пользователя.
func (c *Conn) MonitorStop(ext string) error {
	if mid := c.Monitor(ext); mid != 0 {
		return c.MonitorStopID(mid)
	}
	return nil
}

// Monitor возвращает номер запущенного монитора для указанного внутреннего
// номера пользователя.
func (c *Conn) Monitor(ext string) int64 {
	if ext == "" {
		ext = c.Ext // номер авторизованного пользователя
	}
	if mid, ok := c.monitors.Load(ext); ok {
		return mid.(int64)
	}
	return 0
}

// MonitorExt возвращает внутренний номер по идентификатору монитора.
func (c *Conn) MonitorExt(id int64) string {
	if id == 0 {
		return c.Ext // номер авторизованного пользователя
	}
	var result string
	c.monitors.Range(func(ext, mid interface{}) bool {
		if mid.(int64) == id {
			result = ext.(string)
			return false
		}
		return true
	})
	return result
}
