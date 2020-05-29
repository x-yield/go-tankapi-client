package tankapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	createBreakpoint  = "init"
	prepareBreakpoint = "start"
)

var netTransport = &http.Transport{
	Dial: (&net.Dialer{
		Timeout: 5 * time.Second,
	}).Dial,
	TLSHandshakeTimeout: 5 * time.Second,
}

var netClient = &http.Client{
	Transport: netTransport,
	Timeout:   time.Second * 10,
}

type Session struct {
	Tank     *Tank
	Config   *Config
	Name     string
	Failures []string
	Stage    string
	Status   string
}

func NewSession(tank, config string) *Session {
	return &Session{
		Tank:   &Tank{Url: tank},
		Config: &Config{Contents: config},
	}
}

type Config struct {
	Contents string
}

// validate - goroutine that validates config for single tank
func (s *Session) validate() (err error) {
	s.Stage = "validation"
	s.Failures = []string{}
	err = s.checkTank()
	if err != nil {
		return
	}
	err = s.checkConfig()
	if err != nil {
		return
	}
	resp, err := netClient.Post(fmt.Sprintf("%v/validate", s.Tank.Url), "application/yaml", bytes.NewReader([]byte(s.Config.Contents)))
	if err != nil {
		err = errors.New(fmt.Sprintf("http.POST failed: %v", err))
		log.Println(err)
		s.setFailed([]string{err.Error()})
		return
	}
	defer resp.Body.Close()
	respBody, err := checkResponseCode(*resp)
	if err != nil {
		log.Println(err)
		s.setFailed([]string{err.Error()})
		return
	}
	var respJson map[string]interface{}
	err = json.Unmarshal(respBody, &respJson)
	if err != nil {
		err = errors.New(fmt.Sprintf("failed to unmarshal tank response body into json: %v", err))
		log.Println(err)
		s.setFailed([]string{err.Error()})
		return
	}
	validationErrors := respJson["errors"]
	switch validationErrors := validationErrors.(type) {
	case []interface{}:
		if len(validationErrors) > 0 {
			var e []string
			for _, v := range validationErrors {
				e = append(e, fmt.Sprintf("%v", v))
			}
			err = errors.New(fmt.Sprintf("session config is invalid %v", strings.Join(e, "\n")))
			log.Println(err)
			s.setFailed(e)
		}
		return
	case map[string]interface{}:
		if len(validationErrors) > 0 {
			var e []string
			for k, v := range validationErrors {
				e = append(e, fmt.Sprintf("%v: %v", k, v))
			}
			err = errors.New(fmt.Sprintf("session config is invalid %v", strings.Join(e, "\n")))
			log.Println(err)
			s.setFailed(e)
		}
		return
	case nil:
		return
	default:
		err = errors.New(fmt.Sprintf("unexpected tank validation response: %T", validationErrors))
		log.Println(err)
		s.setFailed([]string{err.Error()})
		return
	}
}

// create - creates tankapi session and acquires tank lock
func (s *Session) create() (err error) {
	err = s.checkTank()
	if err != nil {
		return
	}
	err = s.checkConfig()
	if err != nil {
		return
	}
	resp, err := netClient.Post(fmt.Sprintf("%v/run?break=%v", s.Tank.Url, createBreakpoint), "application/yaml", bytes.NewReader([]byte(s.Config.Contents)))
	if err != nil {
		log.Printf("http.POST failed: %v", err)
		s.setFailed([]string{fmt.Sprintf("http.POST failed: %v", err)})
		return
	}
	defer resp.Body.Close()
	respBody, err := checkResponseCode(*resp)
	if err != nil {
		log.Println(err)
		s.setFailed([]string{err.Error()})
		return
	}
	var respJson map[string]interface{}
	err = json.Unmarshal(respBody, &respJson)
	if err != nil {
		err = errors.New(fmt.Sprintf("failed to unmarshal tank response body into json: %v", err))
		log.Println(err)
		s.setFailed([]string{err.Error()})
		return
	}
	sessionName := respJson["session"]
	switch sessionName := sessionName.(type) {
	case string:
		s.Name = sessionName
	case nil:
		err = errors.New("failed to create session, try validating your config")
		log.Println(err)
		s.setFailed([]string{err.Error()})
		return
	default:
		err = errors.New(fmt.Sprintf("unexpected tank session creation response: %T", sessionName))
		log.Println(err)
		s.setFailed([]string{err.Error()})
		return
	}
	failed, failures := s.isFailed()
	if failed {
		err = errors.New(fmt.Sprintf("preparing session %v@%v failed %v", s.Name, s.Tank.Url, s.Failures))
		log.Println(err)
		s.setFailed(failures)
	}
	return
}

