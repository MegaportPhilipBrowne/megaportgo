// Copyright 2020 Megaport Pty Ltd
//
// Licensed under the Mozilla Public License, Version 2.0 (the
// "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//       https://mozilla.org/MPL/2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// the `mcr` package is designed to allow you to orchestrate the creation of Megaport Cloud Routers. It provides
// complete lifecycle management of an MCR.
package mcr

import (
	"encoding/json"
	"errors"
	"io"
	"slices"
	"time"

	"github.com/megaport/megaportgo/config"
	"github.com/megaport/megaportgo/mega_err"
	"github.com/megaport/megaportgo/service/product"
	"github.com/megaport/megaportgo/shared"
	"github.com/megaport/megaportgo/types"
)

type MCR struct {
	*config.Config
	product *product.Product
}

// NewLocation
func New(cfg *config.Config) *MCR {
	return &MCR{
		Config:  cfg,
		product: product.New(cfg),
	}
}

// BuyMCR purchases an MCR.
func (m *MCR) BuyMCR(locationID int, name string, term int, portSpeed int, mcrASN int) (string, error) {
	orderConfig := types.MCROrderConfig{}

	if term != 1 && term != 12 && term != 24 && term != 36 {
		return "", errors.New(mega_err.ERR_TERM_NOT_VALID)
	}

	if mcrASN != 0 {
		orderConfig.ASN = mcrASN
	}

	if portSpeed != 1000 && portSpeed != 2500 && portSpeed != 5000 && portSpeed != 10000 {
		return "", errors.New(mega_err.ERR_MCR_INVALID_PORT_SPEED)
	}

	order := []types.MCROrder{
		{
			LocationID: locationID,
			Name:       name,
			Term:       term,
			Type:       "MCR2",
			PortSpeed:  portSpeed,
			Config:     orderConfig,
		},
	}

	requestBody, marshalErr := json.Marshal(order)

	if marshalErr != nil {
		return "", marshalErr
	}

	body, resErr := m.product.ExecuteOrder(&requestBody)

	if resErr != nil {
		return "", resErr
	}

	orderInfo := types.MCROrderResponse{}
	unmarshalErr := json.Unmarshal(*body, &orderInfo)

	if unmarshalErr != nil {
		return "", unmarshalErr
	}

	return orderInfo.Data[0].TechnicalServiceUID, nil
}

// CreatePrefixFilterList creates a Prefix Filter List on an MCR.
func (m *MCR) CreatePrefixFilterList(id string, prefixFilterList types.MCRPrefixFilterList) (bool, error) {
	prefix, prefixErr := m.product.CreateMCRPrefixFilterList(id, prefixFilterList)
	return prefix, prefixErr
}

// GetMCRDetails get the details of an MCR.
func (m *MCR) GetMCRDetails(id string) (types.MCR, error) {
	url := "/v2/product/" + id
	response, err := m.Config.MakeAPICall("GET", url, nil)
	defer response.Body.Close()

	isError, parsedError := m.Config.IsErrorResponse(response, &err, 200)

	if isError {
		return types.MCR{}, parsedError
	}

	body, fileErr := io.ReadAll(response.Body)

	if fileErr != nil {
		return types.MCR{}, fileErr
	}

	portDetails := types.MCRResponse{}
	unmarshalErr := json.Unmarshal(body, &portDetails)

	if unmarshalErr != nil {
		return types.MCR{}, unmarshalErr
	}

	return portDetails.Data, nil
}

// ModifyMCR modifies an MCR.
func (m *MCR) ModifyMCR(mcrId string, name string, costCentre string, marketplaceVisibility bool) (bool, error) {
	return m.product.ModifyProduct(mcrId, types.PRODUCT_MCR, name, costCentre, marketplaceVisibility)
}

// ModifyMCR deletes an MCR.
func (m *MCR) DeleteMCR(id string, deleteNow bool) (bool, error) {
	return m.product.DeleteProduct(id, deleteNow)
}

// ModifyMCR un-deletes an MCR.
func (m *MCR) RestoreMCR(id string) (bool, error) {
	return m.product.RestoreProduct(id)
}

// DebugWaitMCRLive should be used for testing only.
func (m *MCR) WaitForMcrProvisioning(mcrId string) (bool, error) {
	// Try for ~5mins.
	for i := 0; i < 30; i++ {
		details, err := m.GetMCRDetails(mcrId)
		if err != nil {
			return false, err
		}

		if slices.Contains(shared.SERVICE_STATE_READY, details.ProvisioningStatus) {
			return true, nil
		}

		// Wrong status, wait a bit and try again.
		m.Log.Debugf("MVE status is %q - waiting", details.ProvisioningStatus)
		time.Sleep(10 * time.Second)
	}

	return false, errors.New(mega_err.ERR_MCR_PROVISION_TIMEOUT_EXCEED)
}
