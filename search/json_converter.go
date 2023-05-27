package search

import (
	"encoding/json"
	"fmt"
)

type TokenConfig struct {
	Amount			string `json:"amount"`
	CustomData		string `json:"custom_data"`
	TokenFactoryID	string `json:"token_factory_id"`
}

type Issue struct {			
	Memo    		string `json:"memo"`
	To      		string `json:"to"`
	TokenConfigs	[]TokenConfig `json:"token_configs"`
}

func (c *TokenConfig) UnmarshalJSON(data []byte) error {
	type Alias TokenConfig // Create an alias type to avoid recursion

	aux := &struct {
		Amount 			json.RawMessage `json:"amount"`
		TokenFactoryID	json.RawMessage `json:"token_factory_id"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.Amount != nil {
		var amount int
		if err := json.Unmarshal(aux.Amount, &amount); err != nil {
			return err
		}
		c.Amount = fmt.Sprintf("%d", amount) // Convert the number to string
	}

	if aux.TokenFactoryID != nil {
		var tokenFactoryID int
		if err := json.Unmarshal(aux.TokenFactoryID, &tokenFactoryID); err != nil {
			return err
		}
		c.TokenFactoryID = fmt.Sprintf("%d", tokenFactoryID) // Convert the number to string
	}

	return nil
}

func convertNumericFieldsToStrings(normalized interface{}) (map[string]interface{}, error) {
	// convert to Issue struct value
	jsonData, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	var issue Issue
	if err = json.Unmarshal(jsonData, &issue); err != nil {
		return nil, err
	}

	// convert again, this time to map[string]interface{} value
	convertedJsonData, err := json.Marshal(issue)
	if err != nil {
		return nil, err
	}
	var converted map[string]interface{}
    if err = json.Unmarshal(convertedJsonData, &converted); err != nil {
		return nil, err
	}
	return converted, nil
}
