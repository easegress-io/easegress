package cluster

import (
	"bytes"

	"github.com/hashicorp/go-msgpack/codec"
)

func Pack(obj interface{}, t uint8) ([]byte, error) {
	buff := bytes.NewBuffer(nil)

	// write header
	buff.WriteByte(t)

	// write payload
	encoder := codec.NewEncoder(buff, &codec.MsgpackHandle{})
	err := encoder.Encode(obj)
	return buff.Bytes(), err
}

func Unpack(buff []byte, obj interface{}) error {
	decoder := codec.NewDecoder(bytes.NewReader(buff), &codec.MsgpackHandle{})
	return decoder.Decode(obj)
}

////

func PackNodeTags(tags map[string]string) ([]byte, error) {
	buff := bytes.NewBuffer(nil)
	encoder := codec.NewEncoder(buff, &codec.MsgpackHandle{})
	err := encoder.Encode(tags)
	return buff.Bytes(), err
}

func UnpackNodeTags(buff []byte) (map[string]string, error) {
	ret := make(map[string]string)

	decoder := codec.NewDecoder(bytes.NewReader(buff), &codec.MsgpackHandle{})

	err := decoder.Decode(&ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
