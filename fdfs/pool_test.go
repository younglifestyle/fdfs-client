package fdfs

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"myClient/library/pool"
	xtime "myClient/library/time"
	"testing"
	"time"
)

var p *Pool
var config *Config
var testTrackerAddr = "10.0.2.15:22122"

func init() {
	config = &Config{
		Proto:        "tcp",
		Addr:         testTrackerAddr,
		DialTimeout:  xtime.Duration(time.Second),
		ReadTimeout:  xtime.Duration(time.Second),
		WriteTimeout: xtime.Duration(time.Second),
	}
	config.Config = &pool.Config{
		Active:      10,
		Idle:        5,
		IdleTimeout: xtime.Duration(90 * time.Second),
	}
}

func prepareEnv2() {
	if p != nil {
		return
	}
	p = NewPool(config)
}

func TestWriteHeader(t *testing.T) {
	prepareEnv2()
	conn := p.Get(context.Background())
	defer conn.Close()

	task := &trackerTask{
		header: header{cmd: TRACKER_PROTO_CMD_SERVICE_QUERY_STORE_WITHOUT_GROUP_ONE},
	}

	buffer := new(bytes.Buffer)
	if err := binary.Write(buffer, binary.BigEndian, task.header.pkgLen); err != nil {
		t.Errorf("binary write error(%v)", err)
		return
	}
	buffer.WriteByte(byte(task.header.cmd))
	buffer.WriteByte(byte(task.header.status))

	if err := conn.Send(buffer.Bytes()); err != nil {
		t.Errorf("binary write error(%v)", err)
		return
	}

	buf, err := conn.Receive()
	if err != nil {
		t.Errorf("read error(%v)", err)
		return
	}

	hd := &header{}
	buff := bytes.NewBuffer(buf[:10])
	if err := binary.Read(buff, binary.BigEndian, &hd.pkgLen); err != nil {
		t.Errorf("binary write error(%v)", err)
		return
	}

	cmd, err := buffer.ReadByte()
	if err != nil {
		t.Errorf("binary write error(%v)", err)
		return
	}
	status, err := buffer.ReadByte()
	if err != nil {
		t.Errorf("binary write error(%v)", err)
		return
	}

	fmt.Println("length :", hd.pkgLen)
	fmt.Println("lengt123 :", len(buf))
	fmt.Println("cmd :", cmd)
	fmt.Println("status :", status)

	for i := 0; i < 100; i++ {
		/*get :=*/ p.Get(context.Background())
		//get.Close()
	}

	time.Sleep(time.Second*5)
}