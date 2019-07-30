package fdfs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"
	"time"
	"unsafe"
)

var testAddr = "10.0.2.15:22122"
var c Conn



func prepareEnv(t *testing.T) {
	if c != nil {
		return
	}
	var err error
	cnop := DialConnectTimeout(time.Duration(2 * time.Second))
	rdop := DialReadTimeout(time.Duration(2 * time.Second))
	wrop := DialWriteTimeout(time.Duration(2 * time.Second))
	c, err = Dial("tcp", testAddr, cnop, rdop, wrop)
	if err != nil {
		t.Errorf("Dial() error(%v)", err)
		t.FailNow()
	}
}

func TestOne(t *testing.T) {
	h := header{}
	h1 := trackerTask{}
	fmt.Println( unsafe.Sizeof(h))
	fmt.Println( unsafe.Sizeof(h1))
}

func TestConn(t *testing.T) {
	prepareEnv(t)
}

func TestQueryStorageInfoWithTracker(t *testing.T) {
	prepareEnv(t)

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

	if err := c.Send(buffer.Bytes()); err != nil {
		t.Errorf("binary write error(%v)", err)
		return
	}

	buf, err := c.Receive()
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
}
