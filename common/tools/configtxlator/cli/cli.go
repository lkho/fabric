package cli

import (
	"io"
	"github.com/hyperledger/fabric/common/tools/protolator"
	"fmt"
	"github.com/golang/protobuf/proto"
	"reflect"
	"os"
	"io/ioutil"
	"runtime"
	"path"
	"gopkg.in/alecthomas/kingpin.v2"
	"github.com/hyperledger/fabric/common/tools/configtxlator/update"
	"github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/common/tools/configtxlator/sanitycheck"
	"encoding/json"
	"github.com/hyperledger/fabric/protos/utils"
	"github.com/hyperledger/fabric/msp/mgmt"
	"github.com/hyperledger/fabric/bccsp/factory"
	"github.com/hyperledger/fabric/common/localmsp"
	"github.com/hyperledger/fabric/common/crypto"
	"github.com/hyperledger/fabric/common/util"
)

type Cli struct {
	app           *kingpin.Application
	encodeCmd     *kingpin.CmdClause
	encodeMsg     *string
	encodeIn      **os.File
	encodeInJson  *bool
	encodeOut     *string
	encodeOutJson *bool

	updateCmd     *kingpin.CmdClause
	updateIn1     **os.File
	updateIn1Json *bool
	updateIn2     **os.File
	updateIn2Json *bool
	updateChannel *string
	updateEnv     *bool
	updateOut     *string
	updateOutJson *bool

	verifyCmd    *kingpin.CmdClause
	verifyIn     **os.File
	verifyInJson *bool
	verifyOut    *string

	signCmd     *kingpin.CmdClause
	signIn      **os.File
	signInJson  *bool
	signChannel *string
	signMspId   *string
	signMspDir  *string
	signOut     *string
	signOutJson *bool
}

func NewCli(app *kingpin.Application) (cli *Cli) {
	cli = new(Cli)
	cli.app = app

	cli.encodeCmd = app.Command("encode", "encode/decode between protobuf/json format")
	cli.encodeMsg = cli.encodeCmd.Flag("msg", "message name e.g. common.Block").Short('m').Required().String()
	cli.encodeIn = cli.encodeCmd.Flag("in", "input file. Read from STDIN if not specified.").Short('i').File()
	cli.encodeInJson = cli.encodeCmd.Flag("ij", "use json format instead of protobuf").Bool()
	cli.encodeOut = cli.encodeCmd.Flag("out", "output file. Write to STDOUT if not specified.").Short('o').String()
	cli.encodeOutJson = cli.encodeCmd.Flag("oj", "use json format instead of protobuf").Bool()

	cli.updateCmd = app.Command("update", "compute update from configs")
	cli.updateIn1 = cli.updateCmd.Flag("original", "original Config").Short('1').File()
	cli.updateIn1Json = cli.updateCmd.Flag("1j", "use json format instead of protobuf").Bool()
	cli.updateIn2 = cli.updateCmd.Flag("updated", "updated Config").Short('2').File()
	cli.updateIn2Json = cli.updateCmd.Flag("2j", "use json format instead of protobuf").Bool()
	cli.updateChannel = cli.updateCmd.Flag("channel", "Channel ID").Required().Short('c').String()
	cli.updateEnv = cli.updateCmd.Flag("envelope", "output ConfigUpdate wrapped in Envelope").Short('e').Bool()
	cli.updateOut = cli.updateCmd.Flag("out", "output ConfigUpdate file. Write to STDOUT if not specified.").Short('o').String()
	cli.updateOutJson = cli.updateCmd.Flag("oj", "use json format instead of protobuf").Bool()

	cli.verifyCmd = app.Command("verify", "Sanity check Config")
	cli.verifyIn = cli.verifyCmd.Flag("in", "input Config. Read from STDIN if not specified.").Short('i').File()
	cli.verifyInJson = cli.verifyCmd.Flag("ij", "use json format instead of protobuf").Bool()
	cli.verifyOut = cli.verifyCmd.Flag("out", "output json file. Write to STDOUT if not specified.").Short('o').String()

	cli.signCmd = app.Command("sign-config-update", "sign a ConfigUpdate Envelope. Signatures will be appended").Alias("sc")
	cli.signIn = cli.signCmd.Flag("in", "input Envelope file. Read from STDIN if not specified.").Short('i').File()
	cli.signInJson = cli.signCmd.Flag("ij", "use json format instead of protobuf").Bool()
	cli.signChannel = cli.signCmd.Flag("channel", "Channel ID").Required().Short('c').String()
	cli.signMspId = cli.signCmd.Flag("mspid", "MSP ID used to sign").Required().String()
	cli.signMspDir = cli.signCmd.Flag("mspdir", "MSP directory path").Required().ExistingDir()
	cli.signOut = cli.signCmd.Flag("out", "output Envelope file. Write to STDOUT if not specified.").Short('o').String()
	cli.signOutJson = cli.signCmd.Flag("oj", "use json format instead of protobuf").Bool()

	return cli
}

