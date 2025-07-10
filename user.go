package wifire

import (
	"encoding/json"
	"errors"
	"net/http"
)

type getUserDataResponse struct {
	UserID         string  `json:"userId"`
	GivenName      string  `json:"givenName"`
	FamiltyName    string  `json:"familyName"`
	FullName       string  `json:"fullName"`
	Email          string  `json:"email"`
	Username       string  `json:"username"`
	Cognito        string  `json:"cognito"`
	CustomerID     string  `json:"customerId"`
	UrbanAirshipID string  `json:"urbanAirshipId"`
	Teams          []team  `json:"teams"`
	Things         []thing `json:"things"`
}

type team struct {
	ID   string `json:"teamID"`
	Name string `json:"teamName"`
}

type thing struct {
	Name         string     `json:"thingName"`
	FriendlyName string     `json:"friendlyName"`
	DeviceTypeID string     `json:"deviceTypeId"`
	UserID       string     `json:"userId"`
	Status       string     `json:"status"`
	ProductID    string     `json:"productId"`
	GrillModel   grillModel `json:"grillModel"`
}

type grillModel struct {
	ModuelNumber       string `json:"modelNumber"`
	Group              string `json:"group"`
	IOTCapable         bool   `json:"iotCapable"`
	Make               string `json:"make"`
	IsTraeger          bool   `json:"isTraegerBrand"`
	Region             string `json:"regionIso"`
	DeviceTypeID       string `json:"deviceTypeId"`
	Image              image  `json:"image"`
	OwnersManualURL    string `json:"ownersManualUrl"`
	Name               string `json:"name"`
	Description        string `json:"description"`
	ReferenceProductID string `json:"referenceProductId"`
}

type image struct {
	DefaultHost string `json:"defaultHost"`
	Endpoint    string `json:"endpoint"`
	Name        string `json:"name"`
}

// UserData fetches the /prod/users/self information from the WiFire API.
func (w *WiFire) UserData() (*getUserDataResponse, error) {
	// response is read only user doesn't need to create a new struct
	client := http.Client{}

	req, err := http.NewRequest(http.MethodGet, w.config.baseURL+"/prod/users/self", http.NoBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("authorization", w.token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch user data")
	}

	var data getUserDataResponse

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	return &data, nil
}
