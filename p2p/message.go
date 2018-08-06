package p2p

import (
	"encoding/binary"

	"github.com/invin/kkchain/p2p/protobuf"
)


// SerializeMessage serializes the message bytes for cryptographic signing purposes.
func SerializeMessage(id *protobuf.ID, message []byte) []byte {
	const uint32Size = 4
	serialized := make([]byte, uint32Size+len(id.Address)+uint32Size+len(id.PublicKey)+uint32Size+len(message))
	
	buffer := serialized[:]
	buffer = appendElement(buffer, []byte(id.Address))
	buffer = appendElement(buffer, []byte(id.PublicKey))
	buffer = appendElement(buffer, []byte(message))

	if 0 != cap(buffer) {
		panic("internal error: invalid serialization output")
	}

	return serialized
}

// DeserializeMessage deserizes the encoded meesage to original content
func DeserializeMessage(in []byte) (protobuf.ID, []byte) {
	buffer, address := getElement(in)
	buffer, publicKey := getElement(buffer)
	buffer, message := getElement(buffer)

	id := protobuf.ID{
		PublicKey: publicKey,
		Address: string(address),
	}

	return id, message
}

// appendElement appends an element with 4 bytes size plus element content
func appendElement(buffer, element []byte) []byte {
	const uint32Size = 4
	pos := 0
	// write length
	binary.LittleEndian.PutUint32(buffer[pos:], uint32(len(element)))
	pos += uint32Size
	
	// write body
	copy(buffer[pos:], element)
	pos += len(element)

	return buffer[pos:]

}

// getElement gets an element from bytes
func getElement(buffer []byte) ([]byte, []byte) {
	const uint32Size = 4
	pos := 0
	// read length
	len := binary.LittleEndian.Uint32(buffer[:uint32Size])
	pos += uint32Size

	// read body
	body := make([]byte, len)
	copy(body, buffer[pos:(pos+int(len))])
	pos += int(len)

	return buffer[pos:], body
}

