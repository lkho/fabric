package sign

import (
	"github.com/hyperledger/fabric/msp/mgmt"
	"github.com/hyperledger/fabric/bccsp/factory"
	"github.com/hyperledger/fabric/common/localmsp"
	cb "github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/common/crypto"
	"fmt"
	"github.com/hyperledger/fabric/protos/utils"
	"github.com/hyperledger/fabric/common/util"
)

func SignConfigUpdateEnvelope(mspID string, mspDir string, env *cb.ConfigUpdateEnvelope) error {
	var signer crypto.LocalSigner
	if mspDir != "" {
		mgmt.LoadLocalMsp(mspDir, factory.GetDefaultOpts(), mspID)
		signer = localmsp.NewSigner()
	} else {
		signer = nil
	}

	if signer != nil {
		sigHeader, err := signer.NewSignatureHeader()

		if err != nil {
			return fmt.Errorf("Bad signer: %s", err)
		}

		configSig := &cb.ConfigSignature{
			SignatureHeader: utils.MarshalOrPanic(sigHeader),
		}

		configSig.Signature, err = signer.Sign(util.ConcatenateBytes(configSig.SignatureHeader, env.ConfigUpdate))
		if err != nil {
			return fmt.Errorf("Error signing config update: %s", err)
		}

		env.Signatures = append(env.Signatures, configSig)
	}
	return nil
}
