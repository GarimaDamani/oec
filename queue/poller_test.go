package queue

import (
	"math/rand"
	"github.com/aws/aws-sdk-go/service/sqs"
	"strconv"
	"sync"
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/pkg/errors"
	"github.com/opsgenie/marid2/conf"
)

var mockPollerConf = &conf.PollerConf{
	pollingWaitIntervalInMillis,
	visibilityTimeoutInSec,
	maxNumberOfMessages,
}

func newPollerTest() *MaridPoller {
	return &MaridPoller {
		quit:           make(chan struct{}),
		wakeUpChan:     make(chan struct{}),
		isRunning:		false,
		startStopMutex: &sync.Mutex{},
		pollerConf:     mockPollerConf,
		workerPool:     NewMockWorkerPool(),
		queueProvider:  NewMockQueueProvider(),
	}
}

func mockReceiveMessageSuccess(mqp *MaridQueueProvider, numOfMessage int64, visibilityTimeout int64) ([]*sqs.Message, error) {
	messages := make([]*sqs.Message, 0)
	for i := 1; i < rand.Intn(int(numOfMessage+1)) ; i++ {
		sqsMessage := &sqs.Message{}
		random := rand.Intn(301) * i
		sqsMessage.SetMessageId( strconv.Itoa(int(random) ))
		messages = append(messages, sqsMessage)
	}
	return messages, nil
}

func mockReceiveMessageExactNumber(numOfMessage int64, visibilityTimeout int64) ([]*sqs.Message, error) {
	messages := make([]*sqs.Message, 0)
	for i := int64(0); i < numOfMessage ; i++ {
		sqsMessage := &sqs.Message{}
		sqsMessage.SetMessageId(strconv.Itoa(int(i+1)))
		messages = append(messages, sqsMessage)
	}
	return messages, nil
}

func TestStartAndStopPolling(t *testing.T) {

	poller := newPollerTest()

	err := poller.StartPolling()
	assert.Nil(t, err)
	assert.Equal(t, true, poller.isRunning)

	err = poller.StartPolling()
	assert.NotNil(t, err)
	assert.Equal(t, "Poller is already running.", err.Error())
	assert.Equal(t, true, poller.isRunning)

	err = poller.StopPolling()
	assert.Nil(t, err)
	assert.Equal(t, false, poller.isRunning)
}


func TestStopPollingNonPollingState(t *testing.T) {

	poller := newPollerTest()

	err := poller.StopPolling()
	assert.NotNil(t, err)
	assert.Equal(t, "Poller is not running.", err.Error())
}

func TestPollWithNoAvailableWorker(t *testing.T) {

	poller := newPollerTest()

	poller.workerPool.(*MockWorkerPool).NumberOfAvailableWorkerFunc = func() int32 {
		return 0
	}

	shouldWait := poller.poll()
	assert.True(t, shouldWait)
}

func TestPollWithReceiveError(t *testing.T) {

	poller := newPollerTest()

	poller.workerPool.(*MockWorkerPool).NumberOfAvailableWorkerFunc = func() int32 {
		return 1
	}
	poller.queueProvider.(*MockQueueProvider).ReceiveMessageFunc = func(i int64, i2 int64) ([]*sqs.Message, error) {
		return nil, errors.New("")
	}

	shouldWait := poller.poll()
	assert.True(t, shouldWait)
}

func TestPollZeroMessage(t *testing.T) {

	poller := newPollerTest()

	poller.workerPool.(*MockWorkerPool).NumberOfAvailableWorkerFunc = func() int32 {
		return 1
	}
	poller.queueProvider.(*MockQueueProvider).ReceiveMessageFunc = func(i int64, i2 int64) ([]*sqs.Message, error) {
		return []*sqs.Message{{},{},{}}, nil
	}

	shouldWait := poller.poll()
	assert.True(t, shouldWait)
}

func TestPollMaxMessage(t *testing.T) {

	poller := newPollerTest()

	expected := 4

	poller.workerPool.(*MockWorkerPool).NumberOfAvailableWorkerFunc = func() int32 {
		return int32(expected)
	}

	maxNumberOfMessages := 0
	poller.queueProvider.(*MockQueueProvider).ReceiveMessageFunc = func(numOfMessage int64, visibilityTimeout int64) ([]*sqs.Message, error) {
		maxNumberOfMessages = int(numOfMessage)
		return nil, errors.New("Receive Error")
	}

	shouldWait := poller.poll()
	assert.True(t, shouldWait)
	assert.Equal(t, expected, maxNumberOfMessages)
}

func TestPollMaxMessageUpperBound(t *testing.T) {

	poller := newPollerTest()

	availableWorkerCount := 12

	poller.workerPool.(*MockWorkerPool).NumberOfAvailableWorkerFunc = func() int32 {
		return int32(availableWorkerCount)
	}

	maxNumberOfMessages := int64(0)
	poller.queueProvider.(*MockQueueProvider).ReceiveMessageFunc = func(numOfMessage int64, visibilityTimeout int64) ([]*sqs.Message, error) {
		maxNumberOfMessages = numOfMessage
		return nil, errors.New("Receive Error")
	}

	shouldWait := poller.poll()
	assert.True(t, shouldWait)
	assert.Equal(t, poller.pollerConf.MaxNumberOfMessages, maxNumberOfMessages)
}