// prepare - goroutine that prepares single tank, checks if failed.
// if session has no name yet, starts a new one with "start" breakpoint
func (s *Session) prepare() (err error) {
	err = s.checkTank()
	if err != nil {
		return
	}
	if !s.hasName() {
		err = s.create()
		if err != nil {
			return
		}
		fmt.Println(s.Name)
	}
	resp, err := netClient.Get(fmt.Sprintf("%v/run?session=%v&break=%v", s.Tank.Url, s.Name, prepareBreakpoint))
	if err != nil {
		log.Printf("http.POST failed: %v", err)
		s.setFailed([]string{fmt.Sprintf("http.POST failed: %v", err)})
		return
	}
	defer resp.Body.Close()
	_, err = checkResponseCode(*resp)
	if err != nil {
		log.Println(err)
		s.setFailed([]string{err.Error()})
		return
	}
	failed, failures := s.isFailed()
	if failed {
		err = errors.New(fmt.Sprintf("preparing session %v@%v failed %v", s.Name, s.Tank.Url, s.Failures))
		log.Println(err)
		s.setFailed(failures)
	}
	return
}

// run - sends starting request, checks if failed.
// if session has no name yet, starts a new one with no breakpoint
func (s *Session) run() (err error) {
	err = s.checkTank()
	if err != nil {
		return
	}
	if !s.hasName() {
		err = s.create()
		if err != nil {
			return
		}
	}
	resp, err := netClient.Get(fmt.Sprintf("%v/run?session=%v", s.Tank.Url, s.Name))
	if err != nil {
		log.Printf("http.POST failed: %v", err)
		return fmt.Errorf("http.POST failed: %v", err)
	}
	defer resp.Body.Close()
	_, err = checkResponseCode(*resp)
	if err != nil {
		log.Println(err)
		s.setFailed([]string{err.Error()})
		return
	}
	failed, failures := s.isFailed()
	if failed {
		err = errors.New(fmt.Sprintf("starting session %v@%v failed %v", s.Name, s.Tank.Url, s.Failures))
		log.Println(err)
		s.setFailed(failures)
		return
	}
	return nil
}

// stop - sends finishing request, checks if failed.
func (s *Session) stop() (err error) {
	if s.Tank.Url == "" || s.Tank == nil {
		err = errors.New("session needs to have a tank")
		log.Println(err)
		s.setFailed([]string{err.Error()})
		return
	}
	if s.Name == "" {
		err = errors.New("session has to have a name to stop")
		log.Println(err)
		s.setFailed([]string{err.Error()})
		return
	}
	resp, err := netClient.Get(fmt.Sprintf("%v/stop?session=%v", s.Tank.Url, s.Name))
	if err != nil {
		err = errors.New(fmt.Sprintf("http.POST failed: %v", err))
		log.Println(err)
		s.setFailed([]string{err.Error()})
		return
	}
	defer resp.Body.Close()
	_, err = checkResponseCode(*resp)
	if err != nil {
		log.Println(err)
		s.setFailed([]string{err.Error()})
		return
	}
	//wait for session to reach "finished" stage
	failed, failures := s.isFailed()
	if failed {
		s.setFailed(failures)
		return errors.New(fmt.Sprintf("stopping session %v@%v failed %v", s.Name, s.Tank.Url, s.Failures))
	}
	return nil
}

// poll - sends finishing request and waits for test to actually finish, checks if failed.
func (s *Session) poll() (err error) {
	_, err = s.getStatus()
	if err != nil {
		return
	}
	failed, failures := s.isFailed()
	if failed {
		s.setFailed(failures)
		return errors.New(fmt.Sprintf("stopping session %v@%v failed %v", s.Name, s.Tank.Url, s.Failures))
	}
	return nil
}

