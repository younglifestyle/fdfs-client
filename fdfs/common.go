package fdfs

import "github.com/pkg/errors"

var (
	ErrConnClosed = errors.New("memcache: connection closed")
)

const (
	TRACKER_PROTO_CMD_RESP                                  = 100
	TRACKER_PROTO_CMD_SERVICE_QUERY_STORE_WITHOUT_GROUP_ONE = 101
	TRACKER_PROTO_CMD_SERVICE_QUERY_FETCH_ONE               = 102

	FDFS_PROTO_CMD_ACTIVE_TEST          = 111
)

type pkgHeader struct {
	pkgLen int64
	cmd    int8
	status int8
}

type trackerStorageInfo struct {
	groupName      string
	ipAddr         string
	port           int64
	storePathIndex int8
}
type header struct {
	pkgLen int64
	cmd    int8
	status int8
}
type trackerTask struct {
	header
	groupName      string
	remoteFilename string
	trackerStorageInfo
}