func TestPollMessageSubmitFail(t *testing.T) {

	poller := newPollerTest()

	expected := 4

	poller.workerPool.(*MockWorkerPool).NumberOfAvailableWorkerFunc = func() int32 {
		return int32(expected)
	}
	poller.queueProvider.(*MockQueueProvider).ReceiveMessageFunc = mockSuccessReceiveFunc

	submitCount := 0
	poller.workerPool.(*MockWorkerPool).SubmitFunc = func(job Job) (bool, error) {
		submitCount++
		return false, nil
	}

	releaseCount := 0
	poller.queueProvider.(*MockQueueProvider).ChangeMessageVisibilityFunc = func(message *sqs.Message, visibilityTimeout int64) error {
		if visibilityTimeout == 0 {
			releaseCount++
		}
		return nil
	}

	shouldWait := poller.poll()

	assert.False(t, shouldWait)
	assert.Equal(t, expected, submitCount)
	assert.Equal(t, expected, releaseCount)
}

func TestPollMessageSubmitError(t *testing.T) {

	poller := newPollerTest()

	expected := 5

	poller.workerPool.(*MockWorkerPool).NumberOfAvailableWorkerFunc = func() int32 {
		return int32(expected)
	}
	poller.queueProvider.(*MockQueueProvider).ReceiveMessageFunc = mockSuccessReceiveFunc

	submitCount := 0
	poller.workerPool.(*MockWorkerPool).SubmitFunc = func(job Job) (bool, error) {
		submitCount++
		return false, errors.New("Submit Error")
	}

	releaseCount := 0
	poller.queueProvider.(*MockQueueProvider).ChangeMessageVisibilityFunc = func(message *sqs.Message, visibilityTimeout int64) error {
		if visibilityTimeout == 0 {
			releaseCount++
		}
		return nil
	}

	shouldWait := poller.poll()

	assert.True(t, shouldWait)
	assert.Equal(t, 1, submitCount)
	assert.Equal(t, expected, releaseCount)
}

func TestPollMessageWithFalseIntegrationId(t *testing.T) {

	poller := newPollerTest()

	poller.workerPool.(*MockWorkerPool).NumberOfAvailableWorkerFunc = func() int32 {
		return 3
	}
	poller.queueProvider.(*MockQueueProvider).ReceiveMessageFunc = func(numOfMessage int64, visibilityTimeout int64) ([]*sqs.Message, error) {
		integrationId := "falseIntegrationId"
		messageAttr := map[string]*sqs.MessageAttributeValue{"integrationId": {StringValue: &integrationId} }
		message := &sqs.Message{MessageAttributes: messageAttr}
		return []*sqs.Message{message, message, message}, nil
	}

	deleteCount := 0
	poller.queueProvider.(*MockQueueProvider).DeleteMessageFunc = func(message *sqs.Message) error {
		if *message.MessageAttributes["integrationId"].StringValue == "falseIntegrationId" {
			deleteCount++
		}
		return nil
	}

	shouldWait := poller.poll()

	assert.False(t, shouldWait)
	assert.Equal(t, 3, deleteCount)
}

func TestPollMessageSubmitSuccess(t *testing.T) {

	poller := newPollerTest()

	poller.workerPool.(*MockWorkerPool).NumberOfAvailableWorkerFunc = func() int32 {
		return 5
	}
	poller.queueProvider.(*MockQueueProvider).ReceiveMessageFunc = mockSuccessReceiveFunc

	poller.workerPool.(*MockWorkerPool).SubmitFunc = func(job Job) (bool, error) {
		return true, nil
	}

	shouldWait := poller.poll()

	assert.False(t, shouldWait)
}

// Mock Poller
type MockPoller struct {
	StartPollingFunc func() error
	StopPollingFunc func() error

	RefreshClientFunc func(assumeRoleResult AssumeRoleResult) error
	QueueProviderFunc func() QueueProvider
}

func NewMockPoller() Poller {
	return &MockPoller{}
}

func NewMockPollerForQueueProcessor(workerPool WorkerPool, queueProvider QueueProvider, pollerConf *conf.PollerConf, actionMappings *conf.ActionMappings) Poller {
	return NewMockPoller()
}

func (p *MockPoller) StartPolling() error {
	if p.StartPollingFunc != nil {
		return p.StartPollingFunc()
	}
	return nil
}

func (p *MockPoller) StopPolling() error {
	if p.StopPollingFunc != nil {
		return p.StopPollingFunc()
	}
	return nil
}

func (p *MockPoller) RefreshClient(assumeRoleResult AssumeRoleResult) error {
	if p.RefreshClientFunc != nil {
		return p.RefreshClientFunc(assumeRoleResult)
	}
	return nil
}

func (p *MockPoller) QueueProvider() QueueProvider {
	if p.QueueProviderFunc != nil {
		return p.QueueProviderFunc()
	}
	return NewMockQueueProvider()
}

