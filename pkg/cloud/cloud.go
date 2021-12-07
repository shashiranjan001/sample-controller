package cloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gotway/gotway/pkg/env"
	"k8s.io/klog/v2"
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

func CreateVM(name string) (*VM, error) {
	url := cloudAPIURL + "/servers"
	type bodyType struct {
		Name string `json:"name"`
	}
	body := bodyType{Name: name}
	bodyStr, _ := json.Marshal(body)
	res, err := http.Post(url, "application/json", bytes.NewBuffer(bodyStr))
	if err != nil {
		klog.Errorf("Error while invoking %s: %s", url, err.Error())
		return nil, err
	}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		klog.Errorf("Error while reading response body: %s", err.Error())
		return nil, err
	}

	switch res.StatusCode {
	case http.StatusInternalServerError, http.StatusConflict:
		err = newCloudAPIError(res.StatusCode, string(bodyBytes))
		klog.Errorf("Cloud API call failed: %s", err.Error())
		return nil, err

	case http.StatusCreated:
		klog.Info("VM creation in private cloud was successful")
		var vm VM
		if err := json.Unmarshal(bodyBytes, &vm); err != nil {
			klog.Errorf(
				"Error while parsing VM data response from cloud; body: %s, error: %s",
				string(bodyBytes), err.Error())
			return nil, err
		}
		return &vm, nil

	default:
		err = newUnknownCloudAPIError(res.StatusCode)
		klog.Errorf(err.Error())
		return nil, err
	}
}

func IsNameValid(name string) (bool, error) {
	// TODO: We can encode name to escape special characters.
	url := cloudAPIURL + "/check/" + name
	res, err := http.Get(url)
	if err != nil {
		klog.Errorf("Error while invoking %s: %s", url, err.Error())
		return false, err
	}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		klog.Errorf("Error while reading response body: %s", err.Error())
		return false, err
	}

	switch res.StatusCode {
	case http.StatusInternalServerError, http.StatusNotFound:
		err = newCloudAPIError(res.StatusCode, string(bodyBytes))
		klog.Errorf("Cloud API call failed: %s", err.Error())
		return false, err

	case http.StatusForbidden:
		err = newCloudAPIError(res.StatusCode, string(bodyBytes))
		klog.Errorf("Cloud API call failed: %s", err.Error())
		return false, nil

	case http.StatusOK:
		return true, nil

	default:
		err = newUnknownCloudAPIError(res.StatusCode)
		klog.Errorf(err.Error())
		return false, err
	}
}

func GetVMStatus(id string) (*VMStatus, error) {
	url := cloudAPIURL + "/servers/" + id + "/status"
	res, err := http.Get(url)
	if err != nil {
		klog.Errorf("Error while invoking %s: %s", url, err.Error())
		return nil, err
	}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		klog.Errorf("Error while reading response body: %s", err.Error())
		return nil, err
	}

	switch res.StatusCode {
	case http.StatusInternalServerError, http.StatusNotFound:
		err = newCloudAPIError(res.StatusCode, string(bodyBytes))
		klog.Errorf("Cloud API call failed: %s", err.Error())
		return nil, err

	case http.StatusOK:
		klog.Info("VM status was fetched successfully")
		var vmStatus VMStatus
		if err := json.Unmarshal(bodyBytes, &vmStatus); err != nil {
			klog.Errorf(
				"Error while parsing VMStatus data response from cloud; body: %s, error: %s",
				string(bodyBytes), err.Error())
			return nil, err
		}
		return &vmStatus, nil

	default:
		err = newUnknownCloudAPIError(res.StatusCode)
		klog.Errorf(err.Error())
		return nil, err
	}
}

func DeleteVM(id string) error {
	url := cloudAPIURL + "/servers/" + id
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		klog.Errorf("Error while creating request %s: %s", url, err.Error())
		return err
	}

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		klog.Errorf("Error while invoking %s: %s", url, err.Error())
		return err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusInternalServerError:
		err = newCloudAPIError(res.StatusCode, "")
		klog.Errorf("Cloud API call failed: %s", err.Error())
		return err

	case http.StatusNoContent:
		klog.Info("VM was deleted successfully")
		return nil

	default:
		err = newUnknownCloudAPIError(res.StatusCode)
		klog.Errorf(err.Error())
		return err
	}
}
