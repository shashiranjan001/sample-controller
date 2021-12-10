package cloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gotway/gotway/pkg/env"
	log "github.com/sirupsen/logrus"
	"k8s.io/sample-controller/pkg/metrics"
)

const (
	reqPOST   = "POST"
	reqGET    = "GET"
	reqDELETE = "DELETE"
)

// cloudAPIURL is the REST API endpoint of the cloud.
var cloudAPIURL string

func init() {
	cloudAPIURL = env.Get("CLOUD_API_URL", "http://127.0.0.1:9001")
}

type CloudAPIError struct {
	StatusCode    int
	Message       string
	isCodeUnknown bool
}

func (e CloudAPIError) Error() string {
	return fmt.Sprintf("CloudAPIError( Code: %d, Message: '%s' )", e.StatusCode, e.Message)
}

func (e CloudAPIError) IsCodeUnknown() bool {
	return e.isCodeUnknown
}

func newCloudAPIError(statusCode int, body string) error {
	return CloudAPIError{
		StatusCode:    statusCode,
		Message:       fmt.Sprintf("Cloud API returned error message: %s", body),
		isCodeUnknown: false,
	}
}

func newUnknownCloudAPIError(statusCode int) error {
	return CloudAPIError{
		StatusCode:    statusCode,
		Message:       "Cloud API server misbehaving, it has returned an unexpected status code",
		isCodeUnknown: true,
	}
}

// GetCloudErrorCode checks if the underlying error is of cloudAPIError type.
// In case it is of cloudAPIError type, it returns the error, else it returns nil.
func ToCloudError(err error) *CloudAPIError {
	if err == nil {
		return nil
	}
	cerr, ok := err.(CloudAPIError)
	if !ok {
		return nil
	}
	return &cerr
}

type VM struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type VMStatus struct {
	CPUUtilization int32 `json:"cpuUtilization"`
}

func CreateVM(logger *log.Entry, name string) (*VM, error) {
	path := "/servers"
	url := cloudAPIURL + path

	logger = logger.WithFields(log.Fields{
		"path": path,
	})

	type bodyType struct {
		Name string `json:"name"`
	}
	body := bodyType{Name: name}
	bodyStr, _ := json.Marshal(body)
	e := metrics.NewExecution(reqPOST, path)
	res, err := http.Post(url, "application/json", bytes.NewBuffer(bodyStr))
	if err != nil {
		logger.Errorf("Error occurred while getting response from server: %s", err.Error())
		return nil, err
	}
	e.Finish(res.StatusCode)
	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logger.Errorf("Error while reading response body: %s", err.Error())
		return nil, err
	}

	switch res.StatusCode {
	case http.StatusInternalServerError, http.StatusConflict:
		err = newCloudAPIError(res.StatusCode, string(bodyBytes))
		logger.Errorf("Cloud API call failed: %s", err.Error())
		return nil, err

	case http.StatusCreated:
		logger.Info("VM creation in private cloud was successful")
		var vm VM
		if err := json.Unmarshal(bodyBytes, &vm); err != nil {
			logger.Errorf(
				"Error while parsing VM data response from cloud; body: %s, error: %s",
				string(bodyBytes), err.Error())
			return nil, err
		}
		return &vm, nil

	default:
		err = newUnknownCloudAPIError(res.StatusCode)
		logger.Errorf(err.Error())
		return nil, err
	}
}

func IsNameValid(logger *log.Entry, name string) (bool, error) {
	// TODO: We can encode name to escape special characters.
	path := "/check/" + name
	url := cloudAPIURL + path

	logger = logger.WithFields(log.Fields{
		"path": path,
	})

	e := metrics.NewExecution(reqGET, "/check/:name")
	res, err := http.Get(url)
	if err != nil {
		logger.Errorf("Error occurred while getting response from server: %s", err.Error())
		return false, err
	}
	e.Finish(res.StatusCode)
	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logger.Errorf("Error while reading response body: %s", err.Error())
		return false, err
	}

	switch res.StatusCode {
	case http.StatusInternalServerError, http.StatusNotFound:
		err = newCloudAPIError(res.StatusCode, string(bodyBytes))
		logger.Errorf("Cloud API call failed: %s", err.Error())
		return false, err

	case http.StatusForbidden:
		err = newCloudAPIError(res.StatusCode, string(bodyBytes))
		logger.Errorf("Cloud API call failed: %s", err.Error())
		return false, nil

	case http.StatusOK:
		return true, nil

	default:
		err = newUnknownCloudAPIError(res.StatusCode)
		logger.Errorf(err.Error())
		return false, err
	}
}

func GetVMStatus(logger *log.Entry, id string) (*VMStatus, error) {
	path := "/servers/" + id + "/status"
	url := cloudAPIURL + path

	logger = logger.WithFields(log.Fields{
		"path": path,
	})

	e := metrics.NewExecution(reqGET, "/servers/:id/status")
	res, err := http.Get(url)
	if err != nil {
		logger.Errorf("Error occurred while getting response from server: %s", err.Error())
		return nil, err
	}
	e.Finish(res.StatusCode)
	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logger.Errorf("Error while reading response body: %s", err.Error())
		return nil, err
	}

	switch res.StatusCode {
	case http.StatusInternalServerError, http.StatusNotFound:
		err = newCloudAPIError(res.StatusCode, string(bodyBytes))
		logger.Errorf("Cloud API call failed: %s", err.Error())
		return nil, err

	case http.StatusOK:
		logger.Info("VM status was fetched successfully")
		var vmStatus VMStatus
		if err := json.Unmarshal(bodyBytes, &vmStatus); err != nil {
			logger.Errorf(
				"Error while parsing VMStatus data response from cloud; body: %s, error: %s",
				string(bodyBytes), err.Error())
			return nil, err
		}
		return &vmStatus, nil

	default:
		err = newUnknownCloudAPIError(res.StatusCode)
		logger.Errorf(err.Error())
		return nil, err
	}
}

func DeleteVM(logger *log.Entry, id string) error {
	path := "/servers/" + id
	url := cloudAPIURL + path

	logger = logger.WithFields(log.Fields{
		"path": path,
	})

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		logger.Errorf("Error occurred while creating the request: %s", err.Error())
		return err
	}

	client := &http.Client{}
	e := metrics.NewExecution(reqDELETE, "/servers/:id")
	res, err := client.Do(req)
	if err != nil {
		logger.Errorf("Error occurred while getting response from server: %s", err.Error())
		return err
	}
	e.Finish(res.StatusCode)
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusInternalServerError:
		err = newCloudAPIError(res.StatusCode, "")
		logger.Errorf("Cloud API call failed: %s", err.Error())
		return err

	case http.StatusNoContent:
		logger.Info("VM was deleted successfully")
		return nil

	default:
		err = newUnknownCloudAPIError(res.StatusCode)
		logger.Errorf(err.Error())
		return err
	}
}