func fileLine(skip int) (s string) {
	_, fileName, fileLine, ok := runtime.Caller(skip)
	if ok {
		s = fmt.Sprintf("%s:%d ", path.Base(fileName), fileLine)
	} else {
		s = ""
	}
	return
}

func (cli *Cli) FatalIfError(err error, format string, args ...interface{}) {
	if err != nil {
		cli.app.FatalIfError(err, "%s%s", fileLine(2), fmt.Sprintf(format, args...))
	}
}

func Errorf(format string, args ...interface{}) error {
	return fmt.Errorf("%s%s", fileLine(2), fmt.Sprintf(format, args...))
}

func (cli *Cli) Run(command string) {
	switch command {

	case cli.encodeCmd.FullCommand():
		cli.Encode(
			*cli.encodeMsg,
			Input{Stdin: true, File: *cli.encodeIn, Json: *cli.encodeInJson},
			Output{Stdout: true, File: cli.encodeOut, Json: *cli.encodeOutJson})

	case cli.updateCmd.FullCommand():
		cli.Update(
			Input{Stdin: false, File: *cli.updateIn1, Json: *cli.updateIn1Json},
			Input{Stdin: false, File: *cli.updateIn2, Json: *cli.updateIn2Json},
			*cli.updateChannel,
			*cli.updateEnv,
			Output{Stdout: true, File: cli.updateOut, Json: *cli.updateOutJson})

	case cli.verifyCmd.FullCommand():
		cli.Verify(
			Input{Stdin: true, File: *cli.verifyIn, Json: *cli.verifyInJson},
			Output{Stdout: true, File: cli.updateOut, Json: true})

	case cli.signCmd.FullCommand():
		cli.Sign(
			Input{Stdin: true, File: *cli.signIn, Json: *cli.signInJson},
			Output{Stdout: true, File: cli.signOut, Json: *cli.signOutJson},
			*cli.signChannel,
			*cli.signMspId,
			*cli.signMspDir)
	}
}

type Input struct {
	File  *os.File
	Json  bool
	Stdin bool
}

func (in *Input) GetStream() *os.File {
	if in.File != nil {
		return in.File
	} else if in.Stdin {
		return os.Stdin
	}
	return nil
}

func (in *Input) Unmarshal(pb proto.Message) error {
	s := in.GetStream()
	if s == nil {
		return Errorf("No input specified")
	}
	if !in.Json {
		buf, err := ioutil.ReadAll(s)
		if err != nil {
			return Errorf("Cannot read File: %s", err)
		}
		err = proto.Unmarshal(buf, pb)
		if err != nil {
			return Errorf("Cannot unmarshal pb: %s", err)
		}
	} else {
		err := protolator.DeepUnmarshalJSON(s, pb)
		if err != nil {
			return Errorf("Cannot unmarshal Json: %s", err)
		}
	}
	return nil
}

type Output struct {
	File   *string
	Json   bool
	Stdout bool
}

