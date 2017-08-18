package sign

import (
	"github.com/hyperledger/fabric/msp/mgmt"
	"github.com/hyperledger/fabric/bccsp/factory"
	"github.com/hyperledger/fabric/common/localmsp"
	cb "github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/common/crypto"
	"github.com/hyperledger/fabric/protos/utils"
	"github.com/hyperledger/fabric/common/util"
	"net/http"
	"github.com/golang/protobuf/proto"
)

func SigningConfigUpdateEnv(w http.ResponseWriter, r *http.Request, env *cb.ConfigUpdateEnvelope) ([]byte, error) {
	mspID := r.FormValue("mspID")
	mspDir := r.FormValue("mspDir")

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
			/*w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Bad signer :%s\n", err)*/
			return nil, err
		}

		configSig := &cb.ConfigSignature{
			SignatureHeader: utils.MarshalOrPanic(sigHeader),
		}

		configSig.Signature, err = signer.Sign(util.ConcatenateBytes(configSig.SignatureHeader, env.ConfigUpdate))
		if err != nil {
			/*w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error signing config update: %s\n", err)*/
			return nil, err
		}

		env.Signatures = append(env.Signatures, configSig)
	}

	return proto.Marshal(env)
}
