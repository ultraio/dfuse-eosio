package search

import (
	"encoding/json"
	"strconv"
)

type TokenConfig struct {
	TokenFactoryID string `json:"token_factory_id"`
}

type Issue struct {
	To           string        `json:"to"`
	TokenConfigs []TokenConfig `json:"token_configs"`
}

type Transfer struct {
	From    string   `json:"from"`
	To      string   `json:"to"`
	TokenID []string `json:"token_ids"`
}

type Buy struct {
	Buyer      string `json:"buyer"`
	Receiver   string `json:"receiver"`
	TokenID    string `json:"token_id"`
	PromoterID string `json:"promoter_id"`
}

var CheckNFTField = []string{
	"issue",
	"transfer",
	"buy"}

func (c *TokenConfig) UnmarshalJSON(data []byte) error {
	type Alias TokenConfig // Create an alias type to avoid recursion

	aux := &struct {
		TokenFactoryID uint64 `json:"token_factory_id"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Convert to string
	c.TokenFactoryID = strconv.FormatUint(aux.TokenFactoryID, 10)

	return nil
}

func (t *Transfer) UnmarshalJSON(data []byte) error {
	type Alias Transfer

	// Create a secondary struct with the same fields
	// but with Ages as []int for unmarshalling
	aux := &struct {
		TokenIDs []uint64 `json:"token_ids"`
		*Alias
	}{
		Alias: (*Alias)(t),
	}

	// Unmarshal into the secondary struct
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Convert to a slice of string
	if aux.TokenIDs != nil {
		t.TokenID = make([]string, len(aux.TokenIDs))
		for i, id := range aux.TokenIDs {
			t.TokenID[i] = strconv.FormatUint(id, 10)
		}
	}

	return nil
}

func (t *Buy) UnmarshalJSON(data []byte) error {
	type Alias Buy

	// Create a secondary struct with the same fields
	// but with Ages as []int for unmarshalling
	aux := &struct {
		TokenID uint64 `json:"token_id"`
		*Alias
	}{
		Alias: (*Alias)(t),
	}

	// Unmarshal into the secondary struct
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Convert to string
	t.TokenID = strconv.FormatUint(aux.TokenID, 10)

	return nil
}

func convertToMap(jsonData []byte, v interface{}) (map[string]interface{}, error) {
	// Unmarshal the JSON data into the provided struct
	if err := json.Unmarshal(jsonData, v); err != nil {
		return nil, err
	}

	// Marshal the struct back to JSON
	convertedJsonData, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	// Unmarshal the JSON data into a map
	var result map[string]interface{}
	if err := json.Unmarshal(convertedJsonData, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func tokenizeNFTDataField(normalizedField string, normalizedValue interface{}) (map[string]interface{}, error) {

	var shouldSkip = true
	for _, field := range CheckNFTField {
		if normalizedField == field {
			shouldSkip = false
			break
		}
	}
	// skip if it's not NFT supported field
	if shouldSkip {
		return nil, nil
	}

	var result map[string]interface{}
	var err error

	jsonData, err := json.Marshal(normalizedValue)
	if err != nil {
		return nil, err
	}

	//Convert json data to correct struct
	switch normalizedField {
	case "issue":
		var issue Issue
		result, err = convertToMap(jsonData, &issue)
	case "transfer":
		var transfer Transfer
		result, err = convertToMap(jsonData, &transfer)
	case "buy":
		var buy Buy
		result, err = convertToMap(jsonData, &buy)
	default:
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}
