package cli

import (
	"io"
	"github.com/hyperledger/fabric/common/tools/protolator"
	"fmt"
	"github.com/golang/protobuf/proto"
	"os"
	"io/ioutil"
	"runtime"
	"path"
	"gopkg.in/alecthomas/kingpin.v2"
	"encoding/base64"
)

type Cli struct {
	app *kingpin.Application

	encodeCmd *kingpin.CmdClause
	encodeMsg *string
	fIn       **os.File
	fIn64     *bool
	fInJson   *bool
	fOut      *string
	fOutJson  *bool

	updateCmd     *kingpin.CmdClause
	fIn2          **os.File
	fIn264        *bool
	fIn2Json      *bool
	updateChannel *string
	updateEnv     *bool

	verifyCmd *kingpin.CmdClause

	signCmd     *kingpin.CmdClause
	signChannel *string
	signMspId   *string
	signMspDir  *string
}

func NewCli(app *kingpin.Application) (cli *Cli) {
	cli = new(Cli)
	cli.app = app

	cli.encodeCmd = app.Command("encode", "encode/decode between protobuf/json format")
	cli.encodeMsg = cli.encodeCmd.Flag("msg", "message name e.g. common.Block").Short('m').Required().String()
	cli.fIn = cli.encodeCmd.Flag("in", "input file. Read from STDIN if not specified.").Short('i').File()
	cli.fIn64 = cli.encodeCmd.Flag("i64", "use base64 to decode input first").Bool()
	cli.fInJson = cli.encodeCmd.Flag("ij", "use json format instead of protobuf").Bool()
	cli.fOut = cli.encodeCmd.Flag("out", "output file. Write to STDOUT if not specified.").Short('o').String()
	cli.fOutJson = cli.encodeCmd.Flag("oj", "use json format instead of protobuf").Bool()

	cli.updateCmd = app.Command("update", "compute update from configs")
	cli.updateCmd.Flag("original", "original Config").Short('1').FileVar(cli.fIn)
	cli.updateCmd.Flag("164", "use base64 to decode input first").BoolVar(cli.fIn64)
	cli.updateCmd.Flag("1j", "use json format instead of protobuf").BoolVar(cli.fInJson)
	cli.fIn2 = cli.updateCmd.Flag("updated", "updated Config").Short('2').File()
	cli.fIn264 = cli.updateCmd.Flag("264", "use base64 to decode input first").Bool()
	cli.fIn2Json = cli.updateCmd.Flag("2j", "use json format instead of protobuf").Bool()
	cli.updateChannel = cli.updateCmd.Flag("channel", "Channel ID").Required().Short('c').String()
	cli.updateEnv = cli.updateCmd.Flag("envelope", "output ConfigUpdate wrapped in Envelope").Short('e').Bool()
	cli.updateCmd.Flag("out", "output ConfigUpdate file. Write to STDOUT if not specified.").Short('o').StringVar(cli.fOut)
	cli.updateCmd.Flag("oj", "use json format instead of protobuf").BoolVar(cli.fOutJson)

	cli.verifyCmd = app.Command("verify", "Sanity check Config")
	cli.verifyCmd.Flag("in", "input Config. Read from STDIN if not specified.").Short('i').FileVar(cli.fIn)
	cli.verifyCmd.Flag("i64", "use base64 to decode input first").BoolVar(cli.fIn64)
	cli.verifyCmd.Flag("ij", "use json format instead of protobuf").BoolVar(cli.fInJson)
	cli.verifyCmd.Flag("out", "output json file. Write to STDOUT if not specified.").Short('o').StringVar(cli.fOut)

	cli.signCmd = app.Command("sign-config-update", "sign a ConfigUpdate Envelope. Signatures will be appended").Alias("sc")
	cli.signCmd.Flag("in", "input Envelope file. Read from STDIN if not specified.").Short('i').FileVar(cli.fIn)
	cli.signCmd.Flag("i64", "use base64 to decode input first").BoolVar(cli.fIn64)
	cli.signCmd.Flag("ij", "use json format instead of protobuf").BoolVar(cli.fInJson)
	cli.signChannel = cli.signCmd.Flag("channel", "Channel ID").Required().Short('c').String()
	cli.signMspId = cli.signCmd.Flag("mspid", "MSP ID used to sign").Required().String()
	cli.signMspDir = cli.signCmd.Flag("mspdir", "MSP directory path").Required().ExistingDir()
	cli.signCmd.Flag("out", "output Envelope file. Write to STDOUT if not specified.").Short('o').StringVar(cli.fOut)
	cli.signCmd.Flag("oj", "use json format instead of protobuf").BoolVar(cli.fOutJson)

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
			Input{Stdin: true, File: *cli.fIn, Json: *cli.fInJson, Base64: *cli.fIn64},
			Output{Stdout: true, File: cli.fOut, Json: *cli.fOutJson})

	case cli.updateCmd.FullCommand():
		cli.Update(
			Input{Stdin: false, File: *cli.fIn, Json: *cli.fInJson, Base64: *cli.fIn64},
			Input{Stdin: false, File: *cli.fIn2, Json: *cli.fIn2Json, Base64: *cli.fIn264},
			*cli.updateChannel,
			*cli.updateEnv,
			Output{Stdout: true, File: cli.fOut, Json: *cli.fOutJson})

	case cli.verifyCmd.FullCommand():
		cli.Verify(
			Input{Stdin: true, File: *cli.fIn, Json: *cli.fInJson, Base64: *cli.fIn64},
			Output{Stdout: true, File: cli.fOut, Json: true})

	case cli.signCmd.FullCommand():
		cli.Sign(
			Input{Stdin: true, File: *cli.fIn, Json: *cli.fInJson, Base64: *cli.fIn64},
			Output{Stdout: true, File: cli.fOut, Json: *cli.fOutJson},
			*cli.signChannel,
			*cli.signMspId,
			*cli.signMspDir)
	}
}

type Input struct {
	File   *os.File
	Json   bool
	Base64 bool
	Stdin  bool
}

func (in *Input) GetInput() *os.File {
	if in.File != nil {
		return in.File
	} else if in.Stdin {
		return os.Stdin
	}
	return nil
}

func (in *Input) GetReader() (io.Reader, error) {
	s := in.GetInput()
	if s == nil {
		return nil, Errorf("No input specified")
	}
	if in.Base64 {
		return base64.NewDecoder(base64.StdEncoding, in.GetInput()), nil
	} else {
		return in.GetInput(), nil
	}
}

func (in *Input) Unmarshal(pb proto.Message) error {
	r, err := in.GetReader()
	if err != nil {
		return err
	}
	if !in.Json {
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			if in.Base64 {
				return Errorf("Cannot read base64 input stream: %s", err)
			} else {
				return Errorf("Cannot read input stream: %s", err)
			}
		}
		err = proto.Unmarshal(buf, pb)
		if err != nil {
			return Errorf("Cannot unmarshal pb: %s", err)
		}
	} else {
		err := protolator.DeepUnmarshalJSON(r, pb)
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