func (out *Output) GetStream() (io.Writer, error) {
	if *out.File != "" {
		f, err := os.Create(*out.File)
		if err != nil {
			return nil, Errorf("%s", err)
		}
		return f, nil
	} else if out.Stdout {
		return os.Stdout, nil
	}
	return nil, nil
}

func (out *Output) Marshal(pb proto.Message) error {
	s, err := out.GetStream()
	if err != nil {
		return err
	}
	if s == nil {
		fmt.Printf("%s: warning: no output specified\n", fileLine(1))
		return nil
	}
	if out.Json {
		err = protolator.DeepMarshalJSON(s, pb)
		if err != nil {
			return Errorf("Cannot marshal Json: %s", err)
		}
	} else {
		buf, err := proto.Marshal(pb)
		if err != nil {
			return Errorf("Cannot marshal protobuf: %s", err)
		}
		_, err = s.Write(buf)
		if err != nil {
			return Errorf("Cannot write to output File: %s", err)
		}
	}
	return nil
}

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
	config := common.Config{}
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
	originalConfig, updatedConfig := common.Config{}, common.Config{}

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
		configUpdateEnvelope := &common.ConfigUpdateEnvelope{}
		configUpdateEnvelope.ConfigUpdate, err = proto.Marshal(configUpdate)
		cli.FatalIfError(err, "Cannot marshal ConfigUpdate")

		out := cli.wrapChannelEnvelope(configUpdateEnvelope, common.HeaderType_CONFIG_UPDATE, 0, channel, 0)
		err = output.Marshal(out)
	} else {
		err = output.Marshal(configUpdate)
	}
	cli.FatalIfError(err, "Error sending output")
}

func (cli *Cli) wrapChannelEnvelope(pb proto.Message, headerType common.HeaderType, version int32, channel string, epoch uint64) *common.Envelope {

	encoded, err := proto.Marshal(pb)

	payloadChannelHeader := utils.MakeChannelHeader(headerType, version, channel, epoch)
	payloadHeader := &common.Header{}
	payloadHeader.ChannelHeader, err = proto.Marshal(payloadChannelHeader)
	cli.FatalIfError(err, "Cannot marshal ChannelHeader")

	payloadBytes, err := proto.Marshal(&common.Payload{
		Header: payloadHeader,
		Data:   encoded,
	})
	cli.FatalIfError(err, "Cannot marshal Payload")

	return &common.Envelope{Payload: payloadBytes}
}

func (cli *Cli) unwrapChannelEnvelope(envelope *common.Envelope, pb proto.Message) {
	payload := &common.Payload{}
	err := proto.Unmarshal(envelope.Payload, payload)
	cli.FatalIfError(err, "Cannot unmarshal Payload")
	err = proto.Unmarshal(payload.Data, pb)
	cli.FatalIfError(err, "Cannot unmarshal Payload.Data")
	return
}

func (cli *Cli) Sign(input Input, output Output, channel string, mspID string, mspDir string) {
	envelop := &common.Envelope{}
	err := input.Unmarshal(envelop)
	cli.FatalIfError(err, "Error reading input")

	configUpdateEnvelope := &common.ConfigUpdateEnvelope{}
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
		configSig := &common.ConfigSignature{}
		configSig.SignatureHeader, err = proto.Marshal(sigHeader)
		cli.FatalIfError(err, "Cannot marshal signature header")
		configSig.Signature, err = signer.Sign(util.ConcatenateBytes(configSig.SignatureHeader, configUpdateEnvelope.ConfigUpdate))
		cli.FatalIfError(err, "Failed to sign ConfigUpdate")
		configUpdateEnvelope.Signatures = append(configUpdateEnvelope.Signatures, configSig)
		fmt.Fprintf(os.Stderr, "Envelope signed with %s\n", mspID)
	}

	out := cli.wrapChannelEnvelope(configUpdateEnvelope, common.HeaderType_CONFIG_UPDATE, 0, channel, 0)
	err = output.Marshal(out)
	cli.FatalIfError(err, "Error writing output Envelope")
}
