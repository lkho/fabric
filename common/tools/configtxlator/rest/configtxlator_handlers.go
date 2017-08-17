/*
Copyright IBM Corp. 2017 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

                 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/hyperledger/fabric/common/tools/configtxlator/sanitycheck"
	"github.com/hyperledger/fabric/common/tools/configtxlator/update"
	cb "github.com/hyperledger/fabric/protos/common"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/configtx"
	"github.com/hyperledger/fabric/protos/utils"
	"github.com/hyperledger/fabric/msp/mgmt"
	"github.com/hyperledger/fabric/bccsp/factory"
	"github.com/hyperledger/fabric/common/localmsp"
	"github.com/hyperledger/fabric/common/crypto"
	"github.com/hyperledger/fabric/common/util"
)

func fieldBytes(fieldName string, r *http.Request) ([]byte, error) {
	fieldFile, _, err := r.FormFile(fieldName)
	if err != nil {
		return nil, err
	}
	defer fieldFile.Close()

	return ioutil.ReadAll(fieldFile)
}

func fieldConfigProto(fieldName string, r *http.Request) (*cb.Config, error) {
	fieldBytes, err := fieldBytes(fieldName, r)
	if err != nil {
		return nil, fmt.Errorf("error reading field bytes: %s", err)
	}

	config := &cb.Config{}
	err = proto.Unmarshal(fieldBytes, config)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling field bytes: %s", err)
	}

	return config, nil
}

func ComputeUpdateFromConfigs(w http.ResponseWriter, r *http.Request) {
	originalConfig, err := fieldConfigProto("original", r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error with field 'original': %s\n", err)
		return
	}

	updatedConfig, err := fieldConfigProto("updated", r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error with field 'updated': %s\n", err)
		return
	}

	configUpdate, err := update.Compute(originalConfig, updatedConfig)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error computing update: %s\n", err)
		return
	}

	configUpdate.ChannelId = r.FormValue("channel")

	encoded, err := proto.Marshal(configUpdate)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error marshaling config update: %s\n", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(encoded)
}

func SanityCheckConfig(w http.ResponseWriter, r *http.Request) {
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, err)
		return
	}

	config := &cb.Config{}
	err = proto.Unmarshal(buf, config)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error unmarshaling data to common.Config': %s\n", err)
		return
	}

	fmt.Printf("Sanity checking %+v\n", config)
	sanityCheckMessages, err := sanitycheck.Check(config)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error performing sanity check: %s\n", err)
		return
	}

	resBytes, err := json.Marshal(sanityCheckMessages)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error marshaling result to JSON: %s\n", err)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(resBytes)
}

func signingConfigUpdateEnv(w http.ResponseWriter, r *http.Request, env *cb.ConfigUpdateEnvelope) ([]byte, error) {
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

func SignConfigUpdateEnvelope(w http.ResponseWriter, r *http.Request) {
	env, err := fieldBytes("configUpdateEnvelope", r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Bad Config Update Envelope: %s\n", err)
		return
	}

	configUpdateEnvelope, err := configtx.UnmarshalConfigUpdateEnvelope(env)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error unmarshaling config update envelope: %s\n", err)
		return
	}

	encoded, err := signingConfigUpdateEnv(w, r, configUpdateEnvelope)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error marshaling config update envelope: %s\n", err)
		return
	}

	channelID := r.FormValue("channelID")
	payloadChannelHeader := utils.MakeChannelHeader(cb.HeaderType_CONFIG_UPDATE, 0, channelID, 0)
	payloadHeader := &cb.Header{}
	payloadHeader.ChannelHeader = utils.MarshalOrPanic(payloadChannelHeader)

	payloadBytes := utils.MarshalOrPanic(&cb.Payload{
		Header: payloadHeader,
		Data:   encoded,
	})

	envelope := &cb.Envelope{
		Payload: payloadBytes,
	}

	encoded = utils.MarshalOrPanic(envelope)

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(encoded)
}

func SignConfigUpdate(w http.ResponseWriter, r *http.Request) {
	updateConfig, err := fieldBytes("configUpdate", r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Bad Config Update: %s\n", err)
		return
	}

	configUpdate, err := configtx.UnmarshalConfigUpdate(updateConfig)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error unmarshaling config update: %s\n", err)
		return
	}

	configUpdateEnvelope := &cb.ConfigUpdateEnvelope{
		ConfigUpdate: utils.MarshalOrPanic(configUpdate),
	}

	encoded, err := signingConfigUpdateEnv(w, r, configUpdateEnvelope)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error marshaling config update envelope: %s\n", err)
		return
	}

	channelID := r.FormValue("channelID")
	payloadChannelHeader := utils.MakeChannelHeader(cb.HeaderType_CONFIG_UPDATE, 0, channelID, 0)
	payloadHeader := &cb.Header{}
	payloadHeader.ChannelHeader = utils.MarshalOrPanic(payloadChannelHeader)

	payloadBytes := utils.MarshalOrPanic(&cb.Payload{
		Header: payloadHeader,
		Data:   encoded,
	})

	envelope := &cb.Envelope{
		Payload: payloadBytes,
	}

	encoded = utils.MarshalOrPanic(envelope)

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(encoded)
}
