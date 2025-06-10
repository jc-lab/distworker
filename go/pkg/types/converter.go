// distworker
// Copyright (C) 2025 JC-Lab
//
// SPDX-License-Identifier: AGPL-3.0-only
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"fmt"
	"github.com/jc-lab/distworker/go/internal/protocol"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

//goland:noinspection GoMixedReceiverTypes
func (i WorkerStatus) MarshalBSONValue() (bsontype.Type, []byte, error) {
	return bson.TypeString, bsoncore.AppendString(nil, string(i)), nil
}

func (i *WorkerStatus) UnmarshalBSONValue(t bsontype.Type, value []byte) error {
	if t != bson.TypeString {
		return fmt.Errorf("invalid bson value type '%s'", t.String())
	}
	s, _, ok := bsoncore.ReadString(value)
	if !ok {
		return fmt.Errorf("invalid bson string value")
	}

	*i = WorkerStatus(s)
	return nil
}

// ConvertWorkerHealthFromProto converts protobuf WorkerStatus to pkg WorkerStatus
func ConvertWorkerHealthFromProto(protoStatus protocol.WorkerHealth) WorkerHealth {
	switch protoStatus {
	case protocol.WorkerHealth_WORKER_HEALTH_UP:
		return WorkerHealthUp
	case protocol.WorkerHealth_WORKER_HEALTH_DOWN:
		return WorkerHealthDown
	case protocol.WorkerHealth_WORKER_HEALTH_WARN:
		return WorkerHealthWarning
	default:
		return WorkerHealthUnknown
	}
}

func ResourceInfoFromProto(info *protocol.ResourceInfo) map[string]interface{} {
	out := make(map[string]interface{})
	out["hostname"] = info.GetHostname()
	out["cpu_cores"] = info.GetCpuCores()
	out["memory_mb"] = info.GetMemoryMb()
	for k, v := range info.GetAdditional().AsMap() {
		out[k] = v
	}
	return out
}
