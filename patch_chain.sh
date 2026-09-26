#!/bin/bash
sed -i '' 's|"github.com/hyperledger/fabric-protos-go/common"|"github.com/hyperledger/fabric-protos-go/common"\n\t"github.com/golang/protobuf/proto"|g' fabric/consenter/chain.go
