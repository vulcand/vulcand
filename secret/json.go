package secret

import (
	"encoding/json"
	"fmt"
)

type sealedValue struct {
	Encryption string
	Value      SealedBytes
}

func SealedValueToJSON(b *SealedBytes) ([]byte, error) {
	data := &sealedValue{
		Encryption: encryptionSecretBox,
		Value:      *b,
	}
	return json.Marshal(&data)
}

func SealedValueFromJSON(bytes []byte) (*SealedBytes, error) {
	var v *sealedValue
	if err := json.Unmarshal(bytes, &v); err != nil {
		return nil, err
	}
	if v.Encryption != encryptionSecretBox {
		return nil, fmt.Errorf("unsupported encryption type: '%s'", v.Encryption)
	}
	return &v.Value, nil
}

const encryptionSecretBox = "secretbox.v1"
