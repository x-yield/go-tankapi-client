package tankapi

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

// Tank - represents tank api server; might have several sessions simultaneously
type Tank struct {
	Url string
}

// Sessions - gets all sessions on a tank with current statuses
func (t Tank) Sessions() (sessions []Session, err error) {
	resp, err := netClient.Get(fmt.Sprintf("%v/status", t.Url))
	if err != nil {
		err = fmt.Errorf("http.GET status failed: %w", err)
		return
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("fail to read response http.GET status body: %w", err)
		return
	}

	var respJson map[string]interface{}
	err = json.Unmarshal(respBody, &respJson)
	if err != nil {
		err = fmt.Errorf("fail to unmarshal response http.GET status body: %w", err)
		return
	}
	if len(respJson) > 0 {
		for k, v := range respJson {
			s := Session{
				Tank: &t,
				Name: k,
			}
			switch v := v.(type) {
			case map[string]interface{}:
				status := v["status"]
				switch status := status.(type) {
				case string:
					s.Status = status
				}
			}
			sessions = append(sessions, s)
		}
	}
	return
}
