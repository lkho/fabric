package cli

import (
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/msp/mgmt"
	"github.com/hyperledger/fabric/bccsp/factory"
	"github.com/hyperledger/fabric/common/localmsp"
	"reflect"
	"github.com/hyperledger/fabric/common/tools/configtxlator/sanitycheck"
	"encoding/json"
	"github.com/hyperledger/fabric/common/tools/configtxlator/update"
	"fmt"
	"os"
	cb "github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/common/crypto"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/protos/utils"
)

func getMsgType(msgName string) (proto.Message, error) {
	msgType := proto.MessageType(msgName)
	if msgType == nil {
		return nil, Errorf("Unknown message name '%s'", msgName)
	}
	return reflect.New(msgType.Elem()).Interface().(proto.Message), nil
}

func (cli *Cli) Encode(msgName string, input Input, output Output) {
	msg, err := getMsgType(msgName)
	cli.FatalIfError(err, "Invalid argument: %s", err)
	err = input.Unmarshal(msg)
	cli.FatalIfError(err, "Error reading input")
	err = output.Marshal(msg)
	cli.FatalIfError(err, "Error sending output")
}

func (cli *Cli) Verify(input Input, output Output) {
	config := cb.Config{}
	err := input.Unmarshal(&config)
	cli.FatalIfError(err, "Error reading input")

	sanityCheckMessages, err := sanitycheck.Check(&config)
	cli.FatalIfError(err, "Error performing sanity check")

	resBytes, err := json.Marshal(sanityCheckMessages)
	cli.FatalIfError(err, "Failed to marshal json")
	w, err := output.GetStream()
	cli.FatalIfError(err, "Error sending output")
	_, err = w.Write(resBytes)
	cli.FatalIfError(err, "Error sending output")
}

func (cli *Cli) Update(in1 Input, in2 Input, channel string, envelop bool, output Output) {
	originalConfig, updatedConfig := cb.Config{}, cb.Config{}

	err := in1.Unmarshal(&originalConfig)
	cli.FatalIfError(err, "Failed to read original config")
	err = in2.Unmarshal(&updatedConfig)
	cli.FatalIfError(err, "Failed to read updated config")

	configUpdate, err := update.Compute(&originalConfig, &updatedConfig)
	cli.FatalIfError(err, "Error computing config update")

	configUpdate.ChannelId = channel

	// wrap Envelope
	if envelop {
		// inner envelope
		configUpdateEnvelope := &cb.ConfigUpdateEnvelope{}
		configUpdateEnvelope.ConfigUpdate, err = proto.Marshal(configUpdate)
		cli.FatalIfError(err, "Cannot marshal ConfigUpdate")

		out := cli.wrapChannelEnvelope(configUpdateEnvelope, cb.HeaderType_CONFIG_UPDATE, 0, channel, 0)
		err = output.Marshal(out)
	} else {
		err = output.Marshal(configUpdate)
	}
	cli.FatalIfError(err, "Error sending output")
}

func (cli *Cli) wrapChannelEnvelope(pb proto.Message, headerType cb.HeaderType, version int32, channel string, epoch uint64) *cb.Envelope {

	encoded, err := proto.Marshal(pb)

	payloadChannelHeader := utils.MakeChannelHeader(headerType, version, channel, epoch)
	payloadHeader := &cb.Header{}
	payloadHeader.ChannelHeader, err = proto.Marshal(payloadChannelHeader)
	cli.FatalIfError(err, "Cannot marshal ChannelHeader")

	payloadBytes, err := proto.Marshal(&cb.Payload{
		Header: payloadHeader,
		Data:   encoded,
	})
	cli.FatalIfError(err, "Cannot marshal Payload")

	return &cb.Envelope{Payload: payloadBytes}
}

func (cli *Cli) unwrapChannelEnvelope(envelope *cb.Envelope, pb proto.Message) {
	payload := &cb.Payload{}
	err := proto.Unmarshal(envelope.Payload, payload)
	cli.FatalIfError(err, "Cannot unmarshal Payload")
	err = proto.Unmarshal(payload.Data, pb)
	cli.FatalIfError(err, "Cannot unmarshal Payload.Data")
	return
}

func (cli *Cli) Sign(input Input, output Output, channel string, mspID string, mspDir string) {
	envelop := &cb.Envelope{}
	err := input.Unmarshal(envelop)
	cli.FatalIfError(err, "Error reading input")

	configUpdateEnvelope := &cb.ConfigUpdateEnvelope{}
	cli.unwrapChannelEnvelope(envelop, configUpdateEnvelope)

	var signer crypto.LocalSigner
	if mspDir != "" {
		mgmt.LoadLocalMsp(mspDir, factory.GetDefaultOpts(), mspID)
		signer = localmsp.NewSigner()
	} else {
		signer = nil
	}

	if signer != nil {
		sigHeader, err := signer.NewSignatureHeader()
		cli.FatalIfError(err, "Cannot create signature header")
		configSig := &cb.ConfigSignature{}
		configSig.SignatureHeader, err = proto.Marshal(sigHeader)
		cli.FatalIfError(err, "Cannot marshal signature header")
		configSig.Signature, err = signer.Sign(util.ConcatenateBytes(configSig.SignatureHeader, configUpdateEnvelope.ConfigUpdate))
		cli.FatalIfError(err, "Failed to sign ConfigUpdate")
		configUpdateEnvelope.Signatures = append(configUpdateEnvelope.Signatures, configSig)
		fmt.Fprintf(os.Stderr, "Envelope signed with %s\n", mspID)
	}

	out := cli.wrapChannelEnvelope(configUpdateEnvelope, cb.HeaderType_CONFIG_UPDATE, 0, channel, 0)
	err = output.Marshal(out)
	cli.FatalIfError(err, "Error writing output Envelope")
}
