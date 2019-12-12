/*
Copyright 2019

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kinesis

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"

	"github.com/knative/eventing-sources/pkg/kncloudevents"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"

	"github.com/knative/pkg/logging"
	cfg "github.com/vmware/vmware-go-kcl/clientlibrary/config"
	kc "github.com/vmware/vmware-go-kcl/clientlibrary/interfaces"
	"github.com/vmware/vmware-go-kcl/clientlibrary/metrics"
	wk "github.com/vmware/vmware-go-kcl/clientlibrary/worker"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

const (
	eventType    = "aws.kinesis.event"
	eventVersion = "1.0"

	//Extension eventName fields to support KinesisEvent
	extEventName = "aws:kinesis:record"

	//Extension eventSource fields to support KinesisEvent
	extEventSource = "aws:kinesis"

	//Extension kinesisSchemaVersion fields to support KinesisEvent
	extKinesisSchemaVersion = "1.0"
)

// Adapter implements the Kinesis adapter to deliver Kinesis messages from
// a stream to a Sink.
type Adapter struct {

	// StreamName is the Stream name
	StreamName string

	// Region is the Kinesis stream region
	Region string

	// SinkURI is the URI messages will be forwarded on to.
	SinkURI string

	// KCLIAMRoleARN is the pre-existing AWS IAM role to access Kinesis stream, it is required when using KIAM approach.
	KCLIAMRoleARN string

	// CredsFile is the full path of the AWS credentials file, it is required when using k8s secret approach.
	CredsFile string

	// stream ARN
	streamARN *string

	//Application consumer name
	ConsumerName string

	// Client sends cloudevents to the target.
	client client.Client
}

// Initialize cloudevent client
func (a *Adapter) initClient() error {
	if a.client == nil {
		var err error
		if a.client, err = kncloudevents.NewDefaultClient(a.SinkURI); err != nil {
			return err
		}
	}
	return nil
}

// Start function
func (a *Adapter) Start(ctx context.Context, stopCh <-chan struct{}) error {

	logger := logging.FromContext(ctx)

	if err := a.initClient(); err != nil {
		logger.Error("Failed to create cloudevent client", zap.Error(err))
		return err
	}

	var creds *credentials.Credentials
	var sess *session.Session

	if len(a.CredsFile) > 0 {
		sess = session.Must(session.NewSessionWithOptions(session.Options{
			SharedConfigState: session.SharedConfigDisable,
			Config:            aws.Config{Region: &a.Region},
			SharedConfigFiles: []string{a.CredsFile},
		}))
		creds = sess.Config.Credentials
	} else if len(a.KCLIAMRoleARN) > 0 {

		sess = session.Must(session.NewSession())

		// Create the credentials from AssumeRoleProvider to assume the role
		// referenced by the "KCLIAMRoleARN" ARN.
		creds = stscreds.NewCredentials(sess, a.KCLIAMRoleARN)
	} else {
		return fmt.Errorf("Neither AWS_APPLICATION_CREDENTIALS nor KCL_IAM_ROLE_ARN is found in ENV")
	}

	// Kinesis API client
	kinesisClient := kinesis.New(sess, &aws.Config{Credentials: creds, Region: aws.String(a.Region)})
	stream, err := kinesisClient.DescribeStream(&kinesis.DescribeStreamInput{StreamName: aws.String(a.StreamName)})
	if err != nil {
		logger.Error("Failed to describe stream input", zap.Error(err))
		return err
	}
	a.streamARN = stream.StreamDescription.StreamARN

	//using the consumer name as worker id. Future enhancement can associate different workers for the same consumer
	kclConfig := cfg.NewKinesisClientLibConfigWithCredential(a.ConsumerName, a.StreamName, a.Region, a.ConsumerName, creds).
		WithInitialPositionInStream(cfg.LATEST).
		WithMaxRecords(10).
		WithMaxLeasesForWorker(20).
		WithShardSyncIntervalMillis(5000).
		WithFailoverTimeMillis(300000)

	// configure cloudwatch as metrics system
	metricsConfig := &metrics.MonitoringConfiguration{
		MonitoringService: "cloudwatch",
		Region:            a.Region,
		CloudWatch: metrics.CloudWatchMonitoringService{
			MetricsBufferTimeMillis: 10000,
			MetricsMaxQueueSize:     20,
			Credentials:             creds,
		}}

	worker := wk.NewWorker(recordProcessorFactory(a, logger), kclConfig, metricsConfig)

	if err = worker.Start(); err != nil {
		return err
	}
	<-stopCh
	logger.Info("Shutting down.")
	return nil
}

// Record processor factory is used to create RecordProcessor
func recordProcessorFactory(adap *Adapter, logger *zap.SugaredLogger) kc.IRecordProcessorFactory {
	return &sourceRecordProcessorFactory{adapter: adap, logger: logger}
}

// Record processor
type sourceRecordProcessorFactory struct {
	adapter *Adapter
	logger  *zap.SugaredLogger
}

// RecordProcessor is the interface for some callback functions invoked by KCL
// The main task of using KCL is to provide implementation on IRecordProcessor interface.
func (s *sourceRecordProcessorFactory) CreateProcessor() kc.IRecordProcessor {
	return &sourceRecordProcessor{adapter: s.adapter, logger: s.logger}
}

// source record processor
type sourceRecordProcessor struct {
	adapter *Adapter
	logger  *zap.SugaredLogger
}

func (s *sourceRecordProcessor) Initialize(input *kc.InitializationInput) {
	s.logger.Infof("Processing SharId: %v at checkpoint: %v", input.ShardId, aws.StringValue(input.ExtendedSequenceNumber.SequenceNumber))
}

func (s *sourceRecordProcessor) ProcessRecords(input *kc.ProcessRecordsInput) {

	var err error

	logger := s.logger
	logger.Info("Processing Records...")

	// don't process empty record
	if len(input.Records) == 0 {
		return
	}

	err = s.adapter.postMessage(input, s.logger)
	if err != nil {
		logger.Errorf("Failed to post message: %v", err)
		return
	}

	// checkpoint it after processing this batch
	lastRecordSequenceNumber := input.Records[len(input.Records)-1].SequenceNumber
	logger.Infof("Checkpoint progress at: %v,  MillisBehindLatest = %v", lastRecordSequenceNumber, input.MillisBehindLatest)
	input.Checkpointer.Checkpoint(lastRecordSequenceNumber)
}

func (s *sourceRecordProcessor) Shutdown(input *kc.ShutdownInput) {
	logger := s.logger
	logger.Infof("Shutdown Reason: %v", aws.StringValue(kc.ShutdownReasonMessage(input.ShutdownReason)))

	// When shutdown reason is terminate checkpoint is issued
	// Failure to do will result in  KCL not making any further progress.
	if input.ShutdownReason == kc.TERMINATE {
		input.Checkpointer.Checkpoint(nil)
	}

}

// postMessage sends an Kinesis event to the SinkURI
func (a *Adapter) postMessage(m *kc.ProcessRecordsInput, logger *zap.SugaredLogger) error {

	sequenceNumber := m.Records[0].SequenceNumber
	recordsCount := len(m.Records)
	logger.Infof("Total record count: %v", recordsCount)
	eventID := fmt.Sprintf("%v:%v", sequenceNumber, recordsCount)
	logger.Infof("Event Id: %v", eventID)
	ext := map[string]interface{}{"Ext_KinesisSchemaVersion": extKinesisSchemaVersion, "Ext_EventSource": extEventSource, "Ext_EventName": extEventName, "Ext_EventSourceARN": *a.streamARN, "Ext_Region": a.Region}
	logger.Infof("time ; %v", m.MillisBehindLatest)

	event := cloudevents.Event{
		Context: cloudevents.EventContextV02{
			ID:         eventID,
			Type:       eventType,
			Source:     *types.ParseURLRef(fmt.Sprintf("/%s", *a.streamARN)),
			Time:       &types.Timestamp{Time: time.Now().Add(-1 * time.Millisecond * time.Duration(m.MillisBehindLatest))},
			Extensions: ext,
		}.AsV02(),
		Data: m,
	}
	_, err := a.client.Send(context.TODO(), event)
	return err
}
