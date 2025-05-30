package models

import (
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"time"
)

// UnixTime is a custom time type that marshals to/from Unix milliseconds
type UnixTime struct {
	time.Time
}

// NewUnixTime creates a new UnixTime from a time.Time
func NewUnixTime(t time.Time) UnixTime {
	return UnixTime{Time: t}
}

// NewUnixTime creates a new UnixTime from a time.Time
func NewUnixTimePtr(t time.Time) *UnixTime {
	return &UnixTime{Time: t}
}

// Now creates a new UnixTime with the current time
func Now() UnixTime {
	return UnixTime{Time: time.Now()}
}

// NowPtr creates a new UnixTime with the current time
func NowPtr() *UnixTime {
	return &UnixTime{Time: time.Now()}
}

// MarshalJSON implements the json.Marshaler interface
//
//goland:noinspection GoMixedReceiverTypes
func (t UnixTime) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(t.Unix())
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (t *UnixTime) UnmarshalJSON(data []byte) error {
	var seconds int64
	if err := json.Unmarshal(data, &seconds); err != nil {
		return err
	}
	t.Time = time.Unix(seconds, 0)
	return nil
}

//goland:noinspection GoMixedReceiverTypes
func (t UnixTime) MarshalBSONValue() (bsontype.Type, []byte, error) {
	if t.IsZero() {
		return bson.TypeNull, nil, nil
	}
	return bson.TypeDateTime, bsoncore.AppendDateTime(nil, t.Time.UnixMilli()), nil
}

func (t *UnixTime) UnmarshalBSONValue(typ bsontype.Type, data []byte) error {
	if len(data) == 0 {
		return nil
	}

	dateTime, _, ok := bsoncore.ReadDateTime(data)
	if !ok {
		return fmt.Errorf("invalid BSON DateTime")
	}

	t.Time = time.UnixMilli(dateTime).UTC()
	return nil
}
