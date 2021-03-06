package manager

import (
	"fmt"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"time"

	"github.com/obazavil/openstack-workload-transcoding/wttypes"
)

const (
	MongoDB              = "transcoding"
	MongoTasksCollection = "tasks"
)

type TaskDB struct {
	ID            bson.ObjectId `bson:"_id"`
	TranscodingID string        `bson:"transcoding_id"`
	ObjectName    string        `bson:"object_name"`
	Profile       string        `bson:"profile"`
	WorkerAddr    string        `bson:"worker_addr"`
	Added         time.Time     `bson:"added"`
	Started       time.Time     `bson:"started"`
	Ended         time.Time     `bson:"ended"`
	Status        string        `bson:"status"`
}

type DataStore struct {
	session *mgo.Session
}

func (ds *DataStore) Close() {
	ds.session.Close()
}

func NewDataStore(s *mgo.Session) *DataStore {
	ds := &DataStore{
		session: s.Copy(),
	}
	return ds
}

func CreateMongoSession() (*mgo.Session, error) {
	// Dial to DB
	session, err := mgo.Dial("localhost")
	if err != nil {
		return nil, err
	}

	// Session is monotonic
	session.SetMode(mgo.Monotonic, true)

	// Get "tasks" collection
	c := session.DB(MongoDB).C(MongoTasksCollection)

	// Indexes
	idxID := mgo.Index{
		Key:        []string{"id"},
		Unique:     true,
		DropDups:   true,
		Background: true,
		Sparse:     true,
	}
	err = c.EnsureIndex(idxID)
	if err != nil {
		return nil, err
	}

	idxTransactionID := mgo.Index{
		Key:        []string{"transaction_id"},
		Unique:     true,
		DropDups:   true,
		Background: true,
		Sparse:     true,
	}
	err = c.EnsureIndex(idxTransactionID)
	if err != nil {
		return nil, err
	}

	idxStatus := mgo.Index{
		Key:        []string{"status"},
		Unique:     false,
		DropDups:   false,
		Background: true,
		Sparse:     true,
	}
	err = c.EnsureIndex(idxStatus)
	if err != nil {
		return nil, err
	}

	return session, nil
}

func (ds *DataStore) AddTask(task wttypes.TranscodingTask) (string, error) {
	id := bson.NewObjectId()

	t := TaskDB{
		ID:            id,
		TranscodingID: task.ID,
		ObjectName:    task.ObjectName,
		Profile:       task.Profile,
		Status:        wttypes.TRANSCODING_QUEUED,
		Added:         time.Now(),
	}

	// Get "tasks" collection
	c := ds.session.DB(MongoDB).C(MongoTasksCollection)

	// Insert
	err := c.Insert(&t)
	if err != nil {
		return "", err
	}

	return task.ID, nil
}

func (ds *DataStore) GetTotalTasksQueued() (int, error) {
	// Get "tasks" collection
	c := ds.session.DB(MongoDB).C(MongoTasksCollection)

	// Query All
	var results []struct {
		Status string `bson:"status"`
	}
	err := c.Find(bson.M{"status": wttypes.TRANSCODING_QUEUED}).Select(bson.M{"status": 1}).All(&results)

	if err != nil {
		return 0, err
	}

	return len(results), nil
}

func (ds *DataStore) GetTotalTasksRunning() (int, error) {
	// Get "tasks" collection
	c := ds.session.DB(MongoDB).C(MongoTasksCollection)

	// Query All
	var results []struct {
		Status string `bson:"status"`
	}
	err := c.Find(bson.M{"status": wttypes.TRANSCODING_RUNNING}).Select(bson.M{"status": 1}).All(&results)

	if err != nil {
		return 0, err
	}

	return len(results), nil
}

func (ds *DataStore) GetNextQueuedTask(workerAddr string) (wttypes.TranscodingTask, error) {
	// Get "tasks" collection
	c := ds.session.DB(MongoDB).C(MongoTasksCollection)

	// Get next one with "queued" status
	result := TaskDB{}
	err := c.Find(bson.M{"status": wttypes.TRANSCODING_QUEUED}).Sort("added").One(&result)
	if err != nil {
		return wttypes.TranscodingTask{}, err
	}

	// TODO: change to TRANSCODING_REQUESTED and add an extra ACK step

	// Update to "running"
	result.Status = wttypes.TRANSCODING_RUNNING
	result.WorkerAddr = workerAddr
	result.Started = time.Now()

	_, err = c.UpsertId(result.ID, result)
	if err != nil {
		return wttypes.TranscodingTask{}, err
	}

	return wttypes.TranscodingTask{
		ID:         result.TranscodingID,
		ObjectName: result.ObjectName,
		Profile:    result.Profile,
	}, nil
}

func (ds *DataStore) UpdateTaskStatus(id string, status string) error {
	fmt.Println("[database] UpdateTaskStatus:", id, status)
	// Get "tasks" collection
	c := ds.session.DB(MongoDB).C(MongoTasksCollection)

	t := TaskDB{}
	err := c.Find(bson.M{"transcoding_id": id}).One(&t)
	if err != nil {
		return err
	}

	// Update Status
	t.Status = status
	if t.Status == wttypes.TRANSCODING_FINISHED {
		t.Ended = time.Now()
	}

	// Update in DB
	_, err = c.UpsertId(t.ID, t)
	if err != nil {
		return err
	}

	fmt.Println("[manager] UpdateTaskStatus updated:", id, status)
	return nil
}

func (ds *DataStore) CancelTask(id string) (string, error) {
	fmt.Println("[database] CancelTask:", id)
	// Get "tasks" collection
	c := ds.session.DB(MongoDB).C(MongoTasksCollection)

	t := TaskDB{}
	err := c.Find(bson.M{"transcoding_id": id}).One(&t)
	if err != nil {
		return "", err
	}

	// Let's cancel on DB
	if t.Status == wttypes.TRANSCODING_QUEUED {
		t.Status = wttypes.TRANSCODING_CANCELLED

		// Update in DB
		_, err = c.UpsertId(t.ID, t)
		if err != nil {
			return "", err
		}
	} else if t.Status == wttypes.TRANSCODING_RUNNING {
		return t.WorkerAddr, nil
	}

	return "", nil
}
