/* This file contains all functions related to using the Blizzard API */

package blizzard

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/FuzzyStatic/blizzard/v3"
	"github.com/FuzzyStatic/blizzard/v3/wowsearch"

	config "github.com/bezerker/sndbot/config"
	util "github.com/bezerker/sndbot/util"
)

var clientID string
var clientSecret string

func RealmSearch() string {
	usBlizzClient, err := blizzard.NewClient(blizzard.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		HTTPClient:   http.DefaultClient,
		Region:       blizzard.US,
		Locale:       blizzard.EnUS,
	})
	if err != nil {
		panic(err)
	}

	realmSearch, _, err := usBlizzClient.ClassicWoWRealmSearch(
		context.TODO(),
		wowsearch.Page(1),
		wowsearch.PageSize(5),
		wowsearch.OrderBy("name.EN_US:asc"),
		wowsearch.Field().
			AND("timezone", "Europe/Paris").
			AND("data.locale", "enGB").
			NOT("type.type", "PVP").
			NOT("id", "4756||4757").
			OR("type.type", "NORMAL", "RP"),
	)
	if err != nil {
		panic(err)
	}

	out, err := json.MarshalIndent(realmSearch, "", "  ")
	if err != nil {
		panic(err)
	}

	return string(out[:])
}

func RealmList() string {
	usBlizzClient, err := blizzard.NewClient(blizzard.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		HTTPClient:   http.DefaultClient,
		Region:       blizzard.US,
		Locale:       blizzard.EnUS,
	})
	util.CheckNilErr(err)

	realmList, _, err := usBlizzClient.ClassicWoWRealmIndex(context.TODO())
	util.CheckNilErr(err)
	out, err := json.MarshalIndent(realmList, "", "  ")
	util.CheckNilErr(err)

	return GetRealmNames(string(out))

}

func GetRealmNames(realmList string) string {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(realmList), &data); err != nil {
		panic(err)
	}

	realms := data["realms"].([]interface{})
	names := make([]string, len(realms))
	for i, realm := range realms {
		names[i] = realm.(map[string]interface{})["name"].(string)
	}

	return strings.Join(names, "\n")
}

func init() {
	config := config.ReadConfig("config.yaml")
	clientID = config.BlizzardClientID
	clientSecret = config.BlizzardClientSecret
}
