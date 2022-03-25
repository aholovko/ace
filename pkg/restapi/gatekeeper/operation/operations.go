/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package operation

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/hyperledger/aries-framework-go/pkg/common/log"
	vdrapi "github.com/hyperledger/aries-framework-go/pkg/framework/aries/api/vdr"
	"github.com/hyperledger/aries-framework-go/spi/storage"

	"github.com/trustbloc/ace/pkg/client/vault"
	"github.com/trustbloc/ace/pkg/internal/common/support"
	"github.com/trustbloc/ace/pkg/restapi/gatekeeper/operation/models"
	"github.com/trustbloc/ace/pkg/restapi/gatekeeper/operation/vcprovider"
	"github.com/trustbloc/ace/pkg/restapi/model"
	"github.com/trustbloc/ace/pkg/store/policy"
	"github.com/trustbloc/ace/pkg/store/protecteddata"
)

var logger = log.New("gatekeeper")

// API endpoints.
const (
	policyIDVarName = "policy_id"
	baseV1Path      = "/v1"
	protectEndpoint = baseV1Path + "/protect"
	policyEndpoint  = baseV1Path + "/policy/{" + policyIDVarName + "}"
)

// Config defines configuration for Gatekeeper operations.
type Config struct {
	StorageProvider storage.Provider
	VaultClient     vault.Vault
	VDRI            vdrapi.Registry
	VCProvider      vcprovider.Provider
}

// New returns a new Operation instance.
func New(config *Config) (*Operation, error) {
	protectedDataStore, err := protecteddata.New(config.StorageProvider)
	if err != nil {
		return nil, err
	}

	policyStore, err := policy.New(config.StorageProvider)
	if err != nil {
		return nil, err
	}

	protectOp := NewProtectOp(&ProtectConfig{
		Store:       protectedDataStore,
		VaultClient: config.VaultClient,
		VDRI:        config.VDRI,
		VCProvider:  config.VCProvider,
	})

	return &Operation{
		protectOperation: protectOp,
		policyStore:      policyStore,
	}, nil
}

// Operation defines handlers for rp operations.
type Operation struct {
	protectOperation ProtectOperation
	policyStore      policy.Repository
}

// GetRESTHandlers get all controller API handler available for this service.
func (o *Operation) GetRESTHandlers() []support.Handler {
	return []support.Handler{
		support.NewHTTPHandler(protectEndpoint, http.MethodPost, o.protectHandler),
		support.NewHTTPHandler(policyEndpoint, http.MethodPut, o.createPolicyHandler),
	}
}

func (o *Operation) protectHandler(rw http.ResponseWriter, r *http.Request) {
	req := &models.ProtectReq{}

	err := json.NewDecoder(r.Body).Decode(req)
	if err != nil {
		respondError(rw, http.StatusBadRequest, err)

		return
	}

	response, err := o.protectOperation.ProtectOp(req)
	if err != nil {
		respondError(rw, http.StatusInternalServerError, err)

		return
	}

	respond(rw, http.StatusOK, response)
}

func (o *Operation) createPolicyHandler(rw http.ResponseWriter, r *http.Request) {
	doc := model.PolicyDocument{}

	err := json.NewDecoder(r.Body).Decode(&doc)
	if err != nil {
		respondError(rw, http.StatusBadRequest, err)

		return
	}

	policyID := strings.ToLower(mux.Vars(r)[policyIDVarName])

	err = o.policyStore.Put(policyID, &doc)
	if err != nil {
		respondError(rw, http.StatusInternalServerError, fmt.Errorf("store policy: %w", err))

		return
	}

	respond(rw, http.StatusOK, nil)
}

func respond(w http.ResponseWriter, statusCode int, payload interface{}) {
	w.Header().Add("Content-Type", "application/json")

	w.WriteHeader(statusCode)

	if payload != nil {
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			logger.Errorf("failed to write response: %s", err.Error())
		}
	}
}

func respondError(w http.ResponseWriter, statusCode int, err error) {
	w.Header().Add("Content-Type", "application/json")

	errorMessage := err.Error()

	logger.Errorf(errorMessage)

	w.WriteHeader(statusCode)

	if encErr := json.NewEncoder(w).Encode(&model.ErrorResponse{Message: errorMessage}); encErr != nil {
		logger.Errorf("failed to write error response: %s", err.Error())
	}
}
