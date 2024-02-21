/* This file contains all functions related to using the Blizzard API */

package blizzard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/FuzzyStatic/blizzard/v3"
	"github.com/FuzzyStatic/blizzard/v3/wowsearch"

	config "github.com/bezerker/sndbot/config"
)

var clientID string
var clientSecret string

func TestRealmSearch() {
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

	fmt.Println(string(out[:]))
}

func init() {
	config := config.ReadConfig("config.yaml")
	clientID = config.BlizzardClientID
	clientSecret = config.BlizzardClientSecret
}
