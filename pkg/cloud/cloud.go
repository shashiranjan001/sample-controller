package cloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"k8s.io/klog/v2"
)

var CloudAPIErrors = map[int]string{
	http.StatusInternalServerError: "CloudAPIInternalServerError",
	http.StatusNotFound:            "CloudAPINotFoundError",
	http.StatusConflict:            "CloudAPIConflictError",
	http.StatusForbidden:           "CloudAPIForbiddenError",
}

const (
	UnexpectedCloudAPIResponseStatus = "UnexpectedCloudAPIResponseStatus"
)

type VM struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type VMStatus struct {
	CPUUtilization int32 `json:"cpuUtilization"`
}

const baseUrl = "http://10.150.108.26:8080"

func CreateVM(name string) (*VM, error) {
	url := baseUrl + "/servers"
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
		err = fmt.Errorf("Error: %s, Message: %s", CloudAPIErrors[res.StatusCode], string(bodyBytes))
		klog.Errorf("Failure message from cloud: %s", err.Error())
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
		klog.Errorf("%s: %d", UnexpectedCloudAPIResponseStatus, res.StatusCode)
		err = fmt.Errorf("%s: %d", UnexpectedCloudAPIResponseStatus, res.StatusCode)
		return nil, err
	}
}

func IsNameValid(name string) (bool, error) {
	// TODO: We can encode name to escape special characters.
	url := baseUrl + "/check/" + name
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
		err = fmt.Errorf("Error: %s, Message: %s", CloudAPIErrors[res.StatusCode], string(bodyBytes))
		klog.Errorf("Failure message from cloud: %s", err.Error())
		return false, err

	case http.StatusForbidden:
		err = fmt.Errorf("Error: %s, Message: %s", CloudAPIErrors[res.StatusCode], string(bodyBytes))
		klog.Errorf("Failure message from cloud: %s", err.Error())
		return false, nil

	case http.StatusOK:
		return true, nil

	default:
		klog.Errorf("%s: %d", UnexpectedCloudAPIResponseStatus, res.StatusCode)
		err = fmt.Errorf("%s: %d", UnexpectedCloudAPIResponseStatus, res.StatusCode)
		return false, err
	}
}

func GetVMStatus(id string) (*VMStatus, error) {
	url := baseUrl + "/servers/" + id + "/status"
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
		err = fmt.Errorf("Error: %s, Message: %s", CloudAPIErrors[res.StatusCode], string(bodyBytes))
		klog.Errorf("Failure message from cloud: %s", err.Error())
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
		klog.Errorf("%s: %d", UnexpectedCloudAPIResponseStatus, res.StatusCode)
		err = fmt.Errorf("%s: %d", UnexpectedCloudAPIResponseStatus, res.StatusCode)
		return nil, err
	}
}

func DeleteVM(id string) error {
	url := baseUrl + "/servers/" + id
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
		err = fmt.Errorf("Error: %s", CloudAPIErrors[res.StatusCode])
		klog.Errorf("Failure message from cloud: %s", err.Error())
		return err

	case http.StatusNoContent:
		klog.Info("VM was deleted successfully")
		return nil

	default:
		klog.Errorf("%s: %d", UnexpectedCloudAPIResponseStatus, res.StatusCode)
		err = fmt.Errorf("%s: %d", UnexpectedCloudAPIResponseStatus, res.StatusCode)
		return err
	}
}
