/* This file contains all functions related to using the Blizzard API */

package blizzard

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/FuzzyStatic/blizzard/v3"

	config "github.com/bezerker/sndbot/config"
	util "github.com/bezerker/sndbot/util"
)

var clientID string
var clientSecret string

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

func GuildMemberLookup(guildName string, realm string) string {
	usBlizzClient, err := blizzard.NewClient(blizzard.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		HTTPClient:   http.DefaultClient,
		Region:       blizzard.US,
		Locale:       blizzard.EnUS,
	})
	util.CheckNilErr(err)

	guildMembers, _, err := usBlizzClient.WoWGuildRoster(context.TODO(), realm, guildName)
	util.CheckNilErr(err)
	out, err := json.MarshalIndent(guildMembers, "", "  ")
	util.CheckNilErr(err)

	return string(out[:])
}

func init() {
	config := config.ReadConfig("config.yaml")
	clientID = config.BlizzardClientID
	clientSecret = config.BlizzardClientSecret
}