// getStatus - returns tankapi session status
// sets current session stage
func (s *Session) getStatus() (map[string]interface{}, error) {
	var dummyMap = make(map[string]interface{})
	err := s.checkTank()
	if err != nil {
		return dummyMap, err
	}
	err = s.checkName()
	if err != nil {
		return dummyMap, err
	}
	resp, err := netClient.Get(fmt.Sprintf("%v/status?session=%v", s.Tank.Url, s.Name))
	if err != nil {
		s.Status = "disconnect"
		return dummyMap, err
	}
	defer resp.Body.Close()
	respBody, err := checkResponseCode(*resp)
	if err != nil {
		log.Println(err)
		s.setFailed([]string{err.Error()})
		return dummyMap, err
	}
	var respJson map[string]interface{}
	err = json.Unmarshal(respBody, &respJson)
	if err != nil {
		return dummyMap, err
	}
	switch stage := respJson["current_stage"].(type) {
	case string:
		s.Stage = stage
	}
	switch status := respJson["status"].(type) {
	case string:
		s.Status = status
	}
	return respJson, nil
}

func (s *Session) isPrepared() bool {
	status, err := s.getStatus()
	if err != nil {
		return false
	}
	if status["current_stage"] == "prepare" && status["stage_completed"] == true {
		return true
	} else {
		return false
	}
}

func (s *Session) isRunning() bool {
	status, err := s.getStatus()
	if err != nil {
		return false
	}
	if status["current_stage"] == "poll" && status["stage_completed"] == false {
		return true
	} else {
		return false
	}
}

func (s *Session) isFinished() bool {
	status, err := s.getStatus()
	if err != nil {
		return false
	}
	if status["current_stage"] == "finished" && status["stage_completed"] == true {
		return true
	} else {
		return false
	}
}

func (s *Session) isFailed() (bool, []string) {
	sessionStatus, err := s.getStatus()
	if err != nil {
		log.Println(err)
		return true, []string{err.Error()}
	}

	failures := sessionStatus["failures"]
	switch failures := failures.(type) {
	case []interface{}:
		if len(failures) > 0 {
			fmt.Println(failures)
			var e []string
			for _, f := range failures {
				switch f := f.(type) {
				case map[string]interface{}:
					switch r := f["reason"].(type) {
					case string:
						e = append(e, r)
					}
				}
			}
			return true, e
		}
	case nil:
	default:

		log.Printf("unexpected tank failures response; expected string array, got: %T", failures)
	}
	return false, []string{}
}

// setFailed - sets status == "failed" and appends failure to failures list
// so that Failures store the "history" of failures
func (s *Session) setFailed(failures []string) {
	s.Failures = append(s.Failures, failures...)
	s.Status = "failed"
}

func (s *Session) checkTank() (err error) {
	if s.Tank.Url == "" || s.Tank == nil {
		err = errors.New("session needs to have a tank")
		log.Println(err)
		s.setFailed([]string{err.Error()})
		return
	}
	return nil
}

func (s *Session) checkConfig() (err error) {
	if !s.hasConfig() {
		err = errors.New("no config provided for validation")
		log.Println(err)
		s.setFailed([]string{err.Error()})
		return
	}
	return nil
}

func (s *Session) checkName() (err error) {
	if !s.hasName() {
		err = errors.New("session has to have a name to run or be polled")
		log.Println(err)
		s.setFailed([]string{err.Error()})
		return
	}
	return nil
}

func (s Session) hasTank() bool {
	if s.Tank == nil || s.Tank.Url == "" {
		return false
	}
	return true
}

func (s Session) hasName() bool {
	if s.Name == "" {
		return false
	}
	return true
}

func (s Session) hasConfig() bool {
	if s.Config == nil || s.Config.Contents == "" {
		return false
	}
	return true
}

// checkResponseCode - checks if status code is 200, otherwise returns a corresponding error
// also returns resp body
// DOES NOT CLOSE RESP BODY
func checkResponseCode(resp http.Response) (respBody []byte, err error) {
	respBody, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		err = errors.New(fmt.Sprintf("failed to read tank response: %v %v", resp.StatusCode, err))
		return
	}
	if resp.StatusCode != 200 {
		err = errors.New(fmt.Sprintf("%d: %v", resp.StatusCode, string(respBody)))
		return
	}
	return
}
