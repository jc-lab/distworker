#!/bin/sh

mkdir -p $PWD/python-worker-sdk/distworker/protocol/
protoc \
	--proto_path=$PWD/proto \
	--python_out=$PWD/python/distworker/protocol/ \
	--pyi_out=$PWD/python/distworker/protocol/ \
	--go_out=$PWD/go/internal/protocol/ \
	--go_opt=paths=source_relative \
	 protocol.proto
