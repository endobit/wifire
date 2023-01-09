package wifire

import (
	"encoding/json"
	"io"
	"net/http"
)

type GetUserDataResponse struct {
	UserID         string  `json:"userId"`
	GivenName      string  `json:"givenName"`
	FamiltyName    string  `json:"familyName"`
	FullName       string  `json:"fullName"`
	Email          string  `json:"email"`
	Username       string  `json:"username"`
	Cognito        string  `json:"cognito"`
	CustomerID     string  `json:"customerId"`
	UrbanAirshipID string  `json:"urbanAirshipId"`
	Teams          []Team  `json:"teams"`
	Things         []Thing `json:"things"`
}

type Team struct {
	ID   string `json:"teamID"`
	Name string `json:"teamName"`
}

type Thing struct {
	Name         string     `json:"thingName"`
	FriendlyName string     `json:"friendlyName"`
	DeviceTypeID string     `json:"deviceTypeId"`
	UserID       string     `json:"userId"`
	Status       string     `json:"status"`
	ProductID    string     `json:"productId"`
	GrillModel   GrillModel `json:"grillModel"`
}

type GrillModel struct {
	ModuelNumber       string `json:"modelNumber"`
	Group              string `json:"group"`
	IOTCapable         bool   `json:"iotCapable"`
	Make               string `json:"make"`
	IsTraeger          bool   `json:"isTraegerBrand"`
	Region             string `json:"regionIso"`
	DeviceTypeID       string `json:"deviceTypeId"`
	Image              Image  `json:"image"`
	OwnersManualURL    string `json:"ownersManualUrl"`
	Name               string `json:"name"`
	Description        string `json:"description"`
	ReferenceProductID string `json:"referenceProductId"`
}

type Image struct {
	DefaultHost string `json:"defaultHost"`
	Endpoint    string `json:"endpoint"`
	Name        string `json:"name"`
}

func (w WiFire) UserData() (*GetUserDataResponse, error) {
	client := http.Client{}

	req, err := http.NewRequest("GET", w.config.baseURL+"/prod/users/self", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("authorization", w.token)

	r, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	resp, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	var data GetUserDataResponse

	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, err
	}

	return &data, nil
}
