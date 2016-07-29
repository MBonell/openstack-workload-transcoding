package monitor

import (
	"crypto/tls"

	"fmt"
	"github.com/go-resty/resty"
	"github.com/obazavil/openstack-workload-transcoding/wtcommon"
	"github.com/obazavil/openstack-workload-transcoding/wttypes"
	"strings"
)

// Service is the interface that provides transcoding monitor methods.
type Service interface {
	// Register a worker
	RegisterWorker(addr string) error

	// Deregister a worker
	DeregisterWorker(addr string) error

	// Update the status of a worker
	UpdateWorkerStatus(ws wttypes.WorkerStatus) error
}

type service struct {
}

func (s *service) RegisterWorker(addr string) error {
	fmt.Println("registering worker:", addr)

	ws := wttypes.WorkerStatus{
		Addr: addr,
		Status: wttypes.WORKER_STATUS_ONLINE,
	}
	err := s.UpdateWorkerStatus(ws)

	return err
}

func (s *service) DeregisterWorker(addr string) error {
	fmt.Println("deregistering worker:", addr)

	ws := wttypes.WorkerStatus{
		Addr: addr,
		Status: wttypes.WORKER_STATUS_OFFLINE,
	}
	err := s.UpdateWorkerStatus(ws)

	return err
}

func (s *service) UpdateWorkerStatus(ws wttypes.WorkerStatus) error {
	fmt.Println("changing status:", ws.Addr, ws.Status)

	// Update Worker in DB
	resp, err := resty.R().
		SetBody(ws).
		Put(wtcommon.Servers["database"] + "/workers/status")

	// Error in communication
	if err != nil {
		fmt.Println("[err] UpdateWorkerStatus:", err)
		return err
	}

	str := resp.String()

	// There was an error in the response?
	if strings.HasPrefix(str, `{"error"`) {
		fmt.Println("[err] UpdateWorkerStatus:", err)
		return wtcommon.JSON2Err(str)
	}

	fmt.Println("changing status OK")
	return nil
}

// NewService creates a transcoding monitor service with necessary dependencies.
func NewService() Service {
	resty.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})

	return &service{}
}
